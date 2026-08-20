package mcpprobe

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/mcp"
	"github.com/homiakus/agctl/internal/model"
	"github.com/homiakus/agctl/internal/paths"
)

const (
	CurrentProtocolVersion           = "2026-07-28"
	FallbackProtocolVersion          = "2025-11-25"
	ClientVersion                    = "3.2.1"
	unsupportedVersionCode           = -32022
	missingClientCapabilityErrorCode = -32021
	headerMismatchErrorCode          = -32020
)

type Report struct {
	Name       string   `json:"name"`
	Transport  string   `json:"transport"`
	OK         bool     `json:"ok"`
	LatencyMS  int64    `json:"latencyMs"`
	Protocol   string   `json:"protocol,omitempty"`
	ServerName string   `json:"serverName,omitempty"`
	Version    string   `json:"version,omitempty"`
	Tools      []string `json:"tools,omitempty"`
	Resources  []string `json:"resources,omitempty"`
	Prompts    []string `json:"prompts,omitempty"`
	Error      string   `json:"error,omitempty"`
	Stderr     string   `json:"stderr,omitempty"`
}

func ProbeConfigured(p paths.Paths, workspace, name string, timeout time.Duration) (Report, error) {
	root, err := jsonx.ReadMap(mcp.ConfigPath(p, workspace))
	if err != nil {
		return Report{}, err
	}
	servers, _ := root["mcpServers"].(map[string]any)
	raw, ok := servers[name]
	if !ok {
		return Report{}, fmt.Errorf("MCP %q not configured", name)
	}
	b, _ := json.Marshal(raw)
	var s model.MCPServer
	if err := json.Unmarshal(b, &s); err != nil {
		return Report{}, err
	}
	return Probe(name, s, timeout)
}

func ProbeAll(p paths.Paths, workspace string, timeout time.Duration) []Report {
	root, err := jsonx.ReadMap(mcp.ConfigPath(p, workspace))
	if err != nil {
		return []Report{{OK: false, Error: err.Error()}}
	}
	servers, _ := root["mcpServers"].(map[string]any)
	reports := make([]Report, 0, len(servers))
	ch := make(chan Report, len(servers))
	var wg sync.WaitGroup
	for name, raw := range servers {
		b, _ := json.Marshal(raw)
		var s model.MCPServer
		if json.Unmarshal(b, &s) != nil || s.Disabled {
			continue
		}
		wg.Add(1)
		go func(n string, sv model.MCPServer) {
			defer wg.Done()
			r, _ := Probe(n, sv, timeout)
			ch <- r
		}(name, s)
	}
	wg.Wait()
	close(ch)
	for r := range ch {
		reports = append(reports, r)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Name < reports[j].Name })
	return reports
}

func Probe(name string, s model.MCPServer, timeout time.Duration) (Report, error) {
	if timeout <= 0 {
		timeout = 12 * time.Second
	}
	start := time.Now()
	var r Report
	var err error
	switch {
	case s.Command != "":
		r, err = probeStdio(name, s, timeout)
	case s.ServerURL != "":
		lowerURL := strings.ToLower(strings.TrimSpace(s.ServerURL))
		if strings.HasPrefix(lowerURL, "ws://") || strings.HasPrefix(lowerURL, "wss://") {
			r = Report{Name: name, Transport: "websocket", Error: "WebSocket MCP is accepted by Antigravity but agctl deep live probe currently supports stdio and HTTP transports only"}
			err = fmt.Errorf("%s", r.Error)
		} else {
			r, err = probeHTTP(name, s, timeout)
		}
	default:
		r = Report{Name: name, Error: "no command/serverUrl"}
		err = fmt.Errorf("%s", r.Error)
	}
	r.LatencyMS = time.Since(start).Milliseconds()
	r.OK = err == nil
	if err != nil && r.Error == "" {
		r.Error = err.Error()
	}
	return r, err
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id"`
	Result  map[string]any `json:"result"`
	Error   map[string]any `json:"error"`
}

type stdioRPC struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
	errBuf *bytes.Buffer
}

func startStdio(s model.MCPServer, timeout time.Duration) (*stdioRPC, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	cmd := exec.CommandContext(ctx, s.Command, s.Args...)
	if s.CWD != "" {
		cmd.Dir = s.CWD
	}
	cmd.Env = os.Environ()
	for k, v := range s.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, nil, err
	}
	errBuf := &bytes.Buffer{}
	cmd.Stderr = errBuf
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, nil, err
	}
	return &stdioRPC{cmd: cmd, stdin: stdin, reader: bufio.NewReader(stdout), errBuf: errBuf}, cancel, nil
}

func (c *stdioRPC) close() {
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	_ = c.cmd.Wait()
}

func (c *stdioRPC) send(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = c.stdin.Write(b)
	return err
}

func (c *stdioRPC) read(id int) (rpcResponse, error) {
	for {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			return rpcResponse{}, err
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var resp rpcResponse
		if json.Unmarshal(line, &resp) != nil {
			continue
		}
		if fmt.Sprint(resp.ID) == fmt.Sprint(id) {
			return resp, nil
		}
	}
}

func modernMeta(version string) map[string]any {
	return map[string]any{
		"io.modelcontextprotocol/protocolVersion":    version,
		"io.modelcontextprotocol/clientInfo":         map[string]any{"name": "agctl", "version": ClientVersion},
		"io.modelcontextprotocol/clientCapabilities": map[string]any{},
	}
}

func modernParams(version string) map[string]any { return map[string]any{"_meta": modernMeta(version)} }

func probeStdio(name string, s model.MCPServer, timeout time.Duration) (Report, error) {
	rep, fallback, err := probeStdioModern(name, s, timeout)
	if err == nil {
		return rep, nil
	}
	if !fallback {
		return rep, err
	}
	return probeStdioLegacy(name, s, timeout)
}

func probeStdioModern(name string, s model.MCPServer, timeout time.Duration) (Report, bool, error) {
	c, cancel, err := startStdio(s, timeout)
	if err != nil {
		return Report{Name: name, Transport: "stdio"}, false, err
	}
	defer cancel()
	defer c.close()

	if err := c.send(rpcRequest{JSONRPC: "2.0", ID: 1, Method: "server/discover", Params: modernParams(CurrentProtocolVersion)}); err != nil {
		return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, false, err
	}
	discover, err := c.read(1)
	if err != nil {
		// A legacy stdio server can reject or ignore pre-initialize requests.
		return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, true, err
	}
	if discover.Error != nil {
		if errorCode(discover.Error) == unsupportedVersionCode {
			if hasLegacyVersion(errorSupportedVersions(discover.Error)) {
				return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, true, fmt.Errorf("modern version unsupported; legacy advertised")
			}
			return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, false, fmt.Errorf("MCP modern version unsupported: %v", discover.Error)
		}
		return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, true, fmt.Errorf("legacy-style response to server/discover: %v", discover.Error)
	}

	if err := validateModernCompleteResult("server/discover", discover.Result); err != nil {
		return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, false, err
	}
	versions := stringSlice(discover.Result["supportedVersions"])
	if len(versions) == 0 {
		return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, false, fmt.Errorf("server/discover missing supportedVersions")
	}
	if !containsString(versions, CurrentProtocolVersion) {
		if hasLegacyVersion(versions) {
			return Report{Name: name, Transport: "stdio"}, true, fmt.Errorf("server advertises only legacy-compatible version")
		}
		return Report{Name: name, Transport: "stdio"}, false, fmt.Errorf("no mutually supported modern MCP version; server=%v", versions)
	}

	rep := modernReport(name, "stdio", discover.Result)
	rep.Stderr = truncate(c.errBuf.String(), 4000)
	probeModernListsStdio(c, &rep, discover.Result)
	return rep, false, nil
}

func probeModernListsStdio(c *stdioRPC, rep *Report, discover map[string]any) {
	capabilities, _ := discover["capabilities"].(map[string]any)
	methods := []struct {
		cap, method string
		id          int
		dest        *[]string
	}{
		{"tools", "tools/list", 2, &rep.Tools},
		{"resources", "resources/list", 3, &rep.Resources},
		{"prompts", "prompts/list", 4, &rep.Prompts},
	}
	for _, x := range methods {
		if capabilities != nil {
			if _, ok := capabilities[x.cap]; !ok {
				continue
			}
		}
		if c.send(rpcRequest{JSONRPC: "2.0", ID: x.id, Method: x.method, Params: modernParams(CurrentProtocolVersion)}) != nil {
			continue
		}
		rr, err := c.read(x.id)
		if err != nil || rr.Error != nil {
			continue
		}
		if validateModernCompleteResult(x.method, rr.Result) != nil {
			continue
		}
		*x.dest = extractNames(rr.Result, x.cap)
	}
}

func probeStdioLegacy(name string, s model.MCPServer, timeout time.Duration) (Report, error) {
	c, cancel, err := startStdio(s, timeout)
	if err != nil {
		return Report{Name: name, Transport: "stdio"}, err
	}
	defer cancel()
	defer c.close()

	initReq := rpcRequest{JSONRPC: "2.0", ID: 1, Method: "initialize", Params: map[string]any{"protocolVersion": FallbackProtocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "agctl", "version": ClientVersion}}}
	if err := c.send(initReq); err != nil {
		return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, err
	}
	initResp, err := c.read(1)
	if err != nil {
		return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, err
	}
	if initResp.Error != nil {
		return Report{Name: name, Transport: "stdio", Stderr: truncate(c.errBuf.String(), 4000)}, fmt.Errorf("MCP initialize error: %v", initResp.Error)
	}
	_ = c.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"})
	rep := legacyReport(name, "stdio", initResp.Result)
	rep.Stderr = truncate(c.errBuf.String(), 4000)
	probeLegacyListsStdio(c, &rep)
	return rep, nil
}

func probeLegacyListsStdio(c *stdioRPC, rep *Report) {
	for idx, method := range []string{"tools/list", "resources/list", "prompts/list"} {
		id := idx + 2
		if c.send(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: map[string]any{}}) != nil {
			continue
		}
		rr, err := c.read(id)
		if err != nil || rr.Error != nil {
			continue
		}
		setList(rep, method, rr.Result)
	}
}

func probeHTTP(name string, s model.MCPServer, timeout time.Duration) (Report, error) {
	rep, fallback, err := probeHTTPModern(name, s, timeout)
	if err == nil {
		return rep, nil
	}
	if !fallback {
		return rep, err
	}
	return probeHTTPLegacy(name, s, timeout)
}

func probeHTTPModern(name string, s model.MCPServer, timeout time.Duration) (Report, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}
	post := func(id int, method string) (rpcResponse, http.Header, int, error) {
		return postHTTPRPC(ctx, client, s, id, method, modernParams(CurrentProtocolVersion), CurrentProtocolVersion, "")
	}
	discover, _, status, err := post(1, "server/discover")
	if err != nil && discover.Error == nil {
		// Unknown method / non-modern HTTP endpoints commonly return 4xx or non-JSON.
		if status >= 400 && status < 500 {
			return Report{Name: name, Transport: "http"}, true, err
		}
		return Report{Name: name, Transport: "http"}, false, err
	}
	if discover.Error != nil {
		code := errorCode(discover.Error)
		if code == unsupportedVersionCode {
			if hasLegacyVersion(errorSupportedVersions(discover.Error)) {
				return Report{Name: name, Transport: "http"}, true, fmt.Errorf("modern version unsupported; legacy advertised")
			}
			return Report{Name: name, Transport: "http"}, false, fmt.Errorf("MCP modern version unsupported: %v", discover.Error)
		}
		// Over HTTP, a JSON-RPC error body for a modern request is evidence that
		// the endpoint speaks modern MCP. Do not incorrectly fall back to the
		// legacy initialize era for recognized modern errors.
		if code == -32601 || code == missingClientCapabilityErrorCode || code == headerMismatchErrorCode {
			return Report{Name: name, Transport: "http"}, false, fmt.Errorf("modern MCP server/discover error: %v", discover.Error)
		}
		return Report{Name: name, Transport: "http"}, false, fmt.Errorf("MCP server/discover error: %v", discover.Error)
	}
	if err := validateModernCompleteResult("server/discover", discover.Result); err != nil {
		return Report{Name: name, Transport: "http"}, false, err
	}
	versions := stringSlice(discover.Result["supportedVersions"])
	if len(versions) == 0 {
		return Report{Name: name, Transport: "http"}, false, fmt.Errorf("server/discover missing supportedVersions")
	}
	if !containsString(versions, CurrentProtocolVersion) {
		if hasLegacyVersion(versions) {
			return Report{Name: name, Transport: "http"}, true, fmt.Errorf("server advertises only legacy-compatible version")
		}
		return Report{Name: name, Transport: "http"}, false, fmt.Errorf("no mutually supported modern MCP version; server=%v", versions)
	}

	rep := modernReport(name, "http", discover.Result)
	capabilities, _ := discover.Result["capabilities"].(map[string]any)
	for idx, x := range []struct{ cap, method string }{{"tools", "tools/list"}, {"resources", "resources/list"}, {"prompts", "prompts/list"}} {
		if capabilities != nil {
			if _, ok := capabilities[x.cap]; !ok {
				continue
			}
		}
		rr, _, _, e := post(idx+2, x.method)
		if e != nil || rr.Error != nil {
			continue
		}
		if validateModernCompleteResult(x.method, rr.Result) != nil {
			continue
		}
		setList(&rep, x.method, rr.Result)
	}
	return rep, false, nil
}

func probeHTTPLegacy(name string, s model.MCPServer, timeout time.Duration) (Report, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}
	session := ""
	protocol := ""
	post := func(id int, method string, params any) (rpcResponse, http.Header, int, error) {
		return postHTTPRPC(ctx, client, s, id, method, params, protocol, session)
	}
	init, headers, _, err := post(1, "initialize", map[string]any{"protocolVersion": FallbackProtocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "agctl", "version": ClientVersion}})
	if err != nil {
		return Report{Name: name, Transport: "http"}, err
	}
	if init.Error != nil {
		return Report{Name: name, Transport: "http"}, fmt.Errorf("MCP initialize error: %v", init.Error)
	}
	protocol = str(init.Result["protocolVersion"])
	if protocol == "" {
		protocol = FallbackProtocolVersion
	}
	session = headers.Get("Mcp-Session-Id")
	// Legacy HTTP servers complete initialization with a notification. The
	// notification itself has no JSON-RPC response, so use the dedicated sender.
	if err := postHTTPNotification(ctx, client, s, "notifications/initialized", map[string]any{}, protocol, session); err != nil {
		return Report{Name: name, Transport: "http"}, fmt.Errorf("MCP initialized notification: %w", err)
	}
	rep := legacyReport(name, "http", init.Result)
	for idx, method := range []string{"tools/list", "resources/list", "prompts/list"} {
		rr, _, _, e := post(idx+2, method, map[string]any{})
		if e != nil || rr.Error != nil {
			continue
		}
		setList(&rep, method, rr.Result)
	}
	return rep, nil
}

func postHTTPNotification(ctx context.Context, client *http.Client, s model.MCPServer, method string, params any, protocolVersion, session string) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.ServerURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if protocolVersion != "" {
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
	}
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %s", resp.Status)
	}
	return nil
}

func postHTTPRPC(ctx context.Context, client *http.Client, s model.MCPServer, id int, method string, params any, protocolVersion, session string) (rpcResponse, http.Header, int, error) {
	reqBody, err := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	if err != nil {
		return rpcResponse{}, nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.ServerURL, bytes.NewReader(reqBody))
	if err != nil {
		return rpcResponse{}, nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if protocolVersion != "" {
		req.Header.Set("MCP-Protocol-Version", protocolVersion)
		req.Header.Set("Mcp-Method", method)
	}
	if session != "" {
		req.Header.Set("Mcp-Session-Id", session)
	}
	for k, v := range s.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return rpcResponse{}, nil, 0, err
	}
	defer resp.Body.Close()
	var rr rpcResponse
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "text/event-stream") {
		rr, err = readSSEForID(resp.Body, id)
	} else {
		err = json.NewDecoder(resp.Body).Decode(&rr)
	}
	if err != nil {
		return rr, resp.Header, resp.StatusCode, fmt.Errorf("HTTP %s: %w", resp.Status, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return rr, resp.Header, resp.StatusCode, fmt.Errorf("HTTP %s", resp.Status)
	}
	return rr, resp.Header, resp.StatusCode, nil
}

func validateModernCompleteResult(method string, result map[string]any) error {
	if result == nil {
		return fmt.Errorf("%s returned no result", method)
	}
	if got := str(result["resultType"]); got != "complete" {
		if got == "" {
			return fmt.Errorf("%s response is not 2026-07-28 compliant: missing required resultType", method)
		}
		return fmt.Errorf("%s returned unsupported resultType %q during health probe", method, got)
	}
	if _, ok := result["ttlMs"]; !ok {
		return fmt.Errorf("%s response is not 2026-07-28 compliant: missing required ttlMs", method)
	}
	scope := str(result["cacheScope"])
	if scope != "public" && scope != "private" {
		return fmt.Errorf("%s response is not 2026-07-28 compliant: invalid/missing cacheScope", method)
	}
	if method == "server/discover" {
		if _, ok := result["capabilities"].(map[string]any); !ok {
			return fmt.Errorf("server/discover missing capabilities")
		}
	}
	return nil
}

func modernReport(name, transport string, result map[string]any) Report {
	rep := Report{Name: name, Transport: transport, Protocol: CurrentProtocolVersion}
	if meta, ok := result["_meta"].(map[string]any); ok {
		if si, ok := meta["io.modelcontextprotocol/serverInfo"].(map[string]any); ok {
			rep.ServerName = str(si["name"])
			rep.Version = str(si["version"])
		}
	}
	return rep
}

func legacyReport(name, transport string, result map[string]any) Report {
	rep := Report{Name: name, Transport: transport, Protocol: str(result["protocolVersion"])}
	if rep.Protocol == "" {
		rep.Protocol = FallbackProtocolVersion
	}
	if si, ok := result["serverInfo"].(map[string]any); ok {
		rep.ServerName = str(si["name"])
		rep.Version = str(si["version"])
	}
	return rep
}

func setList(rep *Report, method string, result map[string]any) {
	key := strings.Split(method, "/")[0]
	names := extractNames(result, key)
	switch method {
	case "tools/list":
		rep.Tools = names
	case "resources/list":
		rep.Resources = names
	case "prompts/list":
		rep.Prompts = names
	}
}

func errorCode(errObj map[string]any) int {
	switch v := errObj["code"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}

func errorSupportedVersions(errObj map[string]any) []string {
	data, _ := errObj["data"].(map[string]any)
	return stringSlice(data["supported"])
}

func stringSlice(v any) []string {
	var out []string
	switch xs := v.(type) {
	case []any:
		for _, x := range xs {
			if s := str(x); s != "" {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, xs...)
	}
	return out
}

func hasLegacyVersion(xs []string) bool {
	for _, x := range xs {
		if x != "" && x != CurrentProtocolVersion {
			return true
		}
	}
	return false
}

func containsString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func readSSEForID(r io.Reader, id int) (rpcResponse, error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var rr rpcResponse
		if json.Unmarshal([]byte(data), &rr) == nil && fmt.Sprint(rr.ID) == fmt.Sprint(id) {
			return rr, nil
		}
	}
	if err := s.Err(); err != nil {
		return rpcResponse{}, err
	}
	return rpcResponse{}, fmt.Errorf("SSE ended without response id %d", id)
}

func extractNames(result map[string]any, key string) []string {
	xs, _ := result[key].([]any)
	var out []string
	for _, x := range xs {
		if m, ok := x.(map[string]any); ok {
			n := str(m["name"])
			if n == "" {
				n = str(m["uri"])
			}
			if n != "" {
				out = append(out, n)
			}
		}
	}
	return out
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
