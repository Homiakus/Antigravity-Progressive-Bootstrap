package antigravity

import (
	"context"

	harnessprovider "github.com/homiakus/agctl/internal/harness/provider"
)

var _ harnessprovider.SnapshotSource = (*Adapter)(nil)

// Observe returns capacity, model and session views derived from the same
// status-line payload. Generic ingestion should prefer this coherent path over
// making three independent live reads through Capacity/Models/Sessions.
func (a *Adapter) Observe(ctx context.Context) (harnessprovider.Observation, error) {
	obs, err := a.Snapshot(ctx)
	if err != nil {
		return harnessprovider.Observation{}, err
	}
	return harnessprovider.Observation{
		Capacity: obs.Capacity,
		Models:   obs.Models,
		Sessions: obs.Sessions,
	}, nil
}
