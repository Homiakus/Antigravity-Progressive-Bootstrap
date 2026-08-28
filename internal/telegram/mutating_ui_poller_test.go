package telegram

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
)

type fakeUIHandler struct{ callbacks int }

func (f *fakeUIHandler) HandleCommand(context.Context, Message, model.TelegramPrincipal) (bool, error) {
	return false, nil
}
func (f *fakeUIHandler) HandleCallback(context.Context, CallbackQuery, model.TelegramPrincipal) (bool, error) {
	f.callbacks++
	return true, nil
}

func TestMutatingUICallbackRequiresOperator(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	callback := CallbackQuery{
		ID: "open-1",
		From: User{ID: 42},
		Message: &Message{MessageID: 5, Chat: Chat{ID: 99}},
		Data: "u2|accounts|rep_1700000000000_00000000000000000001|0",
	}
	pair, _ := NewPairingService(&fakePairStore{}, bytes.NewReader(make([]byte, 20)), func() time.Time { return now })

	viewerAPI := &fakeAPI{updates: []Update{{UpdateID: 1, CallbackQuery: &callback}}}
	viewerStore := &fakePollStore{principal: model.TelegramPrincipal{UserID: 42, Role: model.TelegramRoleViewer, Enabled: true, PairedAt: now}}
	viewerUI := &fakeUIHandler{}
	viewer, err := NewPoller(PollerOptions{BotKey: "viewer", API: viewerAPI, Store: viewerStore, Pairing: pair, UI: viewerUI, IDs: &staticIDs{}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := viewer.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if viewerUI.callbacks != 0 {
		t.Fatalf("viewer reached mutating UI: calls=%d", viewerUI.callbacks)
	}
	if len(viewerAPI.answers) == 0 || viewerAPI.answers[0] != "open-1:Not authorized" {
		t.Fatalf("viewer answers=%v", viewerAPI.answers)
	}

	operatorAPI := &fakeAPI{updates: []Update{{UpdateID: 1, CallbackQuery: &callback}}}
	operatorStore := &fakePollStore{principal: model.TelegramPrincipal{UserID: 42, Role: model.TelegramRoleOperator, Enabled: true, PairedAt: now}}
	operatorUI := &fakeUIHandler{}
	operator, err := NewPoller(PollerOptions{BotKey: "operator", API: operatorAPI, Store: operatorStore, Pairing: pair, UI: operatorUI, IDs: &staticIDs{}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := operator.PollOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if operatorUI.callbacks != 1 {
		t.Fatalf("operator UI calls=%d", operatorUI.callbacks)
	}
	if !operatorStore.callbacks["open-1"] {
		t.Fatal("mutating callback was not replay-reserved before UI execution")
	}
}
