package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

const DefaultBaseURL = "https://registry.modelcontextprotocol.io"

type Client struct {
	BaseURL   string
	HTTP      *http.Client
	CacheRoot string
}

type Server struct {
	Name        string         `json:"name"`
	Version     string         `json:"version,omitempty"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status,omitempty"`
	Raw         map[string]any `json:"raw,omitempty"`
}

type Page struct {
	Servers    []Server `json:"servers"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

func New(p paths.Paths) *Client {
	return &Client{BaseURL: DefaultBaseURL, HTTP: &http.Client{Timeout: 20 * time.Second}, CacheRoot: p.RegistryCacheRoot}
}

func (c *Client) Search(ctx context.Context, q string, limit int) (Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	u := strings.TrimRight(c.BaseURL, "/") + "/v0.1/servers"
	v := url.Values{}
	v.Set("limit", fmt.Sprint(limit))
	v.Set("version", "latest")
	if strings.TrimSpace(q) != "" {
		v.Set("search", q)
	}
	return c.fetchPage(ctx, u+"?"+v.Encode())
}
func (c *Client) Next(ctx context.Context, q, cursor string, limit int) (Page, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	u := strings.TrimRight(c.BaseURL, "/") + "/v0.1/servers"
	v := url.Values{}
	v.Set("limit", fmt.Sprint(limit))
	v.Set("version", "latest")
	if q != "" {
		v.Set("search", q)
	}
	if cursor != "" {
		v.Set("cursor", cursor)
	}
	return c.fetchPage(ctx, u+"?"+v.Encode())
}
func (c *Client) Detail(ctx context.Context, name, version string) (Server, error) {
	if version == "" {
		version = "latest"
	}
	u := strings.TrimRight(c.BaseURL, "/") + "/v0.1/servers/" + url.PathEscape(name) + "/versions/" + url.PathEscape(version)
	var raw map[string]any
	if err := c.getJSON(ctx, u, &raw); err != nil {
		return Server{}, err
	}
	s := normalizeServer(raw)
	_ = c.cache("detail-"+sanitize(name)+"-"+sanitize(version)+".json", raw)
	return s, nil
}

func (c *Client) fetchPage(ctx context.Context, u string) (Page, error) {
	var raw map[string]any
	if err := c.getJSON(ctx, u, &raw); err != nil {
		return Page{}, err
	}
	var out Page
	if xs, ok := raw["servers"].([]any); ok {
		for _, x := range xs {
			if m, ok := x.(map[string]any); ok {
				out.Servers = append(out.Servers, normalizeServer(m))
			}
		}
	}
	if meta, ok := raw["metadata"].(map[string]any); ok {
		out.NextCursor = str(meta["nextCursor"])
	}
	sort.Slice(out.Servers, func(i, j int) bool { return out.Servers[i].Name < out.Servers[j].Name })
	_ = c.cache("search-latest.json", raw)
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "agctl/3.2.1")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("registry HTTP %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func normalizeServer(raw map[string]any) Server {
	body := raw
	if s, ok := raw["server"].(map[string]any); ok {
		body = s
	}
	name := str(body["name"])
	ver := str(body["version"])
	desc := str(body["description"])
	status := ""
	if meta, ok := raw["_meta"].(map[string]any); ok {
		for k, v := range meta {
			if strings.Contains(k, "registry/official") {
				if m, ok := v.(map[string]any); ok {
					status = str(m["status"])
				}
			}
		}
	}
	if status == "" {
		status = str(raw["status"])
	}
	return Server{Name: name, Version: ver, Description: desc, Status: status, Raw: raw}
}

type InstallPlan struct {
	Server      model.MCPServer `json:"server"`
	Source      string          `json:"source"`
	Warnings    []string        `json:"warnings,omitempty"`
	Environment []string        `json:"environment,omitempty"`
}

// Plan converts official Registry metadata into an Antigravity MCP config when
// the metadata exposes a direct remote endpoint or a conventional npm stdio
// package. Complex package launchers remain inspect-only rather than guessed.
func Plan(s Server) (InstallPlan, error) {
	body := s.Raw
	if x, ok := body["server"].(map[string]any); ok {
		body = x
	}
	if rem, ok := body["remotes"].([]any); ok {
		for _, x := range rem {
			m, _ := x.(map[string]any)
			u := str(m["url"])
			if u == "" {
				continue
			}
			return InstallPlan{Server: model.MCPServer{ServerURL: u}, Source: "registry remote"}, nil
		}
	}
	if pkgs, ok := body["packages"].([]any); ok {
		for _, x := range pkgs {
			m, _ := x.(map[string]any)
			typ := strings.ToLower(str(m["registryType"]))
			id := str(m["identifier"])
			ver := str(m["version"])
			if typ != "npm" || id == "" {
				continue
			}
			pkg := id
			if ver != "" && ver != "latest" {
				pkg += "@" + ver
			} else {
				pkg += "@latest"
			}
			args := []string{"-y", pkg}
			server := model.MCPServer{Command: "npx", Args: args}
			if runtime.GOOS == "windows" {
				server = model.MCPServer{Command: "cmd", Args: append([]string{"/c", "npx"}, args...)}
			}
			plan := InstallPlan{Server: server, Source: "registry npm package"}
			for _, key := range []string{"environmentVariables", "environment_variables"} {
				if envs, ok := m[key].([]any); ok {
					for _, ev := range envs {
						if em, ok := ev.(map[string]any); ok {
							n := str(em["name"])
							if n != "" {
								plan.Environment = append(plan.Environment, n)
							}
						}
					}
				}
			}
			if len(plan.Environment) > 0 {
				plan.Warnings = append(plan.Warnings, "server requires environment variables; agctl does not serialize secret values into mcp_config.json")
			}
			return plan, nil
		}
	}
	return InstallPlan{}, fmt.Errorf("registry metadata has no directly supported remote or npm stdio installation")
}

func InstallHints(s Server) []string {
	var out []string
	body := s.Raw
	if x, ok := body["server"].(map[string]any); ok {
		body = x
	}
	if pkgs, ok := body["packages"].([]any); ok {
		for _, x := range pkgs {
			if m, ok := x.(map[string]any); ok {
				out = append(out, fmt.Sprintf("package registry=%s identifier=%s version=%s", str(m["registryType"]), str(m["identifier"]), str(m["version"])))
			}
		}
	}
	if rem, ok := body["remotes"].([]any); ok {
		for _, x := range rem {
			if m, ok := x.(map[string]any); ok {
				out = append(out, fmt.Sprintf("remote transport=%s url=%s", str(m["type"]), str(m["url"])))
			}
		}
	}
	return out
}

func (c *Client) cache(name string, v any) error {
	if c.CacheRoot == "" {
		return nil
	}
	_ = os.MkdirAll(c.CacheRoot, 0o755)
	return jsonx.WriteAtomic(filepath.Join(c.CacheRoot, name), v, "")
}
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func sanitize(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "?", "_", "&", "_")
	return r.Replace(s)
}
