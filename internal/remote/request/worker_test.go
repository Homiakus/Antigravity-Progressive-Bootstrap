package request

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotesession "github.com/homiakus/agctl/internal/remote/session"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type fakeStore struct {
	binding    []model.RemoteSessionRequest
	pending    *model.RemoteSessionRequest
	attachErr  error
	bindings   []model.TelegramBinding
	completed  []model.RemoteSessionRequestID
	failed     []model.RemoteSessionRequestID
	failText   string
}

func (f *fakeStore) ListSessionRequests(_ context.Context, states []model.SessionRequestState, _ int) ([]model.RemoteSessionRequest, error) {
	for _, state := range states {
		if state == model.SessionRequestBinding {
			return append([]model.RemoteSessionRequest(nil), f.binding...), nil
		}
	}
	return nil, nil
}
func (f *fakeStore) ClaimPendingSessionRequest(_ context.Context, at time.Time) (model.RemoteSessionRequest, error) {
	if f.pending == nil {
		return model.RemoteSessionRequest{}, remotestore.ErrNotFound
	}
	request := *f.pending
	f.pending = nil
	request.State = model.SessionRequestProvisioning
	request.StartedAt = &at
	return request, nil
}
func (f *fakeStore) AttachSessionToRequest(_ context.Context, _ model.RemoteSessionRequestID, _ model.RemoteSessionID) error {
	return f.attachErr
}
func (f *fakeStore) CompleteSessionRequest(_ context.Context, id model.RemoteSessionRequestID, _ time.Time) error {
	f.completed = append(f.completed, id)
	return nil
}
func (f *fakeStore) FailSessionRequest(_ context.Context, id model.RemoteSessionRequestID, message string, _ time.Time) error {
	f.failed = append(f.failed, id)
	f.failText = message
	return nil
}
func (f *fakeStore) ReplaceTelegramBinding(_ context.Context, binding model.TelegramBinding) error {
	f.bindings = append(f.bindings, binding)
	return nil
}

type fakeProvisioner struct {
	session model.RemoteSession
	err     error
	calls   int
	spec    remotesession.Spec
}
func (f *fakeProvisioner) Provision(_ context.Context, spec remotesession.Spec) (model.RemoteSession, error) {
	f.calls++
	f.spec = spec
	return f.session, f.err
}

type fakeIDs struct{ n int }
func (f *fakeIDs) New(kind model.IDKind) (string, error) {
	if kind != model.IDTelegramBinding {
		return "", fmt.Errorf("unexpected id kind %s", kind)
	}
	f.n++
	return fmt.Sprintf("tgb_1700000000000_%020x", f.n), nil
}

func requestFixture(id string, state model.SessionRequestState) model.RemoteSessionRequest {
	return model.RemoteSessionRequest{
		ID:                   model.RemoteSessionRequestID(id),
		RepositoryID:         "rep_1700000000000_00000000000000000001",
		AccountID:            "account-1",
		ChatID:               99,
		ThreadID:             7,
		RequesterUserID:      42,
		InstanceStrategy:     "AUTO",
		ConversationStrategy: "NEW",
		IsolationMode:        model.IsolationExclusiveWrite,
		State:                state,
	}
}

func TestWorkerDrainsBindingThenProvisionsPendingRequest(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	binding := requestFixture("rsq_1700000000000_00000000000000000001", model.SessionRequestBinding)
	binding.SessionID = "rsi_1700000000000_00000000000000000001"
	pending := requestFixture("rsq_1700000000000_00000000000000000002", model.SessionRequestPending)
	store := &fakeStore{binding: []model.RemoteSessionRequest{binding}, pending: &pending}
	provisioner := &fakeProvisioner{session: model.RemoteSession{ID: "rsi_1700000000000_00000000000000000002"}}
	ids := &fakeIDs{}
	worker := &Worker{Store: store, Provisioner: provisioner, IDs: ids, Now: func() time.Time { return now }}

	processed, err := worker.RunOnce(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if processed != 2 || provisioner.calls != 1 {
		t.Fatalf("processed=%d provision calls=%d", processed, provisioner.calls)
	}
	if provisioner.spec.RepositoryID != pending.RepositoryID || provisioner.spec.AccountID != pending.AccountID || provisioner.spec.InstanceStrategy != remotesession.InstanceAuto {
		t.Fatalf("provision spec=%#v", provisioner.spec)
	}
	if len(store.bindings) != 2 || store.bindings[0].SessionID != binding.SessionID || store.bindings[1].SessionID != provisioner.session.ID {
		t.Fatalf("bindings=%#v", store.bindings)
	}
	if len(store.completed) != 2 || store.completed[0] != binding.ID || store.completed[1] != pending.ID {
		t.Fatalf("completed=%v", store.completed)
	}
}

func TestWorkerPersistsProvisionFailureWithoutBinding(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	pending := requestFixture("rsq_1700000000000_00000000000000000001", model.SessionRequestPending)
	store := &fakeStore{pending: &pending}
	provisioner := &fakeProvisioner{err: errors.New("bridge unavailable")}
	worker := &Worker{Store: store, Provisioner: provisioner, IDs: &fakeIDs{}, Now: func() time.Time { return now }}

	processed, err := worker.RunOnce(context.Background(), 10)
	if processed != 1 || err == nil {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	if len(store.failed) != 1 || store.failed[0] != pending.ID || store.failText != "bridge unavailable" {
		t.Fatalf("failed=%v text=%q", store.failed, store.failText)
	}
	if len(store.bindings) != 0 {
		t.Fatalf("unexpected bindings=%#v", store.bindings)
	}
}

func TestWorkerFailsClosedWhenDurableSessionAttachFails(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	pending := requestFixture("rsq_1700000000000_00000000000000000001", model.SessionRequestPending)
	store := &fakeStore{pending: &pending, attachErr: errors.New("database unavailable")}
	provisioner := &fakeProvisioner{session: model.RemoteSession{ID: "rsi_1700000000000_00000000000000000001"}}
	worker := &Worker{Store: store, Provisioner: provisioner, IDs: &fakeIDs{}, Now: func() time.Time { return now }}

	processed, err := worker.RunOnce(context.Background(), 10)
	if processed != 1 || err == nil {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	if len(store.failed) != 0 {
		t.Fatalf("ambiguous provision must not be marked failed: %v", store.failed)
	}
	if len(store.bindings) != 0 || len(store.completed) != 0 {
		t.Fatalf("ambiguous provision advanced: bindings=%v completed=%v", store.bindings, store.completed)
	}
	if provisioner.calls != 1 {
		t.Fatalf("provision calls=%d", provisioner.calls)
	}
}
