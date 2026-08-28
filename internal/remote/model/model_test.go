package model

import (
	"bytes"
	"testing"
	"time"
)

func deterministicGenerator() TimeSortableIDGenerator {
	return TimeSortableIDGenerator{Now: func() time.Time { return time.UnixMilli(1700000000000) }, Random: bytes.NewReader(make([]byte, 128))}
}

func mustID(t *testing.T, g TimeSortableIDGenerator, kind IDKind) string {
	t.Helper()
	id, err := g.New(kind)
	if err != nil { t.Fatal(err) }
	return id
}

func TestGeneratedIDsAreTypedAndValidated(t *testing.T) {
	g := deterministicGenerator()
	for _, kind := range []IDKind{IDRepository, IDWorkspace, IDConversation, IDRemoteSession, IDTelegramBinding, IDRemoteCommand, IDRemoteEvent} {
		id := mustID(t, g, kind)
		if err := ValidateGeneratedID(id, kind); err != nil { t.Fatalf("kind %s: %v", kind, err) }
	}
}

func TestRemoteSessionValidationRejectsInvalidState(t *testing.T) {
	g := deterministicGenerator()
	now := time.Now().UTC()
	session := RemoteSession{
		ID: RemoteSessionID(mustID(t, g, IDRemoteSession)), HostID: "host-local", CockpitInstanceID: "cockpit-instance-1",
		RepositoryID: RepositoryID(mustID(t, g, IDRepository)), WorkspaceID: WorkspaceID(mustID(t, g, IDWorkspace)), WorkspacePath: "/tmp/repo",
		ConversationID: ConversationID(mustID(t, g, IDConversation)), DesiredState: SessionDesiredReady, ObservedState: SessionObservedState("IMPOSSIBLE"),
		IsolationMode: IsolationExclusiveWrite, CreatedAt: now, UpdatedAt: now,
	}
	if err := session.Validate(); err == nil { t.Fatal("expected invalid observed state to fail validation") }
}

func TestStateEnumerationsRejectUnknownValues(t *testing.T) {
	if SessionDesiredState("X").Valid() || SessionObservedState("X").Valid() || IsolationMode("X").Valid() || MirrorMode("X").Valid() { t.Fatal("unknown enum value unexpectedly accepted") }
}
