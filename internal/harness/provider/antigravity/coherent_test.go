package antigravity

import (
	"context"
	"fmt"
	"testing"
	"time"

	harnessprovider "github.com/homiakus/agctl/internal/harness/provider"
)

func TestObserveUsesOneStatusLineReadForCoherentSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 28, 13, 45, 0, 0, time.UTC)
	reads := 0
	source := StatusLineSourceFunc(func(context.Context) ([]byte, time.Time, error) {
		reads++
		return []byte(fmt.Sprintf(`{
			"product":"antigravity",
			"conversation_id":"conv-%d",
			"model":{"id":"model-%d"},
			"context_window":{"context_window_size":1000,"used_percentage":10},
			"quota":{"bucket":{"remaining_fraction":0.9}}
		}`, reads, reads)), now.Add(time.Duration(reads) * time.Second), nil
	})
	adapter, err := NewAdapter(testAccount(now), source)
	if err != nil {
		t.Fatal(err)
	}

	var snapshots harnessprovider.SnapshotSource = adapter
	obs, err := snapshots.Observe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if reads != 1 {
		t.Fatalf("status-line reads=%d want=1", reads)
	}
	if len(obs.Models) != 1 || len(obs.Sessions) != 1 {
		t.Fatalf("unexpected coherent observation: %+v", obs)
	}
	if string(obs.Models[0].ID) != "model-1" || string(obs.Sessions[0].ModelID) != "model-1" || string(obs.Sessions[0].ID) != "conv-1" {
		t.Fatalf("observation was torn across provider payloads: %+v", obs)
	}
	if !obs.Capacity.ObservedAt.Equal(now.Add(time.Second)) {
		t.Fatalf("capacity observedAt=%s want=%s", obs.Capacity.ObservedAt, now.Add(time.Second))
	}
}
