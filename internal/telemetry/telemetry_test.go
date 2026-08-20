package telemetry

import (
	"github.com/homiakus/agctl/internal/paths"
	"path/filepath"
	"testing"
)

func TestRecordRecentSummary(t *testing.T) {
	root := t.TempDir()
	p := paths.Paths{TelemetryRoot: filepath.Join(root, "telemetry")}
	if err := Record(p, Event{Type: "tool.permission", Decision: "allow", Risk: "read-low"}); err != nil {
		t.Fatal(err)
	}
	xs, err := Recent(p, 10)
	if err != nil || len(xs) != 1 {
		t.Fatalf("xs=%v err=%v", xs, err)
	}
	s := Summarize(xs)
	if s.Decisions["allow"] != 1 {
		t.Fatalf("summary=%+v", s)
	}
}
