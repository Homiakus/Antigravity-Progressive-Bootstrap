package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/backup"
	"github.com/homiakus/agctl/internal/capability"
	"github.com/homiakus/agctl/internal/doctor"
	"github.com/homiakus/agctl/internal/installer"
	"github.com/homiakus/agctl/internal/loop"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/mcpprobe"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/permissions"
	"github.com/homiakus/agctl/internal/planner"
	"github.com/homiakus/agctl/internal/plugin"
	"github.com/homiakus/agctl/internal/replan"
	"github.com/homiakus/agctl/internal/router"
	"github.com/homiakus/agctl/internal/securityaudit"
	"github.com/homiakus/agctl/internal/skills"
	"github.com/homiakus/agctl/internal/tasks"
	"github.com/homiakus/agctl/internal/telemetry"
)

const AppVersion = "3.2.1"

type SystemSnapshot struct {
	Version          string                 `json:"version"`
	GeneratedAt      string                 `json:"generatedAt"`
	ListenURL        string                 `json:"listenUrl"`
	Workspace        string                 `json:"workspace"`
	LoopConfig       model.LoopConfig       `json:"loopConfig"`
	RouterStatus     map[string]any         `json:"routerStatus"`
	DoctorHealth     DoctorHealthSummary    `json:"doctorHealth"`
	TaskCounts       map[string]int         `json:"taskCounts"`
	PlanStatusCounts map[string]int         `json:"planStatusCounts"`
	PlanCount        int                    `json:"planCount"`
	PlanRevisions    int                    `json:"planRevisions"`
	DynamicNodes     int                    `json:"dynamicNodes"`
	ReplanInbox      int                    `json:"replanInbox"`
	ReplanConfig     model.ReplanConfig     `json:"replanConfig"`
	CapabilityCounts map[string]int         `json:"capabilityCounts"`
	MCPServers       []string               `json:"mcpServers"`
	SkillsCount      int                    `json:"skillsCount"`
	PluginsCount     int                    `json:"pluginsCount"`
	Security         model.SecurityReport   `json:"security"`
	Telemetry        telemetry.Summary      `json:"telemetry"`
	RecentEvents     []telemetry.Event      `json:"recentEvents"`
	RecentTasks      []model.TaskRecord     `json:"recentTasks"`
}

type DoctorHealthSummary struct {
	TotalFindings int    `json:"totalFindings"`
	Errors        int    `json:"errors"`
	Warnings      int    `json:"warnings"`
	Infos         int    `json:"infos"`
	Status        string `json:"status"` // "PASS", "WARN", "FAIL"
}

func BuildSystemSnapshot(p paths.Paths, workspace, listenURL string) SystemSnapshot {
	snap := SystemSnapshot{
		Version:          AppVersion,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339Nano),
		ListenURL:        listenURL,
		Workspace:        workspace,
		TaskCounts:       map[string]int{},
		PlanStatusCounts: map[string]int{},
		CapabilityCounts: map[string]int{},
		MCPServers:       []string{},
		RecentTasks:      []model.TaskRecord{},
		RecentEvents:     []telemetry.Event{},
	}

	if cfg, err := loop.Load(p); err == nil {
		snap.LoopConfig = cfg
	}

	if rCfg, err := router.Load(p); err == nil {
		snap.RouterStatus = map[string]any{
			"enabled": rCfg.Enabled,
			"mode":    rCfg.Mode,
		}
	}

	docReport := doctor.Run(p, workspace)
	var errs, warns, infos int
	for _, f := range docReport.Findings {
		switch f.Level {
		case "ERROR":
			errs++
		case "WARN":
			warns++
		default:
			infos++
		}
	}
	status := "PASS"
	if errs > 0 {
		status = "FAIL"
	} else if warns > 0 {
		status = "WARN"
	}
	snap.DoctorHealth = DoctorHealthSummary{
		TotalFindings: len(docReport.Findings),
		Errors:        errs,
		Warnings:      warns,
		Infos:         infos,
		Status:        status,
	}

	if ts, err := tasks.List(p); err == nil {
		for _, t := range ts {
			snap.TaskCounts[t.Status]++
		}
		if len(ts) > 20 {
			ts = ts[:20]
		}
		snap.RecentTasks = ts
	}

	if ps, err := planner.List(p); err == nil {
		snap.PlanCount = len(ps)
		for _, pl := range ps {
			snap.PlanRevisions += pl.Revision
			snap.DynamicNodes += pl.DynamicNodeCount
			st := pl.Status
			if st == "" {
				st = "legacy"
			}
			snap.PlanStatusCounts[st]++
		}
	}

	if cfg, err := replan.LoadConfig(p); err == nil {
		snap.ReplanConfig = cfg
	}
	if inbox, err := replan.Inbox(p); err == nil {
		snap.ReplanInbox = len(inbox)
	}

	if reg, err := capability.Load(p); err == nil {
		for _, c := range reg.Capabilities {
			if c.Enabled {
				snap.CapabilityCounts[c.Kind]++
			}
		}
	}

	if mcpNames, err := mcp.Names(p, workspace); err == nil && mcpNames != nil {
		snap.MCPServers = mcpNames
	}

	if sks, err := skills.List(p); err == nil {
		snap.SkillsCount = len(sks)
	}

	if plugs, err := plugin.List(p, workspace); err == nil {
		snap.PluginsCount = len(plugs)
	}

	if sec, err := securityaudit.Audit(p, workspace); err == nil {
		snap.Security = sec
	}

	if ev, err := telemetry.Recent(p, 200); err == nil {
		snap.Telemetry = telemetry.Summarize(ev)
		if len(ev) > 30 {
			ev = ev[:30]
		}
		snap.RecentEvents = ev
	}

	return snap
}

func newRouter(p paths.Paths, workspace, listen, url string) http.Handler {
	mux := http.NewServeMux()

	// SSE Live Events Stream
	mux.HandleFunc("/api/events/stream", handleSSEStream)

	// Snapshot
	mux.HandleFunc("/api/snapshot", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, BuildSystemSnapshot(p, workspace, url))
	})

	// Logs history
	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, globalBroker.History())
	})

	// Doctor
	mux.HandleFunc("/api/doctor", func(w http.ResponseWriter, r *http.Request) {
		probeMCP := r.URL.Query().Get("probe_mcp") == "true"
		runSelf := r.URL.Query().Get("self_test") == "true"
		ws := r.URL.Query().Get("workspace")
		if ws == "" {
			ws = workspace
		}

		if runSelf {
			if err := doctor.SelfTest(); err != nil {
				Log("WARN", "doctor", fmt.Sprintf("Self-test issue: %v", err))
			} else {
				Log("INFO", "doctor", "Self-test: PASS")
			}
		}

		report := doctor.RunAdvanced(p, ws, probeMCP)
		Log("INFO", "doctor", fmt.Sprintf("Doctor completed: %d findings (errors: %v)", len(report.Findings), report.HasErrors()))
		writeJSON(w, http.StatusOK, report)
	})

	// Loop control
	mux.HandleFunc("/api/loop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			cfg, err := loop.Load(p)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, cfg)
			return
		}

		if r.Method == http.MethodPost {
			var body struct {
				Action     string `json:"action"`     // "enable" or "disable"
				Profile    string `json:"profile"`    // "standard", "deep", "until-done", "unrestricted"
				RiskAccept bool   `json:"riskAccept"` // for unrestricted
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}

			if body.Action == "disable" {
				if err := loop.Disable(p); err != nil {
					writeError(w, http.StatusInternalServerError, err.Error())
					return
				}
				cfg, _ := loop.Load(p)
				Log("INFO", "loop", "Autonomous completion loop disabled")
				writeJSON(w, http.StatusOK, cfg)
				return
			}

			if body.Profile == "unrestricted" && !body.RiskAccept {
				writeError(w, http.StatusBadRequest, "unrestricted profile requires explicit risk acknowledgement")
				return
			}

			cfg, err := loop.EnableProfile(p, body.Profile)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			Log("INFO", "loop", fmt.Sprintf("Autonomous loop enabled with profile '%s' (max: %d)", body.Profile, cfg.MaxExecutions))
			writeJSON(w, http.StatusOK, cfg)
			return
		}

		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	// Tasks
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			ts, err := tasks.List(p)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, ts)
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	mux.HandleFunc("/api/tasks/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}
		var body struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Kind        string `json:"kind"`
			Priority    string `json:"priority"`
			Workspace   string `json:"workspace"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		ws := body.Workspace
		if ws == "" {
			ws = workspace
		}
		desc := body.Description
		if desc == "" {
			desc = body.Title
		}
		if desc == "" {
			writeError(w, http.StatusBadRequest, "task description required")
			return
		}
		pri := 0
		if body.Priority == "high" {
			pri = 10
		} else if body.Priority == "low" {
			pri = -5
		}
		item, err := tasks.Add(p, desc, ws, pri, false, "", []string{body.Kind})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		Log("INFO", "tasks", fmt.Sprintf("Task added: #%s %s", item.ID, item.Prompt))
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("/api/tasks/action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}
		var body struct {
			Action string `json:"action"` // "run", "cancel", "retry", "remove"
			TaskID string `json:"taskId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}

		switch body.Action {
		case "cancel":
			if err := tasks.Cancel(p, body.TaskID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "tasks", fmt.Sprintf("Task #%s cancelled", body.TaskID))
			writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
		case "retry":
			rec, err := tasks.Retry(p, body.TaskID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "tasks", fmt.Sprintf("Task #%s reset for retry", rec.ID))
			writeJSON(w, http.StatusOK, map[string]string{"status": "retrying"})
		case "remove":
			if err := tasks.Remove(p, body.TaskID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "tasks", fmt.Sprintf("Task #%s removed", body.TaskID))
			writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
		case "run":
			go func() {
				Log("INFO", "tasks", fmt.Sprintf("Executing task #%s in background...", body.TaskID))
				if _, err := tasks.Run(p, body.TaskID); err != nil {
					Log("ERROR", "tasks", fmt.Sprintf("Task #%s failed: %v", body.TaskID, err))
				} else {
					Log("INFO", "tasks", fmt.Sprintf("Task #%s completed successfully", body.TaskID))
				}
			}()
			writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
		default:
			writeError(w, http.StatusBadRequest, "unknown action")
		}
	})

	// Skills
	mux.HandleFunc("/api/skills", func(w http.ResponseWriter, r *http.Request) {
		sks, err := skills.List(p)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, sks)
	})

	mux.HandleFunc("/api/skills/action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}
		var body struct {
			Action string `json:"action"` // "sync-pack", "sync-recommended", "install-embedded", "remove"
			Name   string `json:"name"`
			PackID string `json:"packId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}

		switch body.Action {
		case "sync-recommended":
			res, err := skills.SyncRecommended(p)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "skills", fmt.Sprintf("Sync recommended: %v", res))
			writeJSON(w, http.StatusOK, map[string]any{"result": res})
		case "sync-pack":
			synced, err := skills.SyncPack(p, body.PackID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "skills", fmt.Sprintf("Synced %d skills from pack '%s'", len(synced), body.PackID))
			writeJSON(w, http.StatusOK, map[string]any{"synced": len(synced)})
		case "install-embedded":
			if err := skills.InstallEmbedded(p); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "skills", "Embedded core skills installed")
			writeJSON(w, http.StatusOK, map[string]string{"status": "installed"})
		case "remove":
			if err := skills.Remove(p, body.Name); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "skills", fmt.Sprintf("Skill '%s' removed", body.Name))
			writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
		default:
			writeError(w, http.StatusBadRequest, "unknown skill action")
		}
	})

	// MCP
	mux.HandleFunc("/api/mcp", func(w http.ResponseWriter, r *http.Request) {
		names, err := mcp.Names(p, workspace)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		doctorMsgs := mcp.Doctor(p, workspace)
		writeJSON(w, http.StatusOK, map[string]any{
			"servers": names,
			"doctor":  doctorMsgs,
		})
	})

	mux.HandleFunc("/api/mcp/probe", func(w http.ResponseWriter, r *http.Request) {
		server := r.URL.Query().Get("server")
		timeout := 12 * time.Second
		if server == "" {
			res := mcpprobe.ProbeAll(p, workspace, timeout)
			Log("INFO", "mcp", fmt.Sprintf("Probed all MCP servers (%d results)", len(res)))
			writeJSON(w, http.StatusOK, res)
			return
		}
		res, err := mcpprobe.ProbeConfigured(p, workspace, server, timeout)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/api/mcp/action", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}
		var body struct {
			Action string `json:"action"` // "add", "remove"
			Name   string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		switch body.Action {
		case "add":
			if err := mcp.Install(p, workspace, body.Name); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "mcp", fmt.Sprintf("MCP server '%s' added", body.Name))
			writeJSON(w, http.StatusOK, map[string]string{"status": "added"})
		case "remove":
			if err := mcp.Remove(p, workspace, body.Name); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "mcp", fmt.Sprintf("MCP server '%s' removed", body.Name))
			writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
		default:
			writeError(w, http.StatusBadRequest, "unknown action")
		}
	})

	// Plugins
	mux.HandleFunc("/api/plugins", func(w http.ResponseWriter, r *http.Request) {
		plugs, err := plugin.List(p, workspace)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, plugs)
	})

	// Permissions
	mux.HandleFunc("/api/permissions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			// List standard profiles
			profiles := []string{"safe", "balanced", "autonomous", "yolo"}
			res := make([]permissions.Profile, 0, len(profiles))
			for _, pr := range profiles {
				if pObj, err := permissions.GetProfile(pr); err == nil {
					res = append(res, pObj)
				}
			}
			writeJSON(w, http.StatusOK, res)
			return
		}

		if r.Method == http.MethodPost {
			var body struct {
				Profile    string `json:"profile"`
				RiskAccept bool   `json:"riskAccept"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeError(w, http.StatusBadRequest, "invalid body")
				return
			}
			if body.Profile == "yolo" && !body.RiskAccept {
				writeError(w, http.StatusBadRequest, "yolo profile requires explicit risk acknowledgement")
				return
			}
			if err := permissions.Apply(p, body.Profile); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "permissions", fmt.Sprintf("Permission profile '%s' applied", body.Profile))
			writeJSON(w, http.StatusOK, map[string]string{"status": "applied"})
			return
		}

		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	// Security audit
	mux.HandleFunc("/api/security", func(w http.ResponseWriter, r *http.Request) {
		sec, err := securityaudit.Audit(p, workspace)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, sec)
	})

	// Backups
	mux.HandleFunc("/api/backups", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			list, err := backup.List(p)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, list)
			return
		}
		if r.Method == http.MethodPost {
			created, err := backup.CreateAll(p, workspace)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			Log("INFO", "backup", fmt.Sprintf("Created %d backup snapshots", len(created)))
			writeJSON(w, http.StatusOK, map[string]any{"created": created})
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	// Replan & Router
	mux.HandleFunc("/api/replan", func(w http.ResponseWriter, r *http.Request) {
		stat, err := replan.Status(p, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, stat)
	})

	mux.HandleFunc("/api/router", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			stat, err := router.Load(p)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, stat)
			return
		}
		if r.Method == http.MethodPost {
			var body struct {
				Mode string `json:"mode"` // "full", "lite", "disabled"
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				writeError(w, http.StatusBadRequest, "invalid body")
				return
			}
			if body.Mode == "disabled" {
				_ = router.Disable(p)
				Log("INFO", "router", "Adaptive router disabled")
			} else {
				_ = router.Enable(p, body.Mode)
				Log("INFO", "router", fmt.Sprintf("Adaptive router mode set to '%s'", body.Mode))
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	})

	// Install Action
	mux.HandleFunc("/api/install", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "POST required")
			return
		}
		var body struct {
			Profile string `json:"profile"` // "recommended", "full", "self"
			Prereqs bool   `json:"prereqs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}

		Log("INFO", "install", fmt.Sprintf("Starting %s installation...", body.Profile))
		switch body.Profile {
		case "self":
			bin, err := installer.InstallSelf(p)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			_ = installer.TouchManifest(p, AppVersion)
			Log("INFO", "install", fmt.Sprintf("Self-install complete: %s", bin))
			writeJSON(w, http.StatusOK, map[string]string{"binary": bin})
		case "recommended":
			rep, err := installer.Recommended(p, body.Prereqs)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			_ = installer.TouchManifest(p, AppVersion)
			Log("INFO", "install", fmt.Sprintf("Recommended install complete (%v)", rep.SkillPackCounts))
			writeJSON(w, http.StatusOK, rep)
		case "full":
			rep, err := installer.Full(p, body.Prereqs)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			_ = installer.TouchManifest(p, AppVersion)
			Log("INFO", "install", fmt.Sprintf("Full install complete (%v)", rep.SkillPackCounts))
			writeJSON(w, http.StatusOK, rep)
		default:
			writeError(w, http.StatusBadRequest, "unknown install profile")
		}
	})

	// Root single-page application
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		_, _ = w.Write([]byte(IndexHTML))
	})

	return mux
}

func handleSSEStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	msgChan := make(chan []byte, 50)
	globalBroker.addClient(msgChan)
	defer globalBroker.removeClient(msgChan)

	// Send initial greeting
	initMsg, _ := json.Marshal(LogEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     "INFO",
		Source:    "stream",
		Message:   "Connected to agctl Web Control Plane live stream",
	})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", string(initMsg))
	flusher.Flush()

	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			// keepalive comment
			_, _ = fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case msg, ok := <-msgChan:
			if !ok {
				return
			}
			_, _ = w.Write(msg)
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

var (
	_ = strconv.Itoa
	_ = strings.TrimSpace
)
