package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type User struct { ID int64 `json:"id"`; IsBot bool `json:"is_bot"`; FirstName string `json:"first_name,omitempty"`; Username string `json:"username,omitempty"` }
type Chat struct { ID int64 `json:"id"`; Type string `json:"type,omitempty"`; Title string `json:"title,omitempty"` }
type Message struct { MessageID int64 `json:"message_id"`; MessageThreadID int64 `json:"message_thread_id,omitempty"`; From *User `json:"from,omitempty"`; Chat Chat `json:"chat"`; Text string `json:"text,omitempty"` }
type CallbackQuery struct { ID string `json:"id"`; From User `json:"from"`; Message *Message `json:"message,omitempty"`; Data string `json:"data,omitempty"` }
type Update struct { UpdateID int64 `json:"update_id"`; Message *Message `json:"message,omitempty"`; CallbackQuery *CallbackQuery `json:"callback_query,omitempty"` }

type BotAPI interface { GetUpdates(context.Context,int64,time.Duration)([]Update,error); SendMessage(context.Context,int64,int64,string) error; AnswerCallback(context.Context,string,string) error }
type InlineKeyboardButton struct { Text string `json:"text"`; CallbackData string `json:"callback_data"` }
type InlineKeyboardMarkup struct { InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"` }
type View struct { Text string; Keyboard InlineKeyboardMarkup }
type ViewAPI interface { SendView(context.Context,int64,int64,View)(Message,error); EditView(context.Context,int64,int64,View) error; AnswerCallback(context.Context,string,string) error }

type APIOptions struct { BaseURL string; HTTPClient *http.Client }
type APIClient struct { base *url.URL; token string; http *http.Client }

func NewAPIClient(token string,opts APIOptions)(*APIClient,error){token=strings.TrimSpace(token);if token==""{return nil,fmt.Errorf("telegram bot token is required")};baseURL:=strings.TrimSpace(opts.BaseURL);if baseURL==""{baseURL="https://api.telegram.org"};base,err:=url.Parse(baseURL);if err!=nil{return nil,fmt.Errorf("parse Telegram API base URL: %w",err)};if base.Scheme!="https"&&!(base.Scheme=="http"&&isLoopback(base.Hostname())){return nil,fmt.Errorf("Telegram API requires HTTPS except loopback tests")};client:=opts.HTTPClient;if client==nil{client=&http.Client{Timeout:65*time.Second}};return &APIClient{base:base,token:token,http:client},nil}
func isLoopback(host string)bool{if strings.EqualFold(host,"localhost"){return true};ip:=net.ParseIP(host);return ip!=nil&&ip.IsLoopback()}
type apiEnvelope struct { OK bool `json:"ok"`; Result json.RawMessage `json:"result"`; Description string `json:"description,omitempty"`; ErrorCode int `json:"error_code,omitempty"` }
func(c *APIClient)GetUpdates(ctx context.Context,offset int64,timeout time.Duration)([]Update,error){seconds:=int(timeout/time.Second);if seconds<1{seconds=1};if seconds>50{seconds=50};var out []Update;err:=c.call(ctx,"getUpdates",map[string]any{"offset":offset,"timeout":seconds,"limit":100,"allowed_updates":[]string{"message","callback_query"}},&out);return out,err}
func(c *APIClient)SendMessage(ctx context.Context,chatID,threadID int64,text string)error{_,err:=c.SendView(ctx,chatID,threadID,View{Text:text});return err}
func(c *APIClient)SendView(ctx context.Context,chatID,threadID int64,view View)(Message,error){if chatID==0||strings.TrimSpace(view.Text)==""{return Message{},fmt.Errorf("telegram chat and text are required")};body:=map[string]any{"chat_id":chatID,"text":view.Text};if threadID!=0{body["message_thread_id"]=threadID};if len(view.Keyboard.InlineKeyboard)>0{body["reply_markup"]=view.Keyboard};var out Message;err:=c.call(ctx,"sendMessage",body,&out);return out,err}
func(c *APIClient)EditView(ctx context.Context,chatID,messageID int64,view View)error{if chatID==0||messageID==0||strings.TrimSpace(view.Text)==""{return fmt.Errorf("telegram chat, message and text are required")};body:=map[string]any{"chat_id":chatID,"message_id":messageID,"text":view.Text};if len(view.Keyboard.InlineKeyboard)>0{body["reply_markup"]=view.Keyboard};var ignored Message;return c.call(ctx,"editMessageText",body,&ignored)}
func(c *APIClient)AnswerCallback(ctx context.Context,id,text string)error{if strings.TrimSpace(id)==""{return fmt.Errorf("telegram callback id is required")};body:=map[string]any{"callback_query_id":id};if text!=""{body["text"]=text};var ignored bool;return c.call(ctx,"answerCallbackQuery",body,&ignored)}
func(c *APIClient)call(ctx context.Context,method string,body any,out any)error{u:=*c.base;u.Path=path.Join(c.base.Path,"bot"+c.token,method);payload,err:=json.Marshal(body);if err!=nil{return fmt.Errorf("encode Telegram %s request: %w",method,err)};req,err:=http.NewRequestWithContext(ctx,http.MethodPost,u.String(),bytes.NewReader(payload));if err!=nil{return fmt.Errorf("create Telegram %s request",method)};req.Header.Set("Content-Type","application/json");req.Header.Set("Accept","application/json");resp,err:=c.http.Do(req);if err!=nil{return fmt.Errorf("Telegram %s request failed: %s",method,redactToken(err.Error(),c.token))};defer resp.Body.Close();data,err:=io.ReadAll(io.LimitReader(resp.Body,8<<20));if err!=nil{return fmt.Errorf("read Telegram %s response: %w",method,err)};var envelope apiEnvelope;if err:=json.Unmarshal(data,&envelope);err!=nil{return fmt.Errorf("decode Telegram %s response: %w",method,err)};if !envelope.OK||resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("Telegram %s failed (code %d): %s",method,envelope.ErrorCode,envelope.Description)};if out==nil{return nil};if len(envelope.Result)==0{return fmt.Errorf("Telegram %s returned empty result",method)};if err:=json.Unmarshal(envelope.Result,out);err!=nil{return fmt.Errorf("decode Telegram %s result: %w",method,err)};return nil}
func redactToken(message,token string)string{if token==""{return message};return strings.ReplaceAll(message,token,"<redacted>")}
