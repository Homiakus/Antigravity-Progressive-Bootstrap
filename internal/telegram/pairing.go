package telegram

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/remote/model"
)

type PairingStore interface { CreateTelegramPairing(context.Context,model.TelegramPairing) error; ConsumeTelegramPairing(context.Context,string,int64,int64,time.Time)(model.TelegramPrincipal,error) }
type PairingService struct { store PairingStore; random io.Reader; now func()time.Time }
func NewPairingService(store PairingStore,random io.Reader,now func()time.Time)(*PairingService,error){if store==nil{return nil,fmt.Errorf("telegram pairing store is required")};if random==nil{random=rand.Reader};if now==nil{now=time.Now};return &PairingService{store:store,random:random,now:now},nil}
func (s *PairingService) Create(ctx context.Context,role model.TelegramRole,chatID int64,ttl time.Duration)(string,error){if !role.Valid(){return "",fmt.Errorf("invalid telegram pairing role %q",role)};if ttl<=0{ttl=10*time.Minute};if ttl>time.Hour{return "",fmt.Errorf("telegram pairing TTL exceeds one hour")};entropy:=make([]byte,10);if _,err:=io.ReadFull(s.random,entropy);err!=nil{return "",err};code:=base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(entropy);now:=s.now().UTC();pairing:=model.TelegramPairing{TokenHash:pairingHash(code),Role:role,IntendedChatID:chatID,CreatedAt:now,ExpiresAt:now.Add(ttl)};if err:=s.store.CreateTelegramPairing(ctx,pairing);err!=nil{return "",err};return code,nil}
func (s *PairingService) Consume(ctx context.Context,code string,userID,chatID int64)(model.TelegramPrincipal,error){code=strings.ToUpper(strings.TrimSpace(code));if code==""{return model.TelegramPrincipal{},fmt.Errorf("telegram pairing code is required")};return s.store.ConsumeTelegramPairing(ctx,pairingHash(code),userID,chatID,s.now().UTC())}
func pairingHash(code string)string{sum:=sha256.Sum256([]byte(strings.ToUpper(strings.TrimSpace(code))));return hex.EncodeToString(sum[:])}
