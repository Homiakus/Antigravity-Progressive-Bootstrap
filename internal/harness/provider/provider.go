package provider

import (
	"context"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// Observation is one coherent provider-account snapshot. Adapters backed by a
// single provider payload should expose SnapshotSource so persistence can avoid
// torn reads across independently refreshed capacity/model/session methods.
type Observation struct {
	Capacity harnessmodel.ProviderCapacitySnapshot
	Models   []harnessmodel.ProviderModelDescriptor
	Sessions []harnessmodel.ProviderSessionSnapshot
}

// SnapshotSource exposes capacity, model and session observations derived from
// the same provider payload/revision. It is optional for legacy/adapters whose
// upstream APIs are inherently independent; ingestion should prefer it when
// available.
type SnapshotSource interface {
	Observe(context.Context) (Observation, error)
}

// CapacitySource exposes the latest observable external capacity for one
// provider account. It reports supply; workflow/user spending limits remain in
// the harness budget subsystem.
type CapacitySource interface {
	Capacity(context.Context) (harnessmodel.ProviderCapacitySnapshot, error)
}

// ModelSource discovers models currently exposed by a provider account. Core
// orchestration must not depend on a hard-coded provider model catalog.
type ModelSource interface {
	Models(context.Context) ([]harnessmodel.ProviderModelDescriptor, error)
}

// SessionSource discovers reusable provider sessions/context state for an
// account. Context belongs to sessions rather than quota windows.
type SessionSource interface {
	Sessions(context.Context) ([]harnessmodel.ProviderSessionSnapshot, error)
}

// Adapter is the read/observation boundary for a single provider account.
// Execution is intentionally not part of this contract yet: Antigravity/AGY
// and the existing generic agent executor have different execution contracts,
// and a portable execution interface must be derived from TaskEnvelope rather
// than prematurely coupling those paths.
type Adapter interface {
	Kind() harnessmodel.ProviderKind
	Account() harnessmodel.ProviderAccount
	CapacitySource
	ModelSource
	SessionSource
}
