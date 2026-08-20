package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

type CatalogItem struct {
	ID          string
	Label       string
	Description string
	Recommended bool
	Build       func() model.MCPServer
	Requires    []string
}

func nodeMCP(pkg string, extra ...string) model.MCPServer {
	args := []string{"-y", pkg}
	args = append(args, extra...)
	if runtime.GOOS == "windows" {
		return model.MCPServer{Command: "cmd", Args: append([]string{"/c", "npx"}, args...)}
	}
	return model.MCPServer{Command: "npx", Args: args}
}

func Catalog() []CatalogItem {
	return []CatalogItem{
		{ID: "context7", Label: "Context7", Description: "Current/versioned library documentation", Recommended: true, Requires: []string{"npx"}, Build: func() model.MCPServer { return nodeMCP("@upstash/context7-mcp@latest") }},
		{ID: "playwright", Label: "Playwright", Description: "Browser automation and E2E flows", Recommended: true, Requires: []string{"npx"}, Build: func() model.MCPServer { return nodeMCP("@playwright/mcp@latest") }},
		{ID: "chrome-devtools", Label: "Chrome DevTools", Description: "Browser console/network/runtime/performance diagnosis", Recommended: true, Requires: []string{"npx"}, Build: func() model.MCPServer { return nodeMCP("chrome-devtools-mcp@latest") }},
		{ID: "memory", Label: "Memory", Description: "Experimental reference Memory server; not part of the stable default", Recommended: false, Requires: []string{"npx"}, Build: func() model.MCPServer { return nodeMCP("@modelcontextprotocol/server-memory") }},
		{ID: "gopls", Label: "gopls MCP", Description: "Official but experimental Go semantic diagnostics/refactoring MCP; install explicitly for Go projects", Recommended: false, Requires: []string{"gopls"}, Build: func() model.MCPServer { return model.MCPServer{Command: "gopls", Args: []string{"mcp"}} }},
		{ID: "github", Label: "GitHub MCP", Description: "Remote GitHub repositories/issues/PRs/actions", Recommended: false, Requires: []string{"docker"}, Build: githubServer},
	}
}

func githubServer() model.MCPServer {
	// Do not serialize the secret into mcp_config.json. docker receives the value from its process environment.
	return model.MCPServer{
		Command: "docker",
		Args:    []string{"run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "ghcr.io/github/github-mcp-server"},
	}
}

func ConfigPath(p paths.Paths, workspace string) string {
	if strings.TrimSpace(workspace) == "" {
		return p.GlobalMCP
	}
	return paths.WorkspaceMCP(workspace)
}

func loadRaw(p paths.Paths, workspace string) (map[string]any, map[string]any, error) {
	root, err := jsonx.ReadMap(ConfigPath(p, workspace))
	if err != nil {
		return nil, nil, err
	}
	servers, _ := root["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
		root["mcpServers"] = servers
	}
	return root, servers, nil
}

func InstallServer(p paths.Paths, workspace, name string, server model.MCPServer) error {
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid MCP name %q", name)
	}
	root, servers, err := loadRaw(p, workspace)
	if err != nil {
		return err
	}
	b, err := json.Marshal(server)
	if err != nil {
		return err
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	servers[name] = raw
	root["mcpServers"] = servers
	return jsonx.WriteAtomic(ConfigPath(p, workspace), root, p.BackupsRoot)
}

func Install(p paths.Paths, workspace string, ids ...string) error {
	root, servers, err := loadRaw(p, workspace)
	if err != nil {
		return err
	}
	byID := map[string]CatalogItem{}
	for _, c := range Catalog() {
		byID[c.ID] = c
	}
	for _, id := range ids {
		item, ok := byID[id]
		if !ok {
			return fmt.Errorf("unknown MCP %q", id)
		}
		if id == "github" && strings.TrimSpace(os.Getenv("GITHUB_PERSONAL_ACCESS_TOKEN")) == "" {
			return fmt.Errorf("GitHub MCP requires GITHUB_PERSONAL_ACCESS_TOKEN in the environment; authenticate/export it first")
		}
		b, _ := json.Marshal(item.Build())
		var raw map[string]any
		_ = json.Unmarshal(b, &raw)
		servers[id] = raw
	}
	root["mcpServers"] = servers
	return jsonx.WriteAtomic(ConfigPath(p, workspace), root, p.BackupsRoot)
}

func InstallRecommended(p paths.Paths, workspace string, includeMemory bool) []error {
	var errs []error
	for _, item := range Catalog() {
		wanted := item.Recommended || (includeMemory && item.ID == "memory")
		if !wanted {
			continue
		}
		missing := false
		for _, cmd := range item.Requires {
			if _, err := exec.LookPath(cmd); err != nil {
				missing = true
			}
		}
		if missing {
			continue
		}
		if err := Install(p, workspace, item.ID); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func Remove(p paths.Paths, workspace string, ids ...string) error {
	root, servers, err := loadRaw(p, workspace)
	if err != nil {
		return err
	}
	for _, id := range ids {
		delete(servers, id)
	}
	root["mcpServers"] = servers
	return jsonx.WriteAtomic(ConfigPath(p, workspace), root, p.BackupsRoot)
}

func Names(p paths.Paths, workspace string) ([]string, error) {
	_, servers, err := loadRaw(p, workspace)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(servers))
	for k := range servers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func Doctor(p paths.Paths, workspace string) []string {
	_, servers, err := loadRaw(p, workspace)
	if err != nil {
		return []string{"ERROR: " + err.Error()}
	}
	var out []string
	for name, raw := range servers {
		m, _ := raw.(map[string]any)
		disabled, _ := m["disabled"].(bool)
		command, _ := m["command"].(string)
		serverURL, _ := m["serverUrl"].(string)
		_, hasLegacyURL := m["url"]
		_, hasLegacyHTTPURL := m["httpUrl"]
		status := "OK"
		if hasLegacyURL || hasLegacyHTTPURL {
			status = "INVALID: legacy url/httpUrl unsupported; use serverUrl"
		} else if disabled {
			status = "DISABLED"
		} else if command != "" {
			cmd := command
			if runtime.GOOS == "windows" && strings.EqualFold(command, "cmd") {
				if args, ok := m["args"].([]any); ok && len(args) >= 2 {
					cmd = fmt.Sprint(args[1])
				}
			}
			if _, err := exec.LookPath(cmd); err != nil {
				status = "MISSING COMMAND: " + cmd
			}
		} else if serverURL == "" {
			status = "INVALID: no command/serverUrl"
		}
		out = append(out, fmt.Sprintf("%-20s %s", name, status))
	}
	sort.Strings(out)
	return out
}
