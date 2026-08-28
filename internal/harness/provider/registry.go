package provider

import (
	"fmt"
	"sort"
	"sync"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// Registry is the in-process directory of provider-account adapters. Durable
// provider observations live in the harness Store; this registry only binds an
// account identity to the live adapter capable of refreshing it.
type Registry struct {
	mu       sync.RWMutex
	accounts map[harnessmodel.ProviderAccountID]Adapter
}

func NewRegistry() *Registry {
	return &Registry{accounts: make(map[harnessmodel.ProviderAccountID]Adapter)}
}

func (r *Registry) Register(adapter Adapter) error {
	if adapter == nil {
		return fmt.Errorf("provider adapter is required")
	}
	kind := adapter.Kind()
	if !kind.Valid() {
		return fmt.Errorf("invalid provider kind %q", kind)
	}
	account := adapter.Account()
	if account.ID == "" {
		return fmt.Errorf("provider account id is required")
	}
	if !account.Provider.Valid() {
		return fmt.Errorf("provider account %s has invalid provider %q", account.ID, account.Provider)
	}
	if account.Provider != kind {
		return fmt.Errorf("provider account %s kind mismatch: adapter=%s account=%s", account.ID, kind, account.Provider)
	}
	if !account.State.Valid() {
		return fmt.Errorf("provider account %s has invalid state %q", account.ID, account.State)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.accounts == nil {
		r.accounts = make(map[harnessmodel.ProviderAccountID]Adapter)
	}
	if _, exists := r.accounts[account.ID]; exists {
		return fmt.Errorf("provider account %s is already registered", account.ID)
	}
	r.accounts[account.ID] = adapter
	return nil
}

func (r *Registry) Get(accountID harnessmodel.ProviderAccountID) (Adapter, bool) {
	if r == nil || accountID == "" {
		return nil, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	adapter, ok := r.accounts[accountID]
	return adapter, ok
}

// List returns a deterministic snapshot sorted by provider account ID so
// callers and tests do not depend on Go map iteration order.
func (r *Registry) List() []Adapter {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]harnessmodel.ProviderAccountID, 0, len(r.accounts))
	for id := range r.accounts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]Adapter, 0, len(ids))
	for _, id := range ids {
		out = append(out, r.accounts[id])
	}
	return out
}
