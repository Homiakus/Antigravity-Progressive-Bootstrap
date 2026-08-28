package antigravityide

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

type HTTPClient struct {
	base  *url.URL
	token string
	http  *http.Client
}

type envelope struct {
	ProtocolVersion int             `json:"protocolVersion"`
	OK              bool            `json:"ok"`
	Data            json.RawMessage `json:"data"`
	Error           *bridgeError    `json:"error,omitempty"`
}

type bridgeError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func NewHTTPClient(baseURL, token string, client *http.Client) (*HTTPClient, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse Antigravity Bridge URL: %w", err)
	}
	if parsed.Scheme != "http" {
		return nil, fmt.Errorf("Antigravity Bridge URL must use http over loopback")
	}
	if !isLoopbackHost(parsed.Hostname()) {
		return nil, fmt.Errorf("Antigravity Bridge must be loopback-only; got host %q", parsed.Hostname())
	}
	if parsed.Port() == "" {
		return nil, fmt.Errorf("Antigravity Bridge URL requires explicit port")
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("Antigravity Bridge bearer token is required")
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &HTTPClient{base: parsed, token: token, http: client}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *HTTPClient) Health(ctx context.Context) (Health, error) {
	var out Health
	if err := c.request(ctx, http.MethodGet, "/v1/health", nil, &out); err != nil {
		return Health{}, err
	}
	if out.Status != "ok" {
		return Health{}, fmt.Errorf("Antigravity Bridge health status %q", out.Status)
	}
	return out, nil
}

func (c *HTTPClient) Capabilities(ctx context.Context) (Capabilities, error) {
	var out Capabilities
	if err := c.request(ctx, http.MethodGet, "/v1/capabilities", nil, &out); err != nil {
		return Capabilities{}, err
	}
	if out.ProtocolVersion != ProtocolVersion {
		return Capabilities{}, fmt.Errorf("bridge capability protocol=%d want=%d", out.ProtocolVersion, ProtocolVersion)
	}
	return out, nil
}

func (c *HTTPClient) Context(ctx context.Context) (Context, error) {
	var out Context
	if err := c.request(ctx, http.MethodGet, "/v1/context", nil, &out); err != nil {
		return Context{}, err
	}
	return out, nil
}

func (c *HTTPClient) ListConversations(ctx context.Context) ([]Conversation, error) {
	var out []Conversation
	if err := c.request(ctx, http.MethodGet, "/v1/conversations", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *HTTPClient) CreateConversation(ctx context.Context) (Conversation, error) {
	var out Conversation
	if err := c.request(ctx, http.MethodPost, "/v1/conversations", struct{}{}, &out); err != nil {
		return Conversation{}, err
	}
	if strings.TrimSpace(out.ID) == "" {
		return Conversation{}, fmt.Errorf("bridge returned conversation with empty id")
	}
	return out, nil
}

func (c *HTTPClient) FocusConversation(ctx context.Context, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("conversation id is required")
	}
	return c.request(ctx, http.MethodPost, "/v1/conversations/"+url.PathEscape(id)+"/focus", struct{}{}, nil)
}

func (c *HTTPClient) SendMessage(ctx context.Context, id, text string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(text) == "" {
		return fmt.Errorf("conversation id and message text are required")
	}
	return c.request(ctx, http.MethodPost, "/v1/conversations/"+url.PathEscape(id)+"/messages", map[string]string{"text": text}, nil)
}

func (c *HTTPClient) Events(ctx context.Context, id string, after uint64) ([]BridgeEvent, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	var out []BridgeEvent
	endpoint := "/v1/conversations/" + url.PathEscape(id) + "/events?after=" + strconv.FormatUint(after, 10)
	if err := c.request(ctx, http.MethodGet, endpoint, nil, &out); err != nil {
		return nil, err
	}
	for i, event := range out {
		if event.Seq == 0 || strings.TrimSpace(event.Type) == "" || strings.TrimSpace(event.SourceEventID) == "" || strings.TrimSpace(event.StreamKey) == "" || event.Timestamp.IsZero() || len(event.Payload) == 0 {
			return nil, fmt.Errorf("bridge returned invalid event at index %d", i)
		}
	}
	return out, nil
}

func (c *HTTPClient) OpenWorkspace(ctx context.Context, workspacePath string) (OpenWorkspaceResult, error) {
	if strings.TrimSpace(workspacePath) == "" {
		return OpenWorkspaceResult{}, fmt.Errorf("workspace path is required")
	}
	var out OpenWorkspaceResult
	if err := c.request(ctx, http.MethodPost, "/v1/workspace/open", map[string]string{"path": workspacePath}, &out); err != nil {
		return OpenWorkspaceResult{}, err
	}
	return out, nil
}

func (c *HTTPClient) request(ctx context.Context, method, endpoint string, body any, out any) error {
	ref, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("parse bridge endpoint: %w", err)
	}
	if ref.IsAbs() || ref.Host != "" {
		return fmt.Errorf("bridge endpoint must be relative")
	}
	u := *c.base
	u.Path = path.Join(c.base.Path, ref.Path)
	u.RawQuery = ref.RawQuery
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode bridge request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), reader)
	if err != nil {
		return fmt.Errorf("create bridge request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("Antigravity Bridge %s %s: %w", method, ref.Path, err)
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 4<<20)
	var response envelope
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil {
		return fmt.Errorf("decode Antigravity Bridge response: %w", err)
	}
	if response.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported Antigravity Bridge protocol %d; want %d", response.ProtocolVersion, ProtocolVersion)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !response.OK {
		if response.Error != nil {
			return fmt.Errorf("Antigravity Bridge %s: %s", response.Error.Code, response.Error.Message)
		}
		return fmt.Errorf("Antigravity Bridge returned HTTP %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if len(response.Data) == 0 || bytes.Equal(response.Data, []byte("null")) {
		return fmt.Errorf("Antigravity Bridge returned empty data")
	}
	decoder = json.NewDecoder(bytes.NewReader(response.Data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("decode Antigravity Bridge data: %w", err)
	}
	return nil
}
