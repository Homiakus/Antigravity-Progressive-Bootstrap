package sqlite

import (
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
)

func TestReplaceTelegramBindingAtomicallyMovesTopic(t *testing.T) {
	ctx, store, done := openRemoteTestStore(t)
	defer done()
	seedRemoteGraph(t, ctx, store)
	now := time.Unix(1000, 0).UTC()
	first := model.TelegramBinding{
		ID: "tgb_1700000000000_00000000000000000001", SessionID: "rsi_1700000000000_00000000000000000001",
		ChatID: 99, ThreadID: 7, OwnerUserID: 42, Enabled: true, CreatedAt: now,
	}
	if err := store.ReplaceTelegramBinding(ctx, first); err != nil {
		t.Fatal(err)
	}
	second := first
	second.ID = "tgb_1700000000000_00000000000000000002"
	second.OwnerUserID = 43
	second.CreatedAt = now.Add(time.Second)
	if err := store.ReplaceTelegramBinding(ctx, second); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetTelegramBindingByTopic(ctx, 99, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != second.ID || got.OwnerUserID != 43 {
		t.Fatalf("binding=%#v", got)
	}
	var enabledFirst int
	if err := store.db.QueryRowContext(ctx, `SELECT enabled FROM telegram_bindings WHERE id=?`, first.ID).Scan(&enabledFirst); err != nil {
		t.Fatal(err)
	}
	if enabledFirst != 0 {
		t.Fatalf("old binding still enabled: %d", enabledFirst)
	}
}
