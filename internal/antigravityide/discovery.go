package antigravityide

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Registration struct {
	ProtocolVersion int      `json:"protocolVersion"`
	InstanceID      string   `json:"instanceId"`
	BootNonce       string   `json:"bootNonce"`
	PID             int      `json:"pid"`
	Port            int      `json:"port"`
	WorkspaceFolders []string `json:"workspaceFolders"`
	StartedAt       time.Time `json:"startedAt"`
}

type LocatedBridge struct {
	Registration Registration
	Client       Client
}

type Locator interface {
	Wait(context.Context, string, string) (LocatedBridge, error)
}

type Discovery struct {
	Root         string
	PollInterval time.Duration
	HTTPClient   *http.Client
}

func (d Discovery) Wait(ctx context.Context, instanceID, token string) (LocatedBridge, error) {
	if strings.TrimSpace(d.Root) == "" || strings.TrimSpace(instanceID) == "" || strings.TrimSpace(token) == "" {
		return LocatedBridge{}, fmt.Errorf("bridge discovery requires root, instance id and token")
	}
	interval := d.PollInterval
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	for {
		located, found := d.findHealthy(ctx, instanceID, token)
		if found {
			return located, nil
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return LocatedBridge{}, fmt.Errorf("wait for Antigravity Bridge instance %s: %w", instanceID, ctx.Err())
		case <-timer.C:
		}
	}
}

func (d Discovery) findHealthy(ctx context.Context, instanceID, token string) (LocatedBridge, bool) {
	entries, err := os.ReadDir(d.Root)
	if err != nil {
		return LocatedBridge{}, false
	}
	type candidate struct {
		registration Registration
		modTime      time.Time
	}
	candidates := make([]candidate, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		registration, err := readRegistration(filepath.Join(d.Root, entry.Name()))
		if err != nil || registration.InstanceID != instanceID {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{registration: registration, modTime: info.ModTime()})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].modTime.After(candidates[j].modTime) })
	for _, candidate := range candidates {
		base := (&url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", candidate.registration.Port)}).String()
		client, err := NewHTTPClient(base, token, d.HTTPClient)
		if err != nil {
			continue
		}
		healthCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
		health, err := client.Health(healthCtx)
		cancel()
		if err != nil || health.InstanceID != instanceID || health.BootNonce != candidate.registration.BootNonce {
			continue
		}
		return LocatedBridge{Registration: candidate.registration, Client: client}, true
	}
	return LocatedBridge{}, false
}

func readRegistration(file string) (Registration, error) {
	payload, err := os.ReadFile(file)
	if err != nil {
		return Registration{}, err
	}
	if len(payload) > 64<<10 {
		return Registration{}, fmt.Errorf("bridge registration too large")
	}
	var registration Registration
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registration); err != nil {
		return Registration{}, err
	}
	if registration.ProtocolVersion != ProtocolVersion || strings.TrimSpace(registration.InstanceID) == "" || strings.TrimSpace(registration.BootNonce) == "" || registration.PID <= 0 || registration.Port <= 0 || registration.Port > 65535 || registration.StartedAt.IsZero() {
		return Registration{}, fmt.Errorf("invalid bridge registration")
	}
	return registration, nil
}
