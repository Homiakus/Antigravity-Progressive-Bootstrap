package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
)

type fakeViewAPI struct {
	sent   View
	edited View
}

func (f *fakeViewAPI) SendView(_ context.Context, _ int64, _ int64, view View) (Message, error) {
	f.sent = view
	return Message{MessageID: 1}, nil
}

func (f *fakeViewAPI) EditView(_ context.Context, _ int64, _ int64, view View) error {
	f.edited = view
	return nil
}

func (f *fakeViewAPI) AnswerCallback(context.Context, string, string) error { return nil }

type fakeUIStore struct {
	repo     model.Repository
	instance model.InstanceMirror
	session  model.RemoteSession
	conv     model.Conversation
	requests []model.RemoteSessionRequest
}

func (f *fakeUIStore) ListRepositories(context.Context, bool) ([]model.Repository, error) {
	return []model.Repository{f.repo}, nil
}
func (f *fakeUIStore) ListInstances(context.Context) ([]model.InstanceMirror, error) {
	return []model.InstanceMirror{f.instance}, nil
}
func (f *fakeUIStore) ListSessionsByInstance(context.Context, model.InstanceID, bool) ([]model.RemoteSession, error) {
	if f.session.ID == "" {
		return nil, nil
	}
	return []model.RemoteSession{f.session}, nil
}
func (f *fakeUIStore) GetSession(context.Context, model.RemoteSessionID) (model.RemoteSession, error) {
	return f.session, nil
}
func (f *fakeUIStore) GetRepository(context.Context, model.RepositoryID) (model.Repository, error) {
	return f.repo, nil
}
func (f *fakeUIStore) GetConversation(context.Context, model.ConversationID) (model.Conversation, error) {
	return f.conv, nil
}
func (f *fakeUIStore) GetInstance(context.Context, model.InstanceID) (model.InstanceMirror, error) {
	return f.instance, nil
}
func (f *fakeUIStore) AdmitSessionRequest(_ context.Context, request model.RemoteSessionRequest) (model.RemoteSessionRequest, bool, error) {
	for _, existing := range f.requests {
		if existing.Source == request.Source && existing.SourceMessageID == request.SourceMessageID {
			return existing, false, nil
		}
	}
	f.requests = append(f.requests, request)
	return request, true, nil
}

type fakeUIAccounts struct{ items []cockpit.Account }
func (f fakeUIAccounts) Accounts(context.Context) ([]cockpit.Account, error) {
	return append([]cockpit.Account(nil), f.items...), nil
}

func baseUIStore(now time.Time) *fakeUIStore {
	sid := model.RemoteSessionID("rsi_1700000000000_00000000000000000001")
	return &fakeUIStore{
		repo: model.Repository{
			ID:            "rep_1700000000000_00000000000000000001",
			Name:          "repo",
			CanonicalPath: "/repo",
			Enabled:       true,
			CreatedAt:     now,
			LastSeenAt:    now,
		},
		instance: model.InstanceMirror{
			ID:               "i1",
			UserDataDir:      "/p",
			ObservedState:    model.InstanceReady,
			DesiredState:     model.InstanceDesiredRunning,
			LastReconciledAt: now,
		},
		session: model.RemoteSession{
			ID:                sid,
			RepositoryID:      "rep_1700000000000_00000000000000000001",
			ConversationID:    "rcv_1700000000000_00000000000000000001",
			CockpitInstanceID: "i1",
			WorkspacePath:     "/repo",
			IsolationMode:     model.IsolationExclusiveWrite,
			ObservedState:     model.SessionReady,
		},
		conv: model.Conversation{
			ID:                     "rcv_1700000000000_00000000000000000001",
			ProviderConversationID: "p1",
			Title:                  "chat",
		},
	}
}

func TestSessionViewActionsStayWithinTelegramCallbackLimit(t *testing.T) {
	now := time.Unix(1, 0).UTC()
	store := baseUIStore(now)
	api := &fakeViewAPI{}
	ui, err := NewUI(api, store, nil)
	if err != nil {
		t.Fatal(err)
	}
	handled, err := ui.HandleCallback(
		context.Background(),
		CallbackQuery{
			ID:      "cb",
			From:    User{ID: 1},
			Message: &Message{MessageID: 7, Chat: Chat{ID: 9}},
			Data:    "u1|session|" + string(store.session.ID),
		},
		model.TelegramPrincipal{UserID: 1, Role: model.TelegramRoleOwner, Enabled: true, PairedAt: now},
	)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if !strings.Contains(api.edited.Text, "Project: repo") {
		t.Fatalf("view=%q", api.edited.Text)
	}
	assertCallbackLengths(t, api.edited)
}

func TestProjectOpenFlowUsesShortAccountReferenceAndAdmitsRequest(t *testing.T) {
	now := time.Unix(1, 0).UTC()
	store := baseUIStore(now)
	store.session = model.RemoteSession{}
	accountID := "account-id-that-is-deliberately-long-and-must-not-enter-callback-data"
	accounts := fakeUIAccounts{items: []cockpit.Account{{ID: accountID, Email: "operator@example.com", Plan: "pro"}}}
	api := &fakeViewAPI{}
	ui, err := NewUI(api, store, accounts)
	if err != nil {
		t.Fatal(err)
	}
	principal := model.TelegramPrincipal{UserID: 42, Role: model.TelegramRoleOperator, Enabled: true, PairedAt: now}

	handled, err := ui.HandleCallback(context.Background(), CallbackQuery{
		ID: "project", From: User{ID: 42}, Message: &Message{MessageID: 7, Chat: Chat{ID: 99}, MessageThreadID: 3},
		Data: "u1|project|" + string(store.repo.ID),
	}, principal)
	if err != nil || !handled {
		t.Fatalf("project handled=%v err=%v", handled, err)
	}
	accountsCallback := findCallback(api.edited, "u2|accounts|")
	if accountsCallback == "" {
		t.Fatalf("project view has no new-session callback: %#v", api.edited.Keyboard)
	}

	handled, err = ui.HandleCallback(context.Background(), CallbackQuery{
		ID: "accounts", From: User{ID: 42}, Message: &Message{MessageID: 7, Chat: Chat{ID: 99}, MessageThreadID: 3},
		Data: accountsCallback,
	}, principal)
	if err != nil || !handled {
		t.Fatalf("accounts handled=%v err=%v", handled, err)
	}
	openCallback := findCallback(api.edited, "u2|open|")
	if openCallback == "" {
		t.Fatalf("account view has no open callback: %#v", api.edited.Keyboard)
	}
	if strings.Contains(openCallback, accountID) {
		t.Fatalf("raw account id leaked into callback: %q", openCallback)
	}
	if len([]byte(openCallback)) > 64 {
		t.Fatalf("open callback too long: %d %q", len([]byte(openCallback)), openCallback)
	}

	handled, err = ui.HandleCallback(context.Background(), CallbackQuery{
		ID: "open-callback-1", From: User{ID: 42}, Message: &Message{MessageID: 7, Chat: Chat{ID: 99}, MessageThreadID: 3},
		Data: openCallback,
	}, principal)
	if err != nil || !handled {
		t.Fatalf("open handled=%v err=%v", handled, err)
	}
	if len(store.requests) != 1 {
		t.Fatalf("requests=%d", len(store.requests))
	}
	request := store.requests[0]
	if request.RepositoryID != store.repo.ID || request.AccountID != accountID || request.ChatID != 99 || request.ThreadID != 3 || request.RequesterUserID != 42 {
		t.Fatalf("request=%#v", request)
	}
	if request.InstanceStrategy != "AUTO" || request.ConversationStrategy != "NEW" || request.State != model.SessionRequestPending {
		t.Fatalf("request policy=%#v", request)
	}
	assertCallbackLengths(t, api.edited)
}

func findCallback(view View, prefix string) string {
	for _, row := range view.Keyboard.InlineKeyboard {
		for _, button := range row {
			if strings.HasPrefix(button.CallbackData, prefix) {
				return button.CallbackData
			}
	}
	}
	return ""
}

func assertCallbackLengths(t *testing.T, view View) {
	t.Helper()
	for _, row := range view.Keyboard.InlineKeyboard {
		for _, button := range row {
			if len([]byte(button.CallbackData)) > 64 {
				t.Fatalf("callback too long: %s", button.CallbackData)
			}
		}
	}
}
