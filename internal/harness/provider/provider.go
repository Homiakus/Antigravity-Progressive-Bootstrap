package provider

import (
	"context"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

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
