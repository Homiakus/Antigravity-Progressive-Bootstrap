package tasks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/homiakus/agctl/internal/goal"
	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/telemetry"
)

const (
	StatusQueued     = "queued"
	StatusRunning    = "running"
	StatusSucceeded  = "succeeded"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
	StatusBlocked    = "blocked"
	StatusSuperseded = "superseded"
)

func DefaultConfig() model.TaskSupervisorConfig {
	return model.TaskSupervisorConfig{MaxParallel: 2, CPUWeight: 100, BuildSlots: 1, BrowserSlots: 1, MaxTaskMinutes: 120, MaxRetries: 1}
}

func LoadConfig(p paths.Paths) (model.TaskSupervisorConfig, error) {
	cfg := DefaultConfig()
	if _, err := os.Stat(p.TaskConfig); errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	loaded, err := jsonx.Read(p.TaskConfig, cfg)
	if err != nil {
		return cfg, err
	}
	cfg = loaded
	if cfg.MaxParallel < 1 {
		cfg.MaxParallel = 1
	}
	return cfg, nil
}

func SaveConfig(p paths.Paths, cfg model.TaskSupervisorConfig) error {
	if cfg.MaxParallel < 1 {
		return fmt.Errorf("maxParallel must be >= 1")
	}
	return jsonx.WriteAtomic(p.TaskConfig, cfg, p.BackupsRoot)
}

type Spec struct {
	Prompt         string
	Workspace      string
	Priority       int
	NativeGoal     bool
	Agent          string
	Tags           []string
	PlanID         string
	NodeID         string
	Dependencies   []string
	Resources      model.ResourceRequest
	Revision       int
	DynamicDepth   int
	ParentTaskID   string
	BaseWorkspace  string
	WorktreeBranch string
}

func Add(p paths.Paths, prompt, workspace string, priority int, nativeGoal bool, agent string, tags []string) (model.TaskRecord, error) {
	return AddAdvanced(p, Spec{Prompt: prompt, Workspace: workspace, Priority: priority, NativeGoal: nativeGoal, Agent: agent, Tags: tags})
}

func AddAdvanced(p paths.Paths, spec Spec) (model.TaskRecord, error) {
	prompt := strings.TrimSpace(spec.Prompt)
	if prompt == "" {
		return model.TaskRecord{}, fmt.Errorf("prompt is required")
	}
	workspace := spec.Workspace
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	abs, err := filepath.Abs(workspace)
	if err != nil {
		return model.TaskRecord{}, err
	}
	if st, err := os.Stat(abs); err != nil || !st.IsDir() {
		return model.TaskRecord{}, fmt.Errorf("workspace does not exist or is not a directory: %s", abs)
	}
	now := time.Now().UTC()
	id := fmt.Sprintf("%s-%06d", now.Format("20060102T150405.000000000Z"), now.UnixNano()%1000000)
	r := normalizeResources(spec.Resources)
	rec := model.TaskRecord{
		ID:             id,
		Prompt:         prompt,
		Workspace:      abs,
		PlanID:         strings.TrimSpace(spec.PlanID),
		NodeID:         strings.TrimSpace(spec.NodeID),
		Dependencies:   uniqueStrings(spec.Dependencies),
		Resources:      r,
		Revision:       spec.Revision,
		DynamicDepth:   spec.DynamicDepth,
		ParentTaskID:   strings.TrimSpace(spec.ParentTaskID),
		BaseWorkspace:  strings.TrimSpace(spec.BaseWorkspace),
		WorktreeBranch: strings.TrimSpace(spec.WorktreeBranch),
		Status:         StatusQueued,
		Priority:       spec.Priority,
		CreatedAt:      now.Format(time.RFC3339Nano),
		UpdatedAt:      now.Format(time.RFC3339Nano),
		UseNativeGoal:  spec.NativeGoal,
		Agent:          strings.TrimSpace(spec.Agent),
		Tags:           append([]string(nil), spec.Tags...),
	}
	if rec.PlanID != "" && p.ReplanInbox != "" {
		rec.ReplanProposalPath = filepath.Join(p.ReplanInbox, rec.ID+".json")
	}
	if err := save(p, rec); err != nil {
		return model.TaskRecord{}, err
	}
	_ = telemetry.Record(p, telemetry.Event{Type: "task.queued", Data: map[string]any{"taskId": rec.ID, "planId": rec.PlanID, "nodeId": rec.NodeID, "agent": rec.Agent, "priority": rec.Priority}})
	return rec, nil
}

func taskPath(p paths.Paths, id string) string { return filepath.Join(p.TasksRoot, id+".json") }

func Load(p paths.Paths, id string) (model.TaskRecord, error) {
	rec, err := jsonx.Read(taskPath(p, id), model.TaskRecord{})
	if err != nil {
		return rec, err
	}
	if rec.ID == "" {
		return rec, fmt.Errorf("task %s not found", id)
	}
	return rec, nil
}

func save(p paths.Paths, rec model.TaskRecord) error {
	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return jsonx.WriteAtomic(taskPath(p, rec.ID), rec, "")
}

// SaveRecord persists a task after a control-plane mutation such as adaptive replanning.
func SaveRecord(p paths.Paths, rec model.TaskRecord) error { return save(p, rec) }

// ReplaceDependency rewires queued/blocked tasks in one plan from oldTaskID to
// replacement task IDs. Running/finished tasks are never mutated.
func ReplaceDependency(p paths.Paths, planID, oldTaskID string, replacements []string) ([]string, error) {
	xs, err := List(p)
	if err != nil {
		return nil, err
	}
	replacements = uniqueStrings(replacements)
	var changed []string
	for _, rec := range xs {
		if rec.ParentTaskID == oldTaskID {
			continue
		}
		if rec.PlanID != planID || (rec.Status != StatusQueued && rec.Status != StatusBlocked) {
			continue
		}
		found := false
		var deps []string
		for _, d := range rec.Dependencies {
			if d == oldTaskID {
				deps = append(deps, replacements...)
				found = true
			} else {
				deps = append(deps, d)
			}
		}
		if !found {
			continue
		}
		rec.Dependencies = uniqueStrings(deps)
		if rec.Status == StatusBlocked && strings.Contains(rec.Error, oldTaskID) {
			rec.Status = StatusQueued
			rec.Error = ""
			rec.FinishedAt = ""
		}
		if err := save(p, rec); err != nil {
			return changed, err
		}
		changed = append(changed, rec.ID)
	}
	return changed, nil
}

func List(p paths.Paths) ([]model.TaskRecord, error) {
	entries, err := os.ReadDir(p.TasksRoot)
	if err != nil {
		return nil, err
	}
	out := make([]model.TaskRecord, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		rec, err := jsonx.Read(filepath.Join(p.TasksRoot, e.Name()), model.TaskRecord{})
		if err == nil && rec.ID != "" {
			out = append(out, rec)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Status == StatusQueued && out[j].Status == StatusQueued && out[i].Priority != out[j].Priority {
			return out[i].Priority > out[j].Priority
		}
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out, nil
}

func Run(p paths.Paths, id string) (model.TaskRecord, error) {
	rec, err := Load(p, id)
	if err != nil {
		return rec, err
	}
	if rec.Status == StatusCancelled {
		return rec, fmt.Errorf("task %s is cancelled", id)
	}
	return runRecord(context.Background(), p, rec)
}

type Observer func(model.TaskRecord) (handled bool, err error)

func RunPending(p paths.Paths) ([]model.TaskRecord, error) { return RunPendingObserved(p, nil) }

func RunPendingObserved(p paths.Paths, observer Observer) ([]model.TaskRecord, error) {
	cfg, err := LoadConfig(p)
	if err != nil {
		return nil, err
	}
	var results []model.TaskRecord
	var firstErr error
	var observerMu sync.Mutex

	for {
		all, err := List(p)
		if err != nil {
			return results, err
		}
		byID := make(map[string]model.TaskRecord, len(all))
		for _, rec := range all {
			byID[rec.ID] = rec
		}

		// Dependency failures are terminal for downstream nodes. Mark them
		// explicitly rather than leaving them queued forever.
		changed := false
		for _, rec := range all {
			if rec.Status != StatusQueued || len(rec.Dependencies) == 0 {
				continue
			}
			for _, depID := range rec.Dependencies {
				dep, ok := byID[depID]
				if !ok || dep.Status == StatusFailed || dep.Status == StatusCancelled || dep.Status == StatusBlocked {
					rec.Status = StatusBlocked
					rec.Error = "dependency did not complete successfully: " + depID
					rec.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
					_ = save(p, rec)
					_ = telemetry.Record(p, telemetry.Event{Type: "task.blocked", Reason: rec.Error, Data: map[string]any{"taskId": rec.ID, "planId": rec.PlanID, "nodeId": rec.NodeID, "dependency": depID}})
					changed = true
					break
				}
			}
		}
		if changed {
			continue
		}

		var ready []model.TaskRecord
		var running []model.TaskRecord
		for _, rec := range all {
			if rec.Status == StatusRunning {
				running = append(running, rec)
			}
			if rec.Status != StatusQueued || !dependenciesSatisfied(rec, byID) {
				continue
			}
			ready = append(ready, rec)
		}
		if len(ready) == 0 {
			break
		}
		sort.SliceStable(ready, func(i, j int) bool {
			if ready[i].Priority != ready[j].Priority {
				return ready[i].Priority > ready[j].Priority
			}
			return ready[i].CreatedAt < ready[j].CreatedAt
		})
		batch := selectBatchWithRunning(ready, cfg, running)
		if len(batch) == 0 {
			// A single task may request more than a configured resource budget.
			// Run it alone rather than deadlocking the entire DAG.
			batch = ready[:1]
		}

		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, rec := range batch {
			rec := rec
			wg.Add(1)
			go func() {
				defer wg.Done()
				r, err := runRecord(context.Background(), p, rec)
				requeued := false
				if err != nil && r.Attempts <= cfg.MaxRetries {
					r.Status = StatusQueued
					r.ProcessID = 0
					r.FinishedAt = ""
					r.Error = "retry after failure: " + err.Error()
					_ = save(p, r)
					_ = telemetry.Record(p, telemetry.Event{Type: "task.retry", Reason: err.Error(), Data: map[string]any{"taskId": r.ID, "attempt": r.Attempts, "maxRetries": cfg.MaxRetries}})
					requeued = true
				}
				handled := false
				var obsErr error
				if observer != nil && !requeued {
					observerMu.Lock()
					handled, obsErr = observer(r)
					observerMu.Unlock()
					if handled {
						if refreshed, loadErr := Load(p, r.ID); loadErr == nil {
							r = refreshed
						}
					}
				}
				mu.Lock()
				defer mu.Unlock()
				results = append(results, r)
				if obsErr != nil && firstErr == nil {
					firstErr = obsErr
				}
				if err != nil && r.Status != StatusQueued && !handled && firstErr == nil {
					firstErr = err
				}
			}()
		}
		wg.Wait()
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].CreatedAt < results[j].CreatedAt })
	return results, firstErr
}

func dependenciesSatisfied(rec model.TaskRecord, byID map[string]model.TaskRecord) bool {
	for _, depID := range rec.Dependencies {
		dep, ok := byID[depID]
		if !ok || dep.Status != StatusSucceeded {
			return false
		}
	}
	return true
}

func selectBatch(ready []model.TaskRecord, cfg model.TaskSupervisorConfig) []model.TaskRecord {
	return selectBatchWithRunning(ready, cfg, nil)
}

func selectBatchWithRunning(ready []model.TaskRecord, cfg model.TaskSupervisorConfig, running []model.TaskRecord) []model.TaskRecord {
	maxParallel := cfg.MaxParallel
	if maxParallel < 1 {
		maxParallel = 1
	}
	cpuCap := cfg.CPUWeight
	if cpuCap <= 0 {
		cpuCap = 100
	}
	buildCap := cfg.BuildSlots
	if buildCap < 0 {
		buildCap = 0
	}
	browserCap := cfg.BrowserSlots
	if browserCap < 0 {
		browserCap = 0
	}
	usedCPU, usedBuild, usedBrowser := 0, 0, 0
	workspaceExclusive := map[string]bool{}
	workspaceUsed := map[string]bool{}
	for _, rec := range running {
		r := normalizeResources(rec.Resources)
		usedCPU += r.CPUWeight
		usedBuild += r.BuildSlots
		usedBrowser += r.BrowserSlots
		workspaceUsed[rec.Workspace] = true
		if r.ExclusiveWorkspace {
			workspaceExclusive[rec.Workspace] = true
		}
	}
	availableParallel := maxParallel - len(running)
	if availableParallel <= 0 {
		return nil
	}
	var out []model.TaskRecord
	for _, rec := range ready {
		if len(out) >= availableParallel {
			break
		}
		r := normalizeResources(rec.Resources)
		if usedCPU+r.CPUWeight > cpuCap {
			continue
		}
		if r.BuildSlots > 0 && (buildCap == 0 || usedBuild+r.BuildSlots > buildCap) {
			continue
		}
		if r.BrowserSlots > 0 && (browserCap == 0 || usedBrowser+r.BrowserSlots > browserCap) {
			continue
		}
		if workspaceExclusive[rec.Workspace] || (r.ExclusiveWorkspace && workspaceUsed[rec.Workspace]) {
			continue
		}
		workspaceUsed[rec.Workspace] = true
		if r.ExclusiveWorkspace {
			workspaceExclusive[rec.Workspace] = true
		}
		usedCPU += r.CPUWeight
		usedBuild += r.BuildSlots
		usedBrowser += r.BrowserSlots
		out = append(out, rec)
	}
	return out
}

func normalizeResources(r model.ResourceRequest) model.ResourceRequest {
	if r.CPUWeight <= 0 {
		r.CPUWeight = 25
	}
	if r.BuildSlots < 0 {
		r.BuildSlots = 0
	}
	if r.BrowserSlots < 0 {
		r.BrowserSlots = 0
	}
	return r
}

func uniqueStrings(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func Cancel(p paths.Paths, id string) error {
	rec, err := Load(p, id)
	if err != nil {
		return err
	}
	if rec.Status == StatusSucceeded || rec.Status == StatusFailed || rec.Status == StatusCancelled {
		return nil
	}
	if rec.ProcessID > 0 {
		if proc, err := os.FindProcess(rec.ProcessID); err == nil {
			_ = proc.Kill()
		}
	}
	rec.Status = StatusCancelled
	rec.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	rec.Error = "cancelled by agctl"
	_ = os.Remove(claimPath(p, rec.ID))
	err = save(p, rec)
	_ = telemetry.Record(p, telemetry.Event{Type: "task.cancelled", Reason: rec.Error, Data: map[string]any{"taskId": rec.ID, "planId": rec.PlanID, "nodeId": rec.NodeID}})
	return err
}

func Retry(p paths.Paths, id string) (model.TaskRecord, error) {
	rec, err := Load(p, id)
	if err != nil {
		return rec, err
	}
	rec.Status = StatusQueued
	rec.ProcessID = 0
	rec.ExitCode = 0
	rec.Error = ""
	rec.StartedAt = ""
	rec.FinishedAt = ""
	if err := save(p, rec); err != nil {
		return rec, err
	}
	return rec, nil
}

func Remove(p paths.Paths, id string) error {
	rec, err := Load(p, id)
	if err == nil && rec.Status == StatusRunning {
		return fmt.Errorf("cannot remove running task; cancel it first")
	}
	return os.Remove(taskPath(p, id))
}

func runRecord(ctx context.Context, p paths.Paths, rec model.TaskRecord) (model.TaskRecord, error) {
	release, err := claimTask(p, rec.ID)
	if err != nil {
		return rec, err
	}
	defer release()

	agy := os.Getenv("AGCTL_AGY_COMMAND")
	if strings.TrimSpace(agy) == "" {
		agy = "agy"
	}
	if _, err := exec.LookPath(agy); err != nil {
		rec.Status = StatusFailed
		rec.Error = "agy executable not found: " + err.Error()
		rec.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_ = save(p, rec)
		return rec, errors.New(rec.Error)
	}

	cfg, _ := LoadConfig(p)
	if cfg.MaxTaskMinutes > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(cfg.MaxTaskMinutes)*time.Minute)
		defer cancel()
	}
	prompt := executionPrompt(rec)
	args := []string{"--mode=accept-edits", "-p", prompt, "--output-format", "stream-json"}
	// AGY headless defaults to a 5 minute print timeout. Keep it aligned with
	// the supervisor watchdog so a legitimate long-running DAG node is not
	// terminated by the CLI long before agctl's own timeout.
	if cfg.MaxTaskMinutes > 0 {
		args = append(args, "--print-timeout", fmt.Sprintf("%dm", cfg.MaxTaskMinutes))
	}
	if rec.Agent != "" {
		// Antigravity CLI headless mode supports selecting a discovered custom
		// agent explicitly with --agent. This is deterministic and fails loudly
		// when the requested agent is unknown instead of relying on prompt text.
		args = append(args, "--agent", rec.Agent)
	}

	logDir := filepath.Join(p.TelemetryRoot, "tasks")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return rec, err
	}
	logPath := filepath.Join(logDir, rec.ID+".jsonl")
	logFile, err := os.Create(logPath)
	if err != nil {
		return rec, err
	}
	defer logFile.Close()

	rec.Status = StatusRunning
	rec.StartedAt = time.Now().UTC().Format(time.RFC3339Nano)
	rec.Attempts++
	rec.OutputLog = logPath
	cmd := exec.CommandContext(ctx, agy, args...)
	cmd.Dir = rec.Workspace
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return rec, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return rec, err
	}
	if err := cmd.Start(); err != nil {
		rec.Status = StatusFailed
		rec.Error = err.Error()
		rec.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
		_ = save(p, rec)
		return rec, err
	}
	rec.ProcessID = cmd.Process.Pid
	_ = save(p, rec)
	_ = telemetry.Record(p, telemetry.Event{Type: "task.started", Data: map[string]any{"taskId": rec.ID, "planId": rec.PlanID, "nodeId": rec.NodeID, "agent": rec.Agent, "pid": rec.ProcessID}})

	var ioWG sync.WaitGroup
	var logMu sync.Mutex
	var streamMu sync.Mutex
	streamState := headlessStreamState{}
	copyStream := func(kind string, r io.Reader) {
		defer ioWG.Done()
		s := bufio.NewScanner(r)
		buf := make([]byte, 64*1024)
		s.Buffer(buf, 4*1024*1024)
		enc := json.NewEncoder(logFile)
		for s.Scan() {
			line := s.Text()
			logMu.Lock()
			_ = enc.Encode(map[string]any{"ts": time.Now().UTC().Format(time.RFC3339Nano), "stream": kind, "line": line})
			logMu.Unlock()

			streamMu.Lock()
			if kind == "stdout" {
				streamState.observeStdout(line)
			} else {
				streamState.observeStderr(line)
			}
			streamMu.Unlock()
		}
	}
	ioWG.Add(2)
	go copyStream("stdout", stdout)
	go copyStream("stderr", stderr)
	// StdoutPipe/StderrPipe must be fully drained before Wait closes the pipes.
	// Waiting for the readers first is safe: they reach EOF when the child exits.
	ioWG.Wait()
	waitErr := cmd.Wait()

	rec.ProcessID = 0
	rec.FinishedAt = time.Now().UTC().Format(time.RFC3339Nano)
	streamMu.Lock()
	observed := streamState
	streamMu.Unlock()

	runErr := waitErr
	if waitErr != nil {
		rec.Status = StatusFailed
		rec.Error = waitErr.Error()
		if ee := new(exec.ExitError); errors.As(waitErr, &ee) {
			rec.ExitCode = ee.ExitCode()
		} else {
			rec.ExitCode = -1
		}
	} else if observed.PermissionDenied {
		// Antigravity headless intentionally soft-denies tools that require an
		// interactive approval: the run may still exit 0. Treat that as a failed
		// DAG node so autonomous execution fixes its permission policy instead of
		// falsely claiming the requested work succeeded.
		rec.Status = StatusFailed
		rec.ExitCode = 0
		rec.Error = "Antigravity headless soft-denied a required tool permission"
		runErr = errors.New(rec.Error)
	} else if !observed.SawResult {
		rec.Status = StatusFailed
		rec.ExitCode = 0
		rec.Error = "Antigravity headless stream ended without a terminal result event"
		runErr = errors.New(rec.Error)
	} else if strings.ToUpper(observed.ResultStatus) != "SUCCESS" {
		rec.Status = StatusFailed
		rec.ExitCode = 0
		rec.Error = strings.TrimSpace(observed.ResultError)
		if rec.Error == "" {
			rec.Error = fmt.Sprintf("Antigravity headless terminal status: %s", observed.ResultStatus)
		}
		runErr = errors.New(rec.Error)
	} else {
		rec.Status = StatusSucceeded
		rec.ExitCode = 0
		rec.Error = ""
	}
	if err := save(p, rec); err != nil {
		return rec, err
	}
	durationMS := int64(0)
	if started, err := time.Parse(time.RFC3339Nano, rec.StartedAt); err == nil {
		durationMS = time.Since(started).Milliseconds()
	}
	_ = telemetry.Record(p, telemetry.Event{Type: "task.finished", Reason: rec.Error, Data: map[string]any{"taskId": rec.ID, "planId": rec.PlanID, "nodeId": rec.NodeID, "agent": rec.Agent, "status": rec.Status, "exitCode": rec.ExitCode, "durationMs": durationMS}})
	return rec, runErr
}

type headlessStreamState struct {
	SawResult        bool
	ResultStatus     string
	ResultError      string
	PermissionDenied bool
}

func (s *headlessStreamState) observeStdout(line string) {
	var event struct {
		Event      string `json:"event"`
		StepUpdate struct {
			StepType string `json:"step_type"`
			ToolInfo struct {
				Error *struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			} `json:"tool_info"`
		} `json:"step_update"`
		Result struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		} `json:"result"`
	}
	if json.Unmarshal([]byte(line), &event) != nil {
		return
	}
	if event.Event == "result" {
		s.SawResult = true
		s.ResultStatus = strings.TrimSpace(event.Result.Status)
		s.ResultError = strings.TrimSpace(event.Result.Error)
	}
	if event.Event == "step_update" && event.StepUpdate.StepType == "tool" && event.StepUpdate.ToolInfo.Error != nil {
		msg := event.StepUpdate.ToolInfo.Error.Type + " " + event.StepUpdate.ToolInfo.Error.Message
		if looksLikePermissionDenial(msg) {
			s.PermissionDenied = true
		}
	}
}

func (s *headlessStreamState) observeStderr(line string) {
	if looksLikePermissionDenial(line) {
		s.PermissionDenied = true
	}
}

func looksLikePermissionDenial(text string) bool {
	v := strings.ToLower(strings.TrimSpace(text))
	if v == "" {
		return false
	}
	if strings.Contains(v, "soft-denied") || strings.Contains(v, "soft denied") {
		return true
	}
	permissionContext := strings.Contains(v, "permission") || strings.Contains(v, "approval")
	denied := strings.Contains(v, "denied") || strings.Contains(v, "requires approval") || strings.Contains(v, "not allowed")
	return permissionContext && denied
}

func claimPath(p paths.Paths, id string) string { return filepath.Join(p.TasksRoot, id+".run.lock") }

func claimTask(p paths.Paths, id string) (func(), error) {
	path := claimPath(p, id)
	if st, err := os.Stat(path); err == nil {
		// A crashed runner must not block a task forever. Six hours is longer
		// than the default task watchdog and conservative for long builds.
		if time.Since(st.ModTime()) > 6*time.Hour {
			_ = os.Remove(path)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("task %s is already claimed by another runner", id)
		}
		return nil, err
	}
	_, _ = fmt.Fprintf(f, "pid=%d\nclaimedAt=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
	_ = f.Close()
	return func() { _ = os.Remove(path) }, nil
}

func executionPrompt(rec model.TaskRecord) string {
	base := strings.TrimSpace(rec.Prompt)
	prefix := "Execute the following delegated task to verified completion without asking for intermediate user input. Continue through implementation, testing, diagnosis, fixes, and final verification. Use native subagents/worktrees when useful and obey agctl completion hooks."
	if rec.ReplanProposalPath != "" {
		prefix += "\n\nADAPTIVE REPLANNING PROTOCOL:\nIf this node discovers MATERIAL new work that is required for the original goal but is not represented by this node, write exactly one JSON proposal to: " + rec.ReplanProposalPath + "\nDo not create a proposal for optional polish. Base it on concrete evidence. Schema:\n{\"version\":1,\"planId\":" + quoteJSON(rec.PlanID) + ",\"parentNodeId\":" + quoteJSON(rec.NodeID) + ",\"parentTaskId\":" + quoteJSON(rec.ID) + ",\"reason\":\"why DAG must change\",\"evidence\":[\"concrete evidence\"],\"confidence\":0.0,\"actions\":[{\"id\":\"short-id\",\"title\":\"...\",\"objective\":\"...\",\"agent\":\"implementer|test-engineer|researcher|security-reviewer|architect|code-reviewer\",\"dependsOn\":[],\"verification\":[\"...\"],\"tags\":[\"...\"],\"risk\":\"read-low|write-medium|execution-high\",\"parallelizable\":false}]}\nAction dependsOn values refer to other action IDs in the same proposal; empty means depend on this node. If no material DAG change is required, do not write the proposal file."
	}
	body := prefix + "\n\nTASK:\n" + base
	if rec.UseNativeGoal {
		wrapped, err := goal.HeadlessPrompt(body)
		if err == nil {
			return wrapped
		}
	}
	return body
}

func quoteJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
