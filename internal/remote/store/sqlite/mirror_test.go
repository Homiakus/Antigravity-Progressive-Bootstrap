package sqlite

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
)

func TestAppendRemoteEventAndTelegramOutboxAreAtomicAndDeduped(t *testing.T) {
	ctx, store, done := openRemoteTestStore(t)
	defer done()
	seedRemoteGraph(t, ctx, store)
	now := time.Unix(2000, 0).UTC()
	event := model.RemoteEvent{ID:"rev_1700000000000_00000000000000000011",SessionID:"rsi_1700000000000_00000000000000000001",Source:model.EventSourceIDE,Type:model.EventAgentMessage,SourceEventID:"bridge:42",Payload:json.RawMessage(`{"text":"done"}`),Timestamp:now}
	got, created, err := store.AppendRemoteEventWithOutbox(ctx,event,"telegram",[]byte(`{"render":true}`))
	if err != nil || !created || got.Seq != 1 { t.Fatalf("first=%#v created=%v err=%v",got,created,err) }
	duplicate := event
	duplicate.ID = "rev_1700000000000_00000000000000000012"
	got, created, err = store.AppendRemoteEventWithOutbox(ctx,duplicate,"telegram",[]byte(`{"render":true}`))
	if err != nil || created || got.ID != event.ID { t.Fatalf("duplicate=%#v created=%v err=%v",got,created,err) }
	items, err := store.ListRemoteOutbox(ctx,"telegram",now.Add(time.Second),10)
	if err != nil { t.Fatal(err) }
	if len(items) != 1 || items[0].EventID != event.ID { t.Fatalf("outbox=%#v",items) }
}

func TestTelegramMirrorStateNeverMovesBackward(t *testing.T) {
	ctx, store, done := openRemoteTestStore(t)
	defer done()
	seedRemoteGraph(t, ctx, store)
	now:=time.Unix(2000,0).UTC()
	first:=model.TelegramMirrorState{SessionID:"rsi_1700000000000_00000000000000000001",ChatID:99,MessageID:7,LastEventSeq:5,RenderedText:"new",UpdatedAt:now}
	if err:=store.UpsertTelegramMirrorState(ctx,first);err!=nil{t.Fatal(err)}	
	stale:=first;stale.LastEventSeq=3;stale.RenderedText="old";stale.UpdatedAt=now.Add(time.Second)
	if err:=store.UpsertTelegramMirrorState(ctx,stale);err!=nil{t.Fatal(err)}
	got,err:=store.GetTelegramMirrorState(ctx,first.SessionID);if err!=nil{t.Fatal(err)}
	if got.LastEventSeq!=5||got.RenderedText!="new"{t.Fatalf("mirror=%#v",got)}
}

func TestRemoteOutboxRetryThenDelivery(t *testing.T) {
	ctx, store, done := openRemoteTestStore(t)
	defer done()
	seedRemoteGraph(t, ctx, store)
	now:=time.Unix(2000,0).UTC()
	event:=model.RemoteEvent{ID:"rev_1700000000000_00000000000000000021",SessionID:"rsi_1700000000000_00000000000000000001",Source:model.EventSourceIDE,Type:model.EventAgentMessage,SourceEventID:"bridge:retry",Payload:json.RawMessage(`{}`),Timestamp:now}
	if _,_,err:=store.AppendRemoteEventWithOutbox(ctx,event,"telegram",[]byte(`{}`));err!=nil{t.Fatal(err)}
	items,err:=store.ListRemoteOutbox(ctx,"telegram",now,10);if err!=nil||len(items)!=1{t.Fatalf("items=%#v err=%v",items,err)}
	if err:=store.ScheduleRemoteOutboxRetry(ctx,items[0].ID,now.Add(time.Minute));err!=nil{t.Fatal(err)}
	items,err=store.ListRemoteOutbox(ctx,"telegram",now.Add(30*time.Second),10);if err!=nil||len(items)!=0{t.Fatalf("premature retry=%#v err=%v",items,err)}
	items,err=store.ListRemoteOutbox(ctx,"telegram",now.Add(2*time.Minute),10);if err!=nil||len(items)!=1||items[0].AttemptCount!=1{t.Fatalf("retry=%#v err=%v",items,err)}
	if err:=store.MarkRemoteOutboxDelivered(ctx,items[0].ID,now.Add(2*time.Minute));err!=nil{t.Fatal(err)}
	items,err=store.ListRemoteOutbox(ctx,"telegram",now.Add(3*time.Minute),10);if err!=nil||len(items)!=0{t.Fatalf("delivered still pending=%#v err=%v",items,err)}
}

var _ = context.Background
