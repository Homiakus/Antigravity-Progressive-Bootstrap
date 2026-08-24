package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type WorkerConfig struct {
	WorkerID     harnessmodel.WorkerID
	BaseURL      string
	Capabilities []string
	Trust        string
	Heartbeat    time.Duration
}

type Client struct {
	cfg        WorkerConfig
	httpClient *http.Client
}

func NewClient(cfg WorkerConfig, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.Heartbeat <= 0 {
		cfg.Heartbeat = 5 * time.Second
	}
	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
	}
}

type ClaimResponse struct {
	Attempt   harnessmodel.Attempt    `json:"attempt"`
	Lease     harnessmodel.Lease      `json:"lease"`
	NodeSpec  harnessmodel.NodeSpec   `json:"nodeSpec"`
	Available bool                    `json:"available"`
}

func (c *Client) Register(ctx context.Context) error {
	payload, _ := json.Marshal(map[string]any{
		"workerId":     c.cfg.WorkerID,
		"capabilities": c.cfg.Capabilities,
		"trust":        c.cfg.Trust,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/v1/workers/register", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("register worker failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("register worker failed with status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Heartbeat(ctx context.Context) error {
	payload, _ := json.Marshal(map[string]any{
		"workerId": c.cfg.WorkerID,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/v1/workers/heartbeat", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("worker heartbeat failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("worker heartbeat failed with status %d", resp.StatusCode)
	}
	return nil
}
