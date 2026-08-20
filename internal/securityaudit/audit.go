package securityaudit

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/plugin"
	"github.com/homiakus/agctl/internal/provenance"
	"github.com/homiakus/agctl/internal/sidecar"
)

// Audit computes an explainable 0..100 control-plane security score.
// It is a governance signal, not a proof of safety.
func Audit(p paths.Paths, workspace string) (model.SecurityReport, error) {
	rep := model.SecurityReport{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Workspace: workspace, Score: 100}
	add := func(sev, area, id, msg string, penalty int) {
		rep.Findings = append(rep.Findings, model.SecurityFinding{Severity: sev, Area: area, ID: id, Message: msg, Penalty: penalty})
		rep.Score -= penalty
	}

	// Provenance integrity is the strongest signal because it detects local
	// modification after installation.
	verifications, err := provenance.VerifyAll(p)
	if err == nil {
		for _, v := range verifications {
			if v.OK {
				continue
			}
			if v.Unverifiable {
				add("medium", "provenance", v.ID, "installed component has an unverifiable provenance lock", 5)
			} else {
				add("critical", "provenance", v.ID, fmt.Sprintf("provenance mismatch: changed=%d missing=%d", len(v.Changed), len(v.Missing)), 25)
			}
		}
	}

	if err := auditMCP(p, workspace, add); err != nil {
		add("medium", "mcp", "config", err.Error(), 4)
	}
	auditSkills(skillRoots(p, workspace), add)
	for _, sc := range mustSidecars(p) {
		if !sc.Valid {
			add("high", "sidecar", sc.ID, "invalid sidecar: "+sc.Issue, 10)
			continue
		}
		if sc.Enabled {
			if sc.Builtin == "schedule" {
				add("medium", "sidecar", sc.ID, "enabled recurring scheduler can launch background commands", 4)
			} else {
				add("medium", "sidecar", sc.ID, "enabled persistent background process", 4)
			}
		}
	}

	for _, it := range plugin.Doctor(p, workspace) {
		if !it.Valid {
			add("high", "plugin", it.Name, "invalid plugin: "+strings.Join(it.Issues, "; "), 12)
		}
		componentSet := make(map[string]bool)
		for _, c := range it.Components {
			componentSet[c] = true
		}
		if componentSet["hooks"] {
			add("medium", "plugin", it.Name, "plugin installs execution hooks", 4)
		}
		if componentSet["mcp"] {
			add("medium", "plugin", it.Name, "plugin installs MCP configuration", 3)
		}
		if componentSet["sidecars"] {
			add("medium", "plugin", it.Name, "plugin installs persistent/scheduled sidecars", 4)
		}
	}

	if rep.Score < 0 {
		rep.Score = 0
	}
	rep.Grade = grade(rep.Score)
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		if severityRank(rep.Findings[i].Severity) != severityRank(rep.Findings[j].Severity) {
			return severityRank(rep.Findings[i].Severity) > severityRank(rep.Findings[j].Severity)
		}
		if rep.Findings[i].Area != rep.Findings[j].Area {
			return rep.Findings[i].Area < rep.Findings[j].Area
		}
		return rep.Findings[i].ID < rep.Findings[j].ID
	})
	if p.SecurityRoot != "" {
		name := "global-latest.json"
		if workspace != "" {
			name = "workspace-latest.json"
		}
		_ = jsonx.WriteAtomic(filepath.Join(p.SecurityRoot, name), rep, "")
	}
	return rep, nil
}

func auditMCP(p paths.Paths, workspace string, add func(string, string, string, string, int)) error {
	root, err := jsonx.ReadMap(mcp.ConfigPath(p, workspace))
	if err != nil {
		return err
	}
	servers, _ := root["mcpServers"].(map[string]any)
	for name, raw := range servers {
		if m, ok := raw.(map[string]any); ok {
			if _, legacy := m["url"]; legacy {
				add("high", "mcp", name, "legacy `url` is unsupported by current Antigravity MCP config; use `serverUrl`", 10)
			}
			if _, legacy := m["httpUrl"]; legacy {
				add("high", "mcp", name, "legacy `httpUrl` is unsupported by current Antigravity MCP config; use `serverUrl`", 10)
			}
		}
		b, _ := json.Marshal(raw)
		var s model.MCPServer
		if json.Unmarshal(b, &s) != nil {
			add("high", "mcp", name, "invalid MCP server structure", 10)
			continue
		}
		if s.Disabled {
			continue
		}
		if s.ServerURL != "" {
			u, err := url.Parse(s.ServerURL)
			if err != nil {
				add("high", "mcp", name, "invalid remote URL", 10)
				continue
			}
			if !strings.EqualFold(u.Scheme, "https") {
				add("critical", "mcp", name, "remote MCP does not use HTTPS", 20)
			} else {
				add("info", "mcp", name, "remote MCP expands network trust boundary", 1)
			}
		}
		cmd := strings.ToLower(s.Command + " " + strings.Join(s.Args, " "))
		if strings.Contains(cmd, "@latest") || strings.Contains(cmd, ":latest") {
			add("medium", "mcp", name, "unpinned package/image version", 5)
		}
		if strings.Contains(cmd, "npx") && strings.Contains(cmd, "-y") {
			add("low", "mcp", name, "npx auto-installs package at execution time", 2)
		}
		if len(s.Env) > 0 {
			add("low", "mcp", name, "server receives environment variables; verify least-privilege credentials", 1)
		}
	}
	return nil
}

func auditSkills(roots []string, add func(string, string, string, string, int)) {
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			scriptCount := 0
			executableCount := 0
			_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(dir, path)
				low := strings.ToLower(filepath.ToSlash(rel))
				if strings.HasPrefix(low, "scripts/") || strings.HasSuffix(low, ".ps1") || strings.HasSuffix(low, ".sh") || strings.HasSuffix(low, ".py") || strings.HasSuffix(low, ".js") {
					scriptCount++
				}
				if info.Mode()&0o111 != 0 {
					executableCount++
				}
				return nil
			})
			if scriptCount > 0 {
				add("medium", "skill", e.Name(), fmt.Sprintf("skill contains %d executable/script resources", scriptCount), 3)
			}
			if executableCount > 0 {
				add("medium", "skill", e.Name(), fmt.Sprintf("skill contains %d executable files", executableCount), 2)
			}
		}
	}
}

func skillRoots(p paths.Paths, workspace string) []string {
	out := []string{p.GlobalSkillsRoot}
	if strings.TrimSpace(workspace) != "" {
		out = append(out, paths.WorkspaceSkills(workspace))
	}
	return out
}

func grade(score int) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}
func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 5
	case "high":
		return 4
	case "medium":
		return 3
	case "low":
		return 2
	default:
		return 1
	}
}

// AssessRegistryServer scores raw MCP Registry metadata before installation.
// raw is intentionally accepted as an untyped map to keep this package decoupled
// from the Registry transport client.
func AssessRegistryServer(name, status string, raw map[string]any) model.SecurityReport {
	rep := model.SecurityReport{GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), Score: 100}
	add := func(sev, msg string, penalty int) {
		rep.Findings = append(rep.Findings, model.SecurityFinding{Severity: sev, Area: "registry", ID: name, Message: msg, Penalty: penalty})
		rep.Score -= penalty
	}
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "active", "published", "approved":
		// no penalty
	case "deprecated", "deleted", "inactive":
		add("high", "registry status is "+status, 20)
	case "":
		add("low", "registry status is not explicitly available", 3)
	default:
		add("low", "registry status is "+status, 2)
	}
	body := raw
	if x, ok := raw["server"].(map[string]any); ok {
		body = x
	}
	if remotes, ok := body["remotes"].([]any); ok {
		for _, x := range remotes {
			m, _ := x.(map[string]any)
			u := fmt.Sprint(m["url"])
			if u == "" {
				continue
			}
			parsed, err := url.Parse(u)
			if err != nil || !strings.EqualFold(parsed.Scheme, "https") {
				add("critical", "remote endpoint is not HTTPS: "+u, 30)
			}
			if strings.Contains(strings.ToLower(parsed.Host), "localhost") || parsed.Hostname() == "127.0.0.1" {
				add("info", "remote metadata points to localhost", 0)
			}
		}
	}
	if pkgs, ok := body["packages"].([]any); ok {
		for _, x := range pkgs {
			m, _ := x.(map[string]any)
			ver := strings.TrimSpace(fmt.Sprint(m["version"]))
			if ver == "" || strings.EqualFold(ver, "latest") || ver == "<nil>" {
				add("medium", "package version is not pinned", 8)
			}
			for _, key := range []string{"environmentVariables", "environment_variables"} {
				if envs, ok := m[key].([]any); ok && len(envs) > 0 {
					add("low", fmt.Sprintf("package requires %d environment variables/secrets", len(envs)), 2)
				}
			}
		}
	}
	if rep.Score < 0 {
		rep.Score = 0
	}
	rep.Grade = grade(rep.Score)
	return rep
}

func mustSidecars(p paths.Paths) []sidecar.Item { xs, _ := sidecar.List(p); return xs }
