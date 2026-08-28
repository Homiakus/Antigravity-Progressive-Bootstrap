package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type result struct {
	Endpoint string          `json:"endpoint"`
	OK       bool            `json:"ok"`
	Status   int             `json:"status,omitempty"`
	Body     json.RawMessage `json:"body,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func main() {
	baseURL := flag.String("url", "http://127.0.0.1:51234", "Antigravity Bridge base URL")
	token := flag.String("token", os.Getenv("AGCTL_BRIDGE_TOKEN"), "bridge bearer token (or AGCTL_BRIDGE_TOKEN)")
	timeout := flag.Duration("timeout", 5*time.Second, "per-request timeout")
	flag.Parse()

	endpoints := []string{"/v1/health", "/v1/capabilities", "/v1/context", "/v1/conversations"}
	client := &http.Client{Timeout: *timeout}
	ctx := context.Background()
	output := make([]result, 0, len(endpoints))
	for _, endpoint := range endpoints {
		output = append(output, probe(ctx, client, strings.TrimRight(*baseURL, "/")+endpoint, endpoint, *token))
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, item := range output {
		if !item.OK {
			os.Exit(2)
		}
	}
}

func probe(ctx context.Context, client *http.Client, url, endpoint, token string) result {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return result{Endpoint: endpoint, Error: err.Error()}
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return result{Endpoint: endpoint, Error: err.Error()}
	}
	defer resp.Body.Close()
	var body json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return result{Endpoint: endpoint, Status: resp.StatusCode, Error: "decode response: " + err.Error()}
	}
	return result{Endpoint: endpoint, OK: resp.StatusCode >= 200 && resp.StatusCode < 300, Status: resp.StatusCode, Body: body}
}
