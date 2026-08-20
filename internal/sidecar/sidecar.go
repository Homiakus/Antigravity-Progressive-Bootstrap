package sidecar

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/jsonx"
	"github.com/homiakus/agctl/internal/paths"
)

type Config struct {
	Command       string            `json:"command,omitempty"`
	Builtin       string            `json:"builtin,omitempty"`
	Args          []string          `json:"args,omitempty"`
	RestartPolicy string            `json:"restart_policy,omitempty"`
	Description   string            `json:"description,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	DisplayName   string            `json:"display_name,omitempty"`
}

type UserConfig struct {
	Enabled   bool   `json:"enabled"`
	ProjectID string `json:"projectId,omitempty"`
}

type Item struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	Scope         string `json:"scope"`
	Enabled       bool   `json:"enabled"`
	ProjectID     string `json:"projectId,omitempty"`
	Command       string `json:"command,omitempty"`
	Builtin       string `json:"builtin,omitempty"`
	RestartPolicy string `json:"restartPolicy,omitempty"`
	Description   string `json:"description,omitempty"`
	Valid         bool   `json:"valid"`
	Issue         string `json:"issue,omitempty"`
}

func List(p paths.Paths) ([]Item, error) {
	enabled, err := loadUserConfig(p)
	if err != nil {
		return nil, err
	}
	var out []Item
	out = append(out, listRoot(p.SidecarsRoot, "global", "", enabled)...)
	plugins, _ := os.ReadDir(p.GlobalPluginsRoot)
	for _, pl := range plugins {
		if !pl.IsDir() {
			continue
		}
		root := filepath.Join(p.GlobalPluginsRoot, pl.Name(), "sidecars")
		out = append(out, listRoot(root, "plugin", pl.Name(), enabled)...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func listRoot(root, scope, pluginName string, enabled map[string]UserConfig) []Item {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var out []Item
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if pluginName != "" {
			id = pluginName + "/" + id
		}
		dir := filepath.Join(root, e.Name())
		cfgPath := filepath.Join(dir, "sidecar.json")
		cfg, err := jsonx.Read(cfgPath, Config{})
		item := Item{ID: id, Path: dir, Scope: scope, Valid: true, Command: cfg.Command, Builtin: cfg.Builtin, RestartPolicy: cfg.RestartPolicy, Description: cfg.Description}
		if uc, ok := enabled[id]; ok {
			item.Enabled = uc.Enabled
			item.ProjectID = uc.ProjectID
		}
		if err != nil {
			item.Valid = false
			item.Issue = "invalid sidecar.json: " + err.Error()
		}
		if _, stErr := os.Stat(cfgPath); stErr != nil {
			item.Valid = false
			item.Issue = "missing sidecar.json"
		}
		if item.Valid {
			if (cfg.Command == "") == (cfg.Builtin == "") {
				item.Valid = false
				item.Issue = "exactly one of command or builtin must be set"
			}
			if cfg.Builtin != "" && cfg.Builtin != "schedule" {
				item.Valid = false
				item.Issue = "unsupported builtin: " + cfg.Builtin
			}
			if cfg.RestartPolicy != "" && cfg.RestartPolicy != "always" && cfg.RestartPolicy != "on-failure" && cfg.RestartPolicy != "never" {
				item.Valid = false
				item.Issue = "invalid restart_policy: " + cfg.RestartPolicy
			}
			if cfg.Builtin == "schedule" && len(cfg.Args) < 2 {
				item.Valid = false
				item.Issue = "schedule builtin requires cron expression plus command"
			} else if cfg.Builtin == "schedule" && !validFiveFieldCron(cfg.Args[0]) {
				item.Valid = false
				item.Issue = "schedule builtin requires a standard 5-field cron expression"
			}
		}
		out = append(out, item)
	}
	return out
}

func InstallDir(p paths.Paths, source, id string) (Item, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return Item{}, fmt.Errorf("source directory required")
	}
	if id == "" {
		id = filepath.Base(source)
	}
	if !validID(id) {
		return Item{}, fmt.Errorf("invalid sidecar id %q", id)
	}
	cfgPath := filepath.Join(source, "sidecar.json")
	cfg, err := jsonx.Read(cfgPath, Config{})
	if err != nil {
		return Item{}, fmt.Errorf("read sidecar.json: %w", err)
	}
	if err := validate(cfg); err != nil {
		return Item{}, err
	}
	dst := filepath.Join(p.SidecarsRoot, id)
	if err := copyTreeReplace(source, dst); err != nil {
		return Item{}, err
	}
	xs, _ := List(p)
	for _, x := range xs {
		if x.ID == id {
			return x, nil
		}
	}
	return Item{}, fmt.Errorf("sidecar installed but not discoverable")
}

func CreateSchedule(p paths.Paths, id, cron, command string, args []string, description string) (Item, error) {
	if !validID(id) {
		return Item{}, fmt.Errorf("invalid sidecar id %q", id)
	}
	if strings.TrimSpace(cron) == "" || strings.TrimSpace(command) == "" {
		return Item{}, fmt.Errorf("cron and command are required")
	}
	if !validFiveFieldCron(cron) {
		return Item{}, fmt.Errorf("cron must be a standard 5-field expression")
	}
	dir := filepath.Join(p.SidecarsRoot, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Item{}, err
	}
	cfg := Config{Builtin: "schedule", Args: append([]string{cron, command}, args...), RestartPolicy: "always", Description: description}
	if err := jsonx.WriteAtomic(filepath.Join(dir, "sidecar.json"), cfg, p.BackupsRoot); err != nil {
		return Item{}, err
	}
	xs, _ := List(p)
	for _, x := range xs {
		if x.ID == id {
			return x, nil
		}
	}
	return Item{}, fmt.Errorf("schedule sidecar not discoverable")
}

func Enable(p paths.Paths, id, projectID string) error { return setEnabled(p, id, true, projectID) }
func Disable(p paths.Paths, id string) error           { return setEnabled(p, id, false, "") }

func setEnabled(p paths.Paths, id string, enabled bool, projectID string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("sidecar id required")
	}
	root, err := jsonx.ReadMap(p.GlobalConfig)
	if err != nil {
		return err
	}
	sidecars, _ := root["sidecars"].(map[string]any)
	if sidecars == nil {
		sidecars = map[string]any{}
	}
	entry := map[string]any{"enabled": enabled}
	if projectID != "" {
		entry["projectId"] = projectID
	}
	sidecars[id] = entry
	root["sidecars"] = sidecars
	return jsonx.WriteAtomic(p.GlobalConfig, root, p.BackupsRoot)
}

func Remove(p paths.Paths, id string) error {
	if strings.Contains(id, "/") {
		return fmt.Errorf("plugin sidecars must be removed through their plugin")
	}
	if !validID(id) {
		return fmt.Errorf("invalid sidecar id")
	}
	if err := os.RemoveAll(filepath.Join(p.SidecarsRoot, id)); err != nil {
		return err
	}
	root, err := jsonx.ReadMap(p.GlobalConfig)
	if err == nil {
		if sc, ok := root["sidecars"].(map[string]any); ok {
			delete(sc, id)
			root["sidecars"] = sc
			_ = jsonx.WriteAtomic(p.GlobalConfig, root, p.BackupsRoot)
		}
	}
	return nil
}

func Doctor(p paths.Paths) []string {
	xs, err := List(p)
	if err != nil {
		return []string{"ERROR: " + err.Error()}
	}
	var out []string
	for _, x := range xs {
		status := "OK"
		if !x.Valid {
			status = "INVALID: " + x.Issue
		}
		if !x.Enabled && x.Valid {
			status = "DISABLED"
		}
		out = append(out, fmt.Sprintf("%-30s %-8s %s", x.ID, x.Scope, status))
	}
	sort.Strings(out)
	return out
}

func loadUserConfig(p paths.Paths) (map[string]UserConfig, error) {
	root, err := jsonx.ReadMap(p.GlobalConfig)
	if err != nil {
		return nil, err
	}
	raw, _ := root["sidecars"].(map[string]any)
	out := map[string]UserConfig{}
	for id, v := range raw {
		b, _ := json.Marshal(v)
		var c UserConfig
		if json.Unmarshal(b, &c) == nil {
			out[id] = c
		}
	}
	return out, nil
}
func validate(c Config) error {
	if (c.Command == "") == (c.Builtin == "") {
		return fmt.Errorf("exactly one of command or builtin must be set")
	}
	if c.Builtin != "" && c.Builtin != "schedule" {
		return fmt.Errorf("unsupported builtin %q", c.Builtin)
	}
	if c.RestartPolicy != "" && c.RestartPolicy != "always" && c.RestartPolicy != "on-failure" && c.RestartPolicy != "never" {
		return fmt.Errorf("invalid restart_policy %q", c.RestartPolicy)
	}
	if c.Builtin == "schedule" && len(c.Args) < 2 {
		return fmt.Errorf("schedule builtin requires cron expression plus command")
	}
	if c.Builtin == "schedule" && !validFiveFieldCron(c.Args[0]) {
		return fmt.Errorf("schedule builtin requires a standard 5-field cron expression")
	}
	return nil
}

func validFiveFieldCron(expr string) bool {
	if strings.ContainsAny(expr, "\r\n") {
		return false
	}
	return len(strings.Fields(expr)) == 5
}

func validID(s string) bool {
	s = strings.TrimSpace(s)
	return s != "" && !strings.ContainsAny(s, "/\\") && !strings.Contains(s, "..")
}
func copyTreeReplace(src, dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, e := filepath.Rel(src, path)
		if e != nil {
			return e
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if e = os.MkdirAll(filepath.Dir(target), 0o755); e != nil {
			return e
		}
		return os.WriteFile(target, b, info.Mode().Perm())
	})
}
