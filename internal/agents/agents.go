package agents

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

const (
	ModeOff      = "off"
	ModeBalanced = "balanced"
	ModeParallel = "parallel"
	ModeMaximum  = "maximum"
)

type Item struct{ Name, Path, Scope, Description string }

type Definition struct {
	Name        string
	Description string
	Model       string
	Tools       []string
	Skills      []string
	Body        string
}

var Embedded = []Definition{
	{Name: "architect", Description: "Architecture specialist for boundaries, contracts, tradeoffs, migrations, and system-level risk before complex implementation.", Model: "pro", Tools: []string{"view_file", "grep_search", "find_by_name", "run_command"}, Body: "Analyze architecture, interfaces, state, failure modes and migration risks. Produce concrete findings for the parent agent. Do not rewrite unrelated code."},
	{Name: "implementer", Description: "Focused implementation specialist for coherent production changes with minimal unrelated scope.", Model: "flash", Tools: []string{"view_file", "grep_search", "replace_file_content", "multi_replace_file_content", "write_to_file", "run_command", "manage_task"}, Body: "Implement assigned changes completely. Follow repository conventions, run targeted verification, and report exact files/checks to the parent."},
	{Name: "test-engineer", Description: "Independent verification specialist for unit, integration, regression, browser and failure-path testing.", Model: "flash", Tools: []string{"view_file", "grep_search", "run_command", "manage_task"}, Body: "Verify behavior independently. Prefer reproducible tests and concrete evidence. Do not mark success when checks are skipped."},
	{Name: "security-reviewer", Description: "Security specialist for trust boundaries, secrets, permissions, injection, supply-chain and destructive operations.", Model: "pro", Tools: []string{"view_file", "grep_search", "run_command"}, Body: "Review the requested change for exploitable behavior and unsafe defaults. Prioritize concrete attack paths and remediation."},
	{Name: "code-reviewer", Description: "Independent code-review specialist for correctness, edge cases, maintainability and regression risk.", Model: "pro", Tools: []string{"view_file", "grep_search", "run_command"}, Body: "Review the final diff independently. Focus on bugs, broken contracts, missing tests and unintended behavior rather than style trivia."},
	{Name: "researcher", Description: "Read-focused research specialist for current documentation, source comparison and evidence gathering.", Model: "flash", Tools: []string{"view_file", "grep_search", "search_web", "read_url_content"}, Body: "Gather authoritative evidence and concise conclusions. Prefer primary sources and clearly separate facts from inference."},
}

func DefaultConfig() model.OrchestratorConfig {
	return model.OrchestratorConfig{Enabled: true, Mode: ModeBalanced, MaxParallel: 3, PreferWorktrees: true, AutoRoles: []string{"architect", "test-engineer", "code-reviewer"}, UseNativeGoal: true, VerificationAgent: true, SecurityReviewAgent: true}
}
func LoadConfig(p paths.Paths) (model.OrchestratorConfig, error) {
	c, e := jsonx.Read(p.OrchestratorConfig, DefaultConfig())
	if c.Mode == "" {
		c = DefaultConfig()
	}
	return c, e
}
func SaveConfig(p paths.Paths, c model.OrchestratorConfig) error {
	if c.MaxParallel < 1 {
		c.MaxParallel = 1
	}
	return jsonx.WriteAtomic(p.OrchestratorConfig, c, p.BackupsRoot)
}
func Enable(p paths.Paths, mode string) error {
	c, _ := LoadConfig(p)
	switch mode {
	case ModeBalanced:
		c.Enabled = true
		c.Mode = mode
		c.MaxParallel = 3
	case ModeParallel:
		c.Enabled = true
		c.Mode = mode
		c.MaxParallel = 4
	case ModeMaximum:
		c.Enabled = true
		c.Mode = mode
		c.MaxParallel = 6
	case ModeOff:
		c.Enabled = false
		c.Mode = mode
	default:
		return fmt.Errorf("invalid orchestrator mode %q", mode)
	}
	return SaveConfig(p, c)
}

func InstallEmbedded(p paths.Paths, workspace string) error {
	root := p.GlobalAgentsRoot
	scope := "global"
	if strings.TrimSpace(workspace) != "" {
		root = paths.WorkspaceAgents(workspace)
		scope = "workspace"
		_ = scope
	}
	for _, d := range Embedded {
		dir := filepath.Join(root, d.Name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "agent.md"), []byte(render(d)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func render(d Definition) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\nmodel: %s\nmainAgent: false\nsubagent: true\ncommandExecutionPolicy: auto\ntools:\n", d.Name, d.Description, d.Model)
	for _, x := range d.Tools {
		fmt.Fprintf(&b, "  - %s\n", x)
	}
	if len(d.Skills) > 0 {
		b.WriteString("skills:\n")
		for _, x := range d.Skills {
			fmt.Fprintf(&b, "  - %s\n", x)
		}
	}
	b.WriteString("---\n\n# Core instructions\n\n")
	b.WriteString(d.Body)
	b.WriteString("\n\nReturn concise evidence and actionable findings to the parent agent.\n")
	return b.String()
}

func List(p paths.Paths, workspace string) ([]Item, error) {
	roots := []struct{ path, scope string }{{p.GlobalAgentsRoot, "global"}}
	if workspace != "" {
		roots = append(roots, struct{ path, scope string }{paths.WorkspaceAgents(workspace), "workspace"})
	}
	var out []Item
	for _, r := range roots {
		es, err := os.ReadDir(r.path)
		if err != nil {
			continue
		}
		for _, e := range es {
			var f, fallback string
			if e.IsDir() {
				f = filepath.Join(r.path, e.Name(), "agent.md")
				fallback = e.Name()
			} else if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
				f = filepath.Join(r.path, e.Name())
				fallback = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			} else {
				continue
			}
			if _, err := os.Stat(f); err != nil {
				continue
			}
			name, desc := frontmatter(f)
			if name == "" {
				name = fallback
			}
			out = append(out, Item{Name: name, Path: f, Scope: r.scope, Description: desc})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name == out[j].Name {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func Doctor(p paths.Paths, workspace string) []string {
	xs, _ := List(p, workspace)
	var out []string
	validTools := map[string]bool{
		"view_file": true, "write_to_file": true, "replace_file_content": true, "multi_replace_file_content": true,
		"list_dir": true, "find_by_name": true, "grep_search": true, "search_web": true, "read_url_content": true,
		"run_command": true, "manage_task": true, "schedule": true, "list_permissions": true, "ask_permission": true,
		"invoke_subagent": true, "define_subagent": true, "send_message": true, "manage_subagents": true,
		"ask_question": true, "generate_image": true,
	}
	for _, x := range xs {
		status := "OK"
		name, desc := frontmatter(x.Path)
		b, err := os.ReadFile(x.Path)
		if err != nil || name == "" || desc == "" {
			status = "INVALID FRONTMATTER"
		} else {
			text := string(b)
			if strings.Contains(text, "\npermissionMode:") {
				status = "UNSUPPORTED FIELD permissionMode"
			}
			if policy := frontmatterScalar(x.Path, "commandExecutionPolicy"); policy != "" &&
				policy != "off" && policy != "auto" && policy != "eager" && policy != "sandbox" {
				status = "INVALID commandExecutionPolicy " + policy
			}
			if modelName := frontmatterScalar(x.Path, "model"); modelName != "" &&
				modelName != "inherit" && modelName != "flash" && modelName != "pro" {
				status = "INVALID model " + modelName
			}
			for _, tool := range frontmatterList(x.Path, "tools") {
				if !validTools[tool] {
					status = "INVALID TOOL " + tool
					break
				}
			}
		}
		out = append(out, fmt.Sprintf("%-20s %-10s %s", x.Name, x.Scope, status))
	}
	sort.Strings(out)
	return out
}

func frontmatterScalar(path, key string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(b), "\n")
	in := false
	prefix := key + ":"
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "---" {
			if !in {
				in = true
				continue
			}
			break
		}
		if !in {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			return strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, prefix)), `"'`)
		}
	}
	return ""
}

func frontmatterList(path, key string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	inFrontmatter, inList := false, false
	var out []string
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}
		if !inFrontmatter {
			continue
		}
		if strings.HasPrefix(line, key+":") {
			inList = true
			continue
		}
		if inList {
			if strings.HasPrefix(line, "- ") {
				out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "- ")))
				continue
			}
			if line != "" && !strings.HasPrefix(line, "#") {
				break
			}
		}
	}
	return out
}

func OrchestrationInjection(p paths.Paths, taskHint string) string {
	c, err := LoadConfig(p)
	if err != nil || !c.Enabled {
		return ""
	}
	roles := append([]string(nil), c.AutoRoles...)
	if strings.Contains(strings.ToLower(taskHint), "security") {
		roles = append(roles, "security-reviewer")
	}
	return fmt.Sprintf(`SUBAGENT ORCHESTRATION POLICY
Native Antigravity subagents are available through invoke_subagent/define_subagent. For genuinely separable complex work, delegate independent investigations/verification in parallel rather than serializing everything in the main context.
Orchestrator mode: %s; maxParallel=%d; prefer isolated worktrees=%v.
Preferred roles: %s.
Do not spawn subagents for tiny/local edits. Avoid concurrent writes to the same files. Use a test/review agent independently before verified completion when the task is substantial.`, c.Mode, c.MaxParallel, c.PreferWorktrees, strings.Join(unique(roles), ", "))
}

func frontmatter(path string) (name, desc string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(b), "\n")
	in := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "---" {
			if !in {
				in = true
				continue
			}
			break
		}
		if !in {
			continue
		}
		if strings.HasPrefix(line, "name:") {
			name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "name:")), "\"'")
		}
		if strings.HasPrefix(line, "description:") {
			desc = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "description:")), "\"'")
		}
	}
	return
}
func unique(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
