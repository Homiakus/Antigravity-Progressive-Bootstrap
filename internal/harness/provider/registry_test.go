package provider

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type fakeAdapter struct {
	kind    harnessmodel.ProviderKind
	account harnessmodel.ProviderAccount
}

func (f *fakeAdapter) Kind() harnessmodel.ProviderKind { return f.kind }
func (f *fakeAdapter) Account() harnessmodel.ProviderAccount { return f.account }
func (f *fakeAdapter) Capacity(context.Context) (harnessmodel.ProviderCapacitySnapshot, error) {
	return harnessmodel.ProviderCapacitySnapshot{AccountID: f.account.ID, Provider: f.kind, Health: harnessmodel.ProviderHealthHealthy, ObservedAt: time.Unix(1, 0).UTC()}, nil
}
func (f *fakeAdapter) Models(context.Context) ([]harnessmodel.ProviderModelDescriptor, error) {
	return []harnessmodel.ProviderModelDescriptor{{AccountID: f.account.ID, Provider: f.kind, ID: "model", Enabled: true}}, nil
}
func (f *fakeAdapter) Sessions(context.Context) ([]harnessmodel.ProviderSessionSnapshot, error) {
	return nil, nil
}

func validFake(id harnessmodel.ProviderAccountID, kind harnessmodel.ProviderKind) *fakeAdapter {
	return &fakeAdapter{
		kind: kind,
		account: harnessmodel.ProviderAccount{
			ID: id, Provider: kind, State: harnessmodel.ProviderAccountActive,
		},
	}
}

func TestRegistryRegisterGetAndListDeterministically(t *testing.T) {
	r := NewRegistry()
	b := validFake("pacc_1787918400000_00000000000000000002", harnessmodel.ProviderCodex)
	a := validFake("pacc_1787918400000_00000000000000000001", harnessmodel.ProviderAntigravity)
	if err := r.Register(b); err != nil { t.Fatal(err) }
	if err := r.Register(a); err != nil { t.Fatal(err) }

	got, ok := r.Get(a.account.ID)
	if !ok || got.Account().ID != a.account.ID {
		t.Fatalf("Get(%s) = %#v, %v", a.account.ID, got, ok)
	}
	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List len = %d, want 2", len(list))
	}
	if list[0].Account().ID != a.account.ID || list[1].Account().ID != b.account.ID {
		t.Fatalf("List order = [%s %s]", list[0].Account().ID, list[1].Account().ID)
	}
}

func TestRegistryRejectsDuplicateAndMismatchedAccounts(t *testing.T) {
	r := NewRegistry()
	a := validFake("pacc_1787918400000_00000000000000000001", harnessmodel.ProviderCodex)
	if err := r.Register(a); err != nil { t.Fatal(err) }
	if err := r.Register(a); err == nil {
		t.Fatal("duplicate account unexpectedly accepted")
	}

	mismatch := validFake("pacc_1787918400000_00000000000000000002", harnessmodel.ProviderCodex)
	mismatch.kind = harnessmodel.ProviderAntigravity
	if err := r.Register(mismatch); err == nil {
		t.Fatal("provider kind mismatch unexpectedly accepted")
	}
}

func TestRegistryRejectsInvalidAdapterIdentity(t *testing.T) {
	tests := []struct {
		name string
		adapter Adapter
	}{
		{name: "nil", adapter: nil},
		{name: "missing id", adapter: validFake("", harnessmodel.ProviderCodex)},
		{name: "invalid kind", adapter: validFake("pacc_1787918400000_00000000000000000001", harnessmodel.ProviderKind("OTHER"))},
		{name: "invalid state", adapter: func() Adapter {
			a := validFake("pacc_1787918400000_00000000000000000001", harnessmodel.ProviderCodex)
			a.account.State = harnessmodel.ProviderAccountState("BROKEN")
			return a
		}()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := NewRegistry().Register(tt.adapter); err == nil {
				t.Fatal("invalid adapter unexpectedly accepted")
			}
		})
	}
}

func TestRegistryConcurrentRegistrationAndReads(t *testing.T) {
	r := NewRegistry()
	const count = 32
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := harnessmodel.ProviderAccountID(fmt.Sprintf("pacc_1787918400000_%020x", i+1))
			if err := r.Register(validFake(id, harnessmodel.ProviderCodex)); err != nil {
				t.Errorf("Register(%s): %v", id, err)
				return
			}
			if _, ok := r.Get(id); !ok {
				t.Errorf("Get(%s) missing after register", id)
			}
		}()
	}
	wg.Wait()
	if got := len(r.List()); got != count {
		t.Fatalf("List len = %d, want %d", got, count)
	}
}
