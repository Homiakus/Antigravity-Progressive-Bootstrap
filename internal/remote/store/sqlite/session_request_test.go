package sqlite

import (
	"errors"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

func sessionRequestFixture(now time.Time) model.RemoteSessionRequest {
	return model.RemoteSessionRequest{
		ID: "rsq_1700000000000_00000000000000000001",
		Source: "telegram",
		SourceMessageID: "callback:open-77",
		RepositoryID: "rep_1700000000000_00000000000000000001",
		AccountID: "account-1",
		ChatID: 99,
		ThreadID: 7,
		RequesterUserID: 42,
		InstanceStrategy: "AUTO",
		ConversationStrategy: "NEW",
		IsolationMode: model.IsolationExclusiveWrite,
		State: model.SessionRequestPending,
		RequestedAt: now,
	}
}

func TestSessionRequestAdmissionClaimAttachCompleteIsIdempotent(t *testing.T) {
	ctx, store, done := openRemoteTestStore(t)
	defer done()
	seedRemoteGraph(t, ctx, store)
	now := time.Unix(1700000000, 0).UTC()
	request := sessionRequestFixture(now)
	got, created, err := store.AdmitSessionRequest(ctx, request)
	if err != nil || !created || got.ID != request.ID {
		t.Fatalf("admit created=%v got=%#v err=%v", created, got, err)
	}
	duplicate := request
	duplicate.ID = "rsq_1700000000000_00000000000000000002"
	got, created, err = store.AdmitSessionRequest(ctx, duplicate)
	if err != nil || created || got.ID != request.ID {
		t.Fatalf("dedupe created=%v got=%#v err=%v", created, got, err)
	}
	claimed, err := store.ClaimPendingSessionRequest(ctx, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ID != request.ID || claimed.State != model.SessionRequestProvisioning || claimed.StartedAt == nil {
		t.Fatalf("claimed=%#v", claimed)
	}
	if _, err := store.ClaimPendingSessionRequest(ctx, now.Add(2*time.Second)); !errors.Is(err, remotestore.ErrNotFound) {
		t.Fatalf("second claim err=%v", err)
	}
	const sessionID = model.RemoteSessionID("rsi_1700000000000_00000000000000000001")
	if err := store.AttachSessionToRequest(ctx, request.ID, sessionID); err != nil {
		t.Fatal(err)
	}
	if err := store.AttachSessionToRequest(ctx, request.ID, sessionID); err != nil {
		t.Fatalf("attach replay: %v", err)
	}
	binding, err := store.ListSessionRequests(ctx, []model.SessionRequestState{model.SessionRequestBinding}, 10)
	if err != nil || len(binding) != 1 || binding[0].SessionID != sessionID {
		t.Fatalf("binding=%#v err=%v", binding, err)
	}
	if err := store.CompleteSessionRequest(ctx, request.ID, now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteSessionRequest(ctx, request.ID, now.Add(4*time.Second)); err != nil {
		t.Fatalf("complete replay: %v", err)
	}
	final, err := store.GetSessionRequest(ctx, request.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.State != model.SessionRequestSucceeded || final.SessionID != sessionID || final.CompletedAt == nil {
		t.Fatalf("final=%#v", final)
	}
}

func TestSessionRequestMigrationRejectsBindingWithoutSession(t *testing.T) {
	ctx, store, done := openRemoteTestStore(t)
	defer done()
	seedRemoteGraph(t, ctx, store)
	request := sessionRequestFixture(time.Unix(1700000000, 0).UTC())
	request.State = model.SessionRequestBinding
	if _, _, err := store.AdmitSessionRequest(ctx, request); err == nil {
		t.Fatal("expected non-pristine admission to fail")
	}
	var table string
	if err := store.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='remote_session_requests'`).Scan(&table); err != nil || table != "remote_session_requests" {
		t.Fatalf("migration table=%q err=%v", table, err)
	}
}

func TestSessionRequestIDKind(t *testing.T) {
	if err := model.ValidateGeneratedID("rsq_1700000000000_00000000000000000001", model.IDRemoteSessionRequest); err != nil {
		t.Fatal(err)
	}
}
