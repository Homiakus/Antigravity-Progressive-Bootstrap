package capability

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/sidecar"
)

func Build(p paths.Paths, workspaces []string) (model.CapabilityRegistry, error) {
	var caps []model.Capability
	seen := map[string]struct{}{}
	add := func(c model.Capability) {
		key := c.Kind + ":" + c.Scope + ":" + c.ID
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		caps = append(caps, c)
	}

	discoverSkills(p.GlobalSkillsRoot, "global", add)
	// CLI global skills are intentionally flat mirrors of IDE/global skills in
	// agctl-managed installs. Suppress those mirrors when the canonical IDE
	// skill exists so ranking/planning does not count one capability twice.
	discoverCLISkills(p.CLISkillsRoot, p.GlobalSkillsRoot, "cli-global", add)
	discoverAgents(p.GlobalAgentsRoot, "global", add)
	discoverPlugins(p.GlobalPluginsRoot, "global", add)
	discoverPlugins(p.CLIPluginsRoot, "cli-global", add)
	discoverMCP(p.GlobalMCP, "global", add)
	if sidecars, err := sidecar.List(p); err == nil {
		for _, sc := range sidecars {
			ops := []string{"background-process"}
			if sc.Builtin == "schedule" {
				ops = append(ops, "schedule")
			}
			risk := "background-medium"
			if !sc.Enabled {
				risk = "disabled"
			}
			add(model.Capability{ID: sc.ID, Kind: "sidecar", Description: sc.Description, Scope: sc.Scope, Path: sc.Path, Enabled: sc.Enabled, Operations: ops, Risk: risk})
		}
	}
	for _, w := range workspaces {
		if strings.TrimSpace(w) == "" {
			continue
		}
		discoverSkills(paths.WorkspaceSkills(w), "workspace", add)
		discoverAgents(paths.WorkspaceAgents(w), "workspace", add)
		discoverPlugins(paths.WorkspacePlugins(w), "workspace", add)
		discoverMCP(paths.WorkspaceMCP(w), "workspace", add)
		discoverWorkflows(paths.WorkspaceWorkflows(w), "workspace", add)
	}

	for _, c := range nativeCapabilities() {
		add(c)
	}
	sort.Slice(caps, func(i, j int) bool {
		if caps[i].Kind != caps[j].Kind {
			return caps[i].Kind < caps[j].Kind
		}
		return caps[i].ID < caps[j].ID
	})
	reg := model.CapabilityRegistry{GeneratedAt: time.Now().Format(time.RFC3339Nano), Capabilities: caps}
	if err := jsonx.WriteAtomic(p.CapabilityDB, reg, p.BackupsRoot); err != nil {
		return reg, err
	}
	return reg, nil
}

func discoverCLISkills(root, canonicalIDERoot, scope string, add func(model.Capability)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		fallback := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		file := filepath.Join(root, e.Name())
		if _, err := os.Stat(file); err != nil {
			continue
		}
		name, desc := markdownFrontmatter(file)
		if name == "" {
			name = fallback
		}

		// Prefer the canonical IDE/global copy if agctl has installed the same
		// named skill there. A genuinely CLI-only skill remains discoverable.
		canonical := filepath.Join(canonicalIDERoot, name, "SKILL.md")
		if _, err := os.Stat(canonical); err == nil {
			continue
		}

		add(model.Capability{ID: name, Kind: "skill", Description: desc, Scope: scope, Path: file, Enabled: true, Domains: inferDomains(name + " " + desc), Operations: []string{"methodology", "guidance"}, Risk: "read-only"})
	}
}

func Load(p paths.Paths) (model.CapabilityRegistry, error) {
	return jsonx.Read(p.CapabilityDB, model.CapabilityRegistry{})
}

func Search(reg model.CapabilityRegistry, query string) []model.Capability {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return append([]model.Capability(nil), reg.Capabilities...)
	}
	var out []model.Capability
	for _, c := range reg.Capabilities {
		hay := strings.ToLower(strings.Join(append([]string{c.ID, c.Kind, c.Description}, append(c.Domains, c.Operations...)...), " "))
		if strings.Contains(hay, q) {
			out = append(out, c)
		}
	}
	return out
}

func discoverSkills(root, scope string, add func(model.Capability)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		var file, fallback string
		if e.IsDir() {
			file = filepath.Join(root, e.Name(), "SKILL.md")
			fallback = e.Name()
		} else if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			file = filepath.Join(root, e.Name())
			fallback = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		} else {
			continue
		}
		if _, err := os.Stat(file); err != nil {
			continue
		}
		name, desc := markdownFrontmatter(file)
		if name == "" {
			name = fallback
		}
		add(model.Capability{ID: name, Kind: "skill", Description: desc, Scope: scope, Path: file, Enabled: true, Domains: inferDomains(name + " " + desc), Operations: []string{"methodology", "guidance"}, Risk: "read-only"})
	}
}

func discoverAgents(root, scope string, add func(model.Capability)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		var file, fallback string
		if e.IsDir() {
			file = filepath.Join(root, e.Name(), "agent.md")
			fallback = e.Name()
		} else if strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			file = filepath.Join(root, e.Name())
			fallback = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		} else {
			continue
		}
		if _, err := os.Stat(file); err != nil {
			continue
		}
		name, desc := markdownFrontmatter(file)
		if name == "" {
			name = fallback
		}
		add(model.Capability{ID: name, Kind: "agent", Description: desc, Scope: scope, Path: file, Enabled: true, Domains: inferDomains(name + " " + desc), Operations: []string{"delegate", "parallelize"}, Risk: "agentic"})
	}
}

func discoverPlugins(root, scope string, add func(model.Capability)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		manifest := filepath.Join(root, e.Name(), "plugin.json")
		if _, err := os.Stat(manifest); err != nil {
			continue
		}
		m, _ := jsonx.Read(manifest, model.PluginManifest{})
		name := m.Name
		if name == "" {
			name = e.Name()
		}
		ops := []string{"bundle"}
		for _, pair := range []struct{ f, op string }{{"mcp_config.json", "mcp"}, {"hooks.json", "hooks"}, {"skills", "skills"}, {"agents", "agents"}, {"rules", "rules"}, {"sidecars", "sidecars"}} {
			if _, err := os.Stat(filepath.Join(root, e.Name(), pair.f)); err == nil {
				ops = append(ops, pair.op)
			}
		}
		pluginRoot := filepath.Join(root, e.Name())
		add(model.Capability{ID: name, Kind: "plugin", Description: "Antigravity plugin bundle", Scope: scope, Path: pluginRoot, Enabled: true, Operations: ops, Risk: "mixed"})
		discoverAgents(filepath.Join(pluginRoot, "agents"), scope+":plugin:"+name, add)
	}
}

func discoverMCP(path, scope string, add func(model.Capability)) {
	root, err := jsonx.ReadMap(path)
	if err != nil {
		return
	}
	servers, _ := root["mcpServers"].(map[string]any)
	for name, raw := range servers {
		m, _ := raw.(map[string]any)
		disabled, _ := m["disabled"].(bool)
		desc := "Configured MCP server"
		add(model.Capability{ID: name, Kind: "mcp", Description: desc, Scope: scope, Path: path, Enabled: !disabled, Domains: inferDomains(name), Operations: []string{"external-context", "tools"}, Risk: inferMCPRisk(name)})
	}
}

func discoverWorkflows(root, scope string, add func(model.Capability)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
			continue
		}
		file := filepath.Join(root, e.Name())
		_, desc := markdownFrontmatter(file)
		id := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		add(model.Capability{ID: id, Kind: "workflow", Description: desc, Scope: scope, Path: file, Enabled: true, Operations: []string{"trajectory"}, Risk: "mixed"})
	}
}

func markdownFrontmatter(path string) (name, desc string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	in := false
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
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

func inferDomains(s string) []string {
	s = strings.ToLower(s)
	set := map[string]bool{}
	keywords := map[string][]string{
		"go": {"go", "gopls", "golang"}, "web": {"web", "browser", "playwright", "frontend", "ui"},
		"git": {"git", "github", "pr", "pull request"}, "docs": {"doc", "documentation", "context7", "api"},
		"editorial": {"editor", "writing", "slop", "proofread", "copyedit"}, "security": {"security", "threat", "vulnerability"},
		"testing": {"test", "verification", "qa"}, "infra": {"terraform", "kubernetes", "cloud", "docker"},
		"observability": {"sentry", "grafana", "telemetry", "observability"}, "database": {"database", "postgres", "supabase", "sql"},
	}
	for domain, ks := range keywords {
		for _, k := range ks {
			if strings.Contains(s, k) {
				set[domain] = true
			}
		}
	}
	var out []string
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func inferMCPRisk(name string) string {
	s := strings.ToLower(name)
	switch {
	case strings.Contains(s, "github") || strings.Contains(s, "linear") || strings.Contains(s, "terraform") || strings.Contains(s, "database") || strings.Contains(s, "supabase"):
		return "write-high"
	case strings.Contains(s, "browser") || strings.Contains(s, "playwright") || strings.Contains(s, "devtools"):
		return "execution"
	default:
		return "read-mixed"
	}
}

func nativeCapabilities() []model.Capability {
	return []model.Capability{
		{ID: "native-goal", Kind: "native", Description: "Antigravity /goal until-complete execution", Enabled: true, Domains: []string{"engineering"}, Operations: []string{"until-done"}, Risk: "agentic", Scope: "platform"},
		{ID: "native-subagents", Kind: "native", Description: "Antigravity invoke_subagent/define_subagent collaboration", Enabled: true, Operations: []string{"delegate", "parallelize"}, Risk: "agentic", Scope: "platform"},
		{ID: "native-schedule", Kind: "native", Description: "Antigravity scheduled tasks", Enabled: true, Operations: []string{"schedule"}, Risk: "agentic", Scope: "platform"},
		{ID: "native-browser", Kind: "native", Description: "Built-in Antigravity browser/search/url tools", Enabled: true, Domains: []string{"web"}, Operations: []string{"search", "browser"}, Risk: "network", Scope: "platform"},
	}
}

func Summary(reg model.CapabilityRegistry) string {
	counts := map[string]int{}
	for _, c := range reg.Capabilities {
		counts[c.Kind]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return strings.Join(parts, " ")
}

// RankedCapability is a capability with a lightweight deterministic relevance score.
// It is intentionally local and explainable; the model still makes the final routing decision.
type RankedCapability struct {
	Capability model.Capability `json:"capability"`
	Score      int              `json:"score"`
	Reasons    []string         `json:"reasons,omitempty"`
}

// Rank returns capabilities most relevant to natural-language task text.
// It rewards exact IDs, domain matches and operation keywords while keeping
// disabled capabilities out of the route.
func Rank(reg model.CapabilityRegistry, task string, limit int) []RankedCapability {
	if limit <= 0 {
		limit = 12
	}
	q := strings.ToLower(strings.TrimSpace(task))
	tokens := tokenize(q)
	var out []RankedCapability
	for _, c := range reg.Capabilities {
		if !c.Enabled {
			continue
		}
		score := 0
		var reasons []string
		id := strings.ToLower(c.ID)
		desc := strings.ToLower(c.Description)
		if id != "" && strings.Contains(q, id) {
			score += 12
			reasons = append(reasons, "capability id mentioned")
		}
		for _, d := range c.Domains {
			dl := strings.ToLower(d)
			if containsTokenOrSubstring(q, tokens, dl) {
				score += 7
				reasons = append(reasons, "domain:"+d)
			}
		}
		for _, op := range c.Operations {
			opl := strings.ToLower(op)
			if containsTokenOrSubstring(q, tokens, opl) {
				score += 4
				reasons = append(reasons, "operation:"+op)
			}
		}
		for token := range tokens {
			if len(token) < 3 {
				continue
			}
			if strings.Contains(id, token) {
				score += 3
			}
			if strings.Contains(desc, token) {
				score += 1
			}
		}
		// Bias specialized roles over generic/native capabilities when equally relevant.
		switch c.Kind {
		case "agent", "skill":
			score++
		}
		if score > 0 {
			out = append(out, RankedCapability{Capability: c, Score: score, Reasons: uniqueStrings(reasons)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Capability.Kind != out[j].Capability.Kind {
			return out[i].Capability.Kind < out[j].Capability.Kind
		}
		return out[i].Capability.ID < out[j].Capability.ID
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func tokenize(s string) map[string]bool {
	f := func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && !(r >= 'а' && r <= 'я') && r != 'ё'
	}
	out := map[string]bool{}
	for _, x := range strings.FieldsFunc(strings.ToLower(s), f) {
		if x != "" {
			out[x] = true
		}
	}
	return out
}

func containsTokenOrSubstring(q string, tokens map[string]bool, needle string) bool {
	if needle == "" {
		return false
	}
	if tokens[needle] {
		return true
	}
	return strings.Contains(q, needle)
}

func uniqueStrings(xs []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if x != "" && !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}
