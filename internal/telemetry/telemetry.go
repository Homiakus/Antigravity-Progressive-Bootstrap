package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/paths"
)

type Event struct {
	Timestamp      string         `json:"timestamp"`
	Type           string         `json:"type"`
	ConversationID string         `json:"conversationId,omitempty"`
	Tool           string         `json:"tool,omitempty"`
	Decision       string         `json:"decision,omitempty"`
	Risk           string         `json:"risk,omitempty"`
	Reason         string         `json:"reason,omitempty"`
	Workspace      []string       `json:"workspace,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
}

type Summary struct {
	Total     int            `json:"total"`
	ByType    map[string]int `json:"byType"`
	Decisions map[string]int `json:"decisions"`
	Risks     map[string]int `json:"risks"`
}

// Record uses one-file-per-event storage so independent hook processes do not
// contend on a shared append file or require an external file-lock dependency.
func Record(p paths.Paths, e Event) error {
	if p.TelemetryRoot == "" {
		return nil
	}
	now := time.Now().UTC()
	e.Timestamp = now.Format(time.RFC3339Nano)
	dir := filepath.Join(p.TelemetryRoot, "events", now.Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, fmt.Sprintf("%s-%d-*.json", now.Format("150405.000000000"), os.Getpid()))
	if err != nil {
		return err
	}
	name := f.Name()
	if _, err = f.Write(append(b, '\n')); err != nil {
		f.Close()
		_ = os.Remove(name)
		return err
	}
	if err = f.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func Recent(p paths.Paths, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}
	root := filepath.Join(p.TelemetryRoot, "events")
	var files []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".json") {
			files = append(files, path)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	if len(files) > limit {
		files = files[:limit]
	}
	var out []Event
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var e Event
		if json.Unmarshal(b, &e) == nil {
			out = append(out, e)
		}
	}
	return out, nil
}

func Summarize(events []Event) Summary {
	s := Summary{Total: len(events), ByType: map[string]int{}, Decisions: map[string]int{}, Risks: map[string]int{}}
	for _, e := range events {
		s.ByType[e.Type]++
		if e.Decision != "" {
			s.Decisions[e.Decision]++
		}
		if e.Risk != "" {
			s.Risks[e.Risk]++
		}
	}
	return s
}
