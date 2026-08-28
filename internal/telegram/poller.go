package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type PollStore interface {
	GetTelegramCursor(context.Context,string)(model.TelegramCursor,error)
	AdvanceTelegramCursor(context.Context,model.TelegramCursor) error
	GetTelegramPrincipal(context.Context,int64)(model.TelegramPrincipal,error)
	GetTelegramBindingByTopic(context.Context,int64,int64)(model.TelegramBinding,error)
	AdmitRemoteCommand(context.Context,model.RemoteCommand)(model.RemoteCommand,bool,error)
	ReserveTelegramCallback(context.Context,string,int64,int64,time.Time)(bool,error)
}

type PollerOptions struct { BotKey string; API BotAPI; Store PollStore; Pairing *PairingService; IDs model.IDGenerator; Now func()time.Time; LongPoll time.Duration; ErrorBackoff time.Duration }
type Poller struct { botKey string; api BotAPI; store PollStore; pairing *PairingService; ids model.IDGenerator; now func()time.Time; longPoll,errorBackoff time.Duration }

func NewPoller(opts PollerOptions)(*Poller,error){if strings.TrimSpace(opts.BotKey)==""||opts.API==nil||opts.Store==nil||opts.Pairing==nil{return nil,fmt.Errorf("telegram bot key, API, store and pairing service are required")};ids:=opts.IDs;if ids==nil{g:=model.NewIDGenerator();ids=g};now:=opts.Now;if now==nil{now=time.Now};lp:=opts.LongPoll;if lp<=0{lp=30*time.Second};backoff:=opts.ErrorBackoff;if backoff<=0{backoff=2*time.Second};return &Poller{botKey:opts.BotKey,api:opts.API,store:opts.Store,pairing:opts.Pairing,ids:ids,now:now,longPoll:lp,errorBackoff:backoff},nil}

func (p *Poller) Run(ctx context.Context)error{for{_,err:=p.PollOnce(ctx);if err!=nil{if ctx.Err()!=nil{return ctx.Err()};timer:=time.NewTimer(p.errorBackoff);select{case<-ctx.Done():timer.Stop();return ctx.Err();case<-timer.C:};continue};if ctx.Err()!=nil{return ctx.Err()}}}

func (p *Poller) PollOnce(ctx context.Context)(int,error){offset:=int64(0);cursor,err:=p.store.GetTelegramCursor(ctx,p.botKey);if err==nil{offset=cursor.NextUpdateID}else if !errors.Is(err,remotestore.ErrNotFound){return 0,err};updates,err:=p.api.GetUpdates(ctx,offset,p.longPoll);if err!=nil{return 0,err};sort.Slice(updates,func(i,j int)bool{return updates[i].UpdateID<updates[j].UpdateID});processed:=0;for _,update:=range updates{if update.UpdateID<offset{continue};if err:=p.processUpdate(ctx,update);err!=nil{return processed,err};if err:=p.store.AdvanceTelegramCursor(ctx,model.TelegramCursor{BotKey:p.botKey,NextUpdateID:update.UpdateID+1,UpdatedAt:p.now().UTC()});err!=nil{return processed,err};processed++;offset=update.UpdateID+1};return processed,nil}

func (p *Poller) processUpdate(ctx context.Context,update Update)error{if update.Message!=nil{return p.processMessage(ctx,*update.Message)};if update.CallbackQuery!=nil{return p.processCallback(ctx,*update.CallbackQuery)};return nil}

func (p *Poller) processMessage(ctx context.Context,message Message)error{if message.From==nil||message.From.ID==0||message.From.IsBot{return nil};if code,ok:=pairCode(message.Text);ok{principal,err:=p.pairing.Consume(ctx,code,message.From.ID,message.Chat.ID);if err!=nil{return err};_ = p.api.SendMessage(ctx,message.Chat.ID,message.MessageThreadID,"Pairing complete. Role: "+string(principal.Role));return nil};principal,authorized,err:=p.authorize(ctx,message.From.ID,model.TelegramRoleOperator);if err!=nil{return err};if !authorized{return nil};_ = principal
	binding,err:=p.store.GetTelegramBindingByTopic(ctx,message.Chat.ID,message.MessageThreadID);if errors.Is(err,remotestore.ErrNotFound){return nil};if err!=nil{return err};text:=strings.TrimSpace(message.Text);if text==""{return nil};payload,_:=json.Marshal(map[string]any{"text":text,"telegramUserId":message.From.ID,"chatId":message.Chat.ID,"threadId":message.MessageThreadID,"messageId":message.MessageID});return p.admit(ctx,binding.SessionID,"conversation.send",fmt.Sprintf("message:%d:%d",message.Chat.ID,message.MessageID),payload)
}

func (p *Poller) processCallback(ctx context.Context,query CallbackQuery)error{if query.From.ID==0||query.From.IsBot||query.Message==nil{return nil};action,sessionID,ok:=parseCallbackData(query.Data);if !ok{_ = p.api.AnswerCallback(ctx,query.ID,"Unsupported action");return nil};required,kind,ok:=callbackAction(action);if !ok{_ = p.api.AnswerCallback(ctx,query.ID,"Unsupported action");return nil};_,authorized,err:=p.authorize(ctx,query.From.ID,required);if err!=nil{return err};if !authorized{_ = p.api.AnswerCallback(ctx,query.ID,"Not authorized");return nil};binding,err:=p.store.GetTelegramBindingByTopic(ctx,query.Message.Chat.ID,query.Message.MessageThreadID);if errors.Is(err,remotestore.ErrNotFound){_ = p.api.AnswerCallback(ctx,query.ID,"Session is not bound");return nil};if err!=nil{return err};if binding.SessionID!=sessionID{_ = p.api.AnswerCallback(ctx,query.ID,"Stale session action");return nil};payload,_:=json.Marshal(map[string]any{"action":action,"telegramUserId":query.From.ID,"chatId":query.Message.Chat.ID,"threadId":query.Message.MessageThreadID});if err:=p.admit(ctx,binding.SessionID,kind,"callback:"+query.ID,payload);err!=nil{return err};if _,err:=p.store.ReserveTelegramCallback(ctx,query.ID,query.From.ID,query.Message.Chat.ID,p.now().UTC());err!=nil{return err};_ = p.api.AnswerCallback(ctx,query.ID,"Queued");return nil}

func (p *Poller) admit(ctx context.Context,sessionID model.RemoteSessionID,kind,sourceID string,payload json.RawMessage)error{id,err:=p.ids.New(model.IDRemoteCommand);if err!=nil{return err};command:=model.RemoteCommand{ID:model.RemoteCommandID(id),Source:"telegram",SourceMessageID:sourceID,SessionID:sessionID,Kind:kind,Payload:payload,State:model.CommandPending,RequestedAt:p.now().UTC()};_,_,err=p.store.AdmitRemoteCommand(ctx,command);return err}
func (p *Poller) authorize(ctx context.Context,userID int64,required model.TelegramRole)(model.TelegramPrincipal,bool,error){principal,err:=p.store.GetTelegramPrincipal(ctx,userID);if errors.Is(err,remotestore.ErrNotFound){return model.TelegramPrincipal{},false,nil};if err!=nil{return model.TelegramPrincipal{},false,err};if !principal.Enabled{return principal,false,nil};return principal,roleRank(principal.Role)>=roleRank(required),nil}
func roleRank(role model.TelegramRole)int{switch role{case model.TelegramRoleOwner:return 3;case model.TelegramRoleOperator:return 2;case model.TelegramRoleViewer:return 1;default:return 0}}
func pairCode(text string)(string,bool){fields:=strings.Fields(strings.TrimSpace(text));if len(fields)!=2{return "",false};command:=strings.SplitN(fields[0],"@",2)[0];if !strings.EqualFold(command,"/pair"){return "",false};return fields[1],true}
func parseCallbackData(data string)(string,model.RemoteSessionID,bool){parts:=strings.Split(data,"|");if len(parts)!=3||parts[0]!="r1"||parts[1]==""||parts[2]==""{return "","",false};return parts[1],model.RemoteSessionID(parts[2]),true}
func callbackAction(action string)(model.TelegramRole,string,bool){switch action{case "pause":return model.TelegramRoleOperator,"session.pause",true;case "resume":return model.TelegramRoleOperator,"session.resume",true;case "cancel":return model.TelegramRoleOperator,"conversation.cancel",true;case "close":return model.TelegramRoleOwner,"session.close",true;default:return "","",false}}
