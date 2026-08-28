package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type fakePairStore struct{ pairing model.TelegramPairing; principal model.TelegramPrincipal; consumed bool }
func(f *fakePairStore)CreateTelegramPairing(_ context.Context,p model.TelegramPairing)error{f.pairing=p;return nil}
func(f *fakePairStore)ConsumeTelegramPairing(_ context.Context,hash string,user,chat int64,now time.Time)(model.TelegramPrincipal,error){if hash!=f.pairing.TokenHash{return model.TelegramPrincipal{},remotestore.ErrNotFound};if f.consumed{return f.principal,nil};f.consumed=true;f.principal=model.TelegramPrincipal{UserID:user,Role:f.pairing.Role,Enabled:true,PairedAt:now};return f.principal,nil}
func TestPairingStoresHashNotRawCode(t *testing.T){store:=&fakePairStore{};now:=time.Unix(1000,0).UTC();svc,_:=NewPairingService(store,bytes.NewReader(make([]byte,10)),func()time.Time{return now});code,err:=svc.Create(context.Background(),model.TelegramRoleOwner,99,time.Minute);if err!=nil{t.Fatal(err)};if code==""||store.pairing.TokenHash==code{t.Fatalf("code/hash not separated: %q %q",code,store.pairing.TokenHash)};if _,err:=svc.Consume(context.Background(),code,42,99);err!=nil{t.Fatal(err)}}

type fakeAPI struct{ updates []Update; offsets []int64; answers []string }
func(f *fakeAPI)GetUpdates(_ context.Context,offset int64,_ time.Duration)([]Update,error){f.offsets=append(f.offsets,offset);return f.updates,nil}
func(f *fakeAPI)SendMessage(context.Context,int64,int64,string)error{return nil}
func(f *fakeAPI)AnswerCallback(_ context.Context,id,text string)error{f.answers=append(f.answers,id+":"+text);return nil}

type fakePollStore struct{ cursor model.TelegramCursor; cursorExists bool; cursorFailOnce bool; principal model.TelegramPrincipal; binding model.TelegramBinding; commands map[string]model.RemoteCommand; callbacks map[string]bool }
func(f *fakePollStore)GetTelegramCursor(context.Context,string)(model.TelegramCursor,error){if !f.cursorExists{return model.TelegramCursor{},remotestore.ErrNotFound};return f.cursor,nil}
func(f *fakePollStore)AdvanceTelegramCursor(_ context.Context,c model.TelegramCursor)error{if f.cursorFailOnce{f.cursorFailOnce=false;return errors.New("crash-before-cursor")};f.cursor=c;f.cursorExists=true;return nil}
func(f *fakePollStore)GetTelegramPrincipal(context.Context,int64)(model.TelegramPrincipal,error){if f.principal.UserID==0{return model.TelegramPrincipal{},remotestore.ErrNotFound};return f.principal,nil}
func(f *fakePollStore)GetTelegramBindingByTopic(context.Context,int64,int64)(model.TelegramBinding,error){if f.binding.ID==""{return model.TelegramBinding{},remotestore.ErrNotFound};return f.binding,nil}
func(f *fakePollStore)AdmitRemoteCommand(_ context.Context,c model.RemoteCommand)(model.RemoteCommand,bool,error){if f.commands==nil{f.commands=map[string]model.RemoteCommand{}};if existing,ok:=f.commands[c.SourceMessageID];ok{return existing,false,nil};f.commands[c.SourceMessageID]=c;return c,true,nil}
func(f *fakePollStore)ReserveTelegramCallback(_ context.Context,id string,_ int64,_ int64,_ time.Time)(bool,error){if f.callbacks==nil{f.callbacks=map[string]bool{}};if f.callbacks[id]{return false,nil};f.callbacks[id]=true;return true,nil}

type staticIDs struct{}
func(*staticIDs)New(model.IDKind)(string,error){return "rcm_1700000000000_00000000000000000001",nil}

func TestPollerReplayAfterCursorFailureDoesNotDuplicateCommand(t *testing.T){now:=time.Unix(2000,0).UTC();api:=&fakeAPI{updates:[]Update{{UpdateID:77,Message:&Message{MessageID:5,From:&User{ID:42},Chat:Chat{ID:99},Text:"hello"}}}};store:=&fakePollStore{cursorFailOnce:true,principal:model.TelegramPrincipal{UserID:42,Role:model.TelegramRoleOperator,Enabled:true,PairedAt:now},binding:model.TelegramBinding{ID:"tgb_1700000000000_00000000000000000001",SessionID:"rsi_1700000000000_00000000000000000001",ChatID:99,OwnerUserID:42,Enabled:true,CreatedAt:now}};pair,_:=NewPairingService(&fakePairStore{},bytes.NewReader(make([]byte,20)),func()time.Time{return now});poller,err:=NewPoller(PollerOptions{BotKey:"primary",API:api,Store:store,Pairing:pair,IDs:&staticIDs{},Now:func()time.Time{return now}});if err!=nil{t.Fatal(err)};if _,err:=poller.PollOnce(context.Background());err==nil{t.Fatal("expected cursor failure")};if len(store.commands)!=1{t.Fatalf("commands=%d",len(store.commands))};if _,err:=poller.PollOnce(context.Background());err!=nil{t.Fatal(err)};if len(store.commands)!=1{t.Fatalf("duplicate commands=%d",len(store.commands))};if store.cursor.NextUpdateID!=78{t.Fatalf("cursor=%d",store.cursor.NextUpdateID)}}

func TestCallbackSessionBindingAndRoleGuard(t *testing.T){now:=time.Unix(2000,0).UTC();sid:=model.RemoteSessionID("rsi_1700000000000_00000000000000000001");api:=&fakeAPI{updates:[]Update{{UpdateID:9,CallbackQuery:&CallbackQuery{ID:"cb1",From:User{ID:42},Message:&Message{Chat:Chat{ID:99}},Data:"r1|close|"+string(sid)}}}};store:=&fakePollStore{principal:model.TelegramPrincipal{UserID:42,Role:model.TelegramRoleOperator,Enabled:true,PairedAt:now},binding:model.TelegramBinding{ID:"tgb_1700000000000_00000000000000000001",SessionID:sid,ChatID:99,OwnerUserID:42,Enabled:true,CreatedAt:now}};pair,_:=NewPairingService(&fakePairStore{},bytes.NewReader(make([]byte,20)),func()time.Time{return now});poller,_:=NewPoller(PollerOptions{BotKey:"primary",API:api,Store:store,Pairing:pair,IDs:&staticIDs{},Now:func()time.Time{return now}});if _,err:=poller.PollOnce(context.Background());err!=nil{t.Fatal(err)};if len(store.commands)!=0{payload,_:=json.Marshal(store.commands);t.Fatalf("operator admitted owner close: %s",payload)}}
