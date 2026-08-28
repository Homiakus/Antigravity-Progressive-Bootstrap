package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
)

type fakeViewAPI struct {
	sent   View
	edited View
}

func (f *fakeViewAPI) SendView(context.Context, int64, int64, View) (Message, error) {
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
}

func (f *fakeUIStore) ListRepositories(context.Context, bool) ([]model.Repository, error) {
	return []model.Repository{f.repo}, nil
}
func (f *fakeUIStore) ListInstances(context.Context) ([]model.InstanceMirror, error) {
	return []model.InstanceMirror{f.instance}, nil
}
func (f *fakeUIStore) ListSessionsByInstance(context.Context, model.InstanceID, bool) ([]model.RemoteSession, error) {
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

func TestSessionViewActionsStayWithinTelegramCallbackLimit(t *testing.T) {
	now := time.Unix(1, 0).UTC()
	sid := model.RemoteSessionID("rsi_1700000000000_00000000000000000001")
	store := &fakeUIStore{
		repo: model.Repository{
			ID:            "rep_1700000000000_00000000000000000001",
			Name:          "repo",
			CanonicalPath: "/repo",
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
			Data:    "u1|session|" + string(sid),
		},
		model.TelegramPrincipal{UserID: 1, Role: model.TelegramRoleOwner, Enabled: true, PairedAt: now},
	)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if !strings.Contains(api.edited.Text, "Project: repo") {
		t.Fatalf("view=%q", api.edited.Text)
	}
	for _, row := range api.edited.Keyboard.InlineKeyboard {
		for _, button := range row {
			if len([]byte(button.CallbackData)) > 64 {
				t.Fatalf("callback too long: %s", button.CallbackData)
			}
		}
	}
}
