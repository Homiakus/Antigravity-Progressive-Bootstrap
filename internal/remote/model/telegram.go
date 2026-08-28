package model

import (
	"fmt"
	"strings"
	"time"
)

type TelegramRole string

const (
	TelegramRoleOwner    TelegramRole = "OWNER"
	TelegramRoleOperator TelegramRole = "OPERATOR"
	TelegramRoleViewer   TelegramRole = "VIEWER"
)

func (r TelegramRole) Valid() bool {
	return r == TelegramRoleOwner || r == TelegramRoleOperator || r == TelegramRoleViewer
}

type TelegramCursor struct {
	BotKey       string
	NextUpdateID int64
	UpdatedAt    time.Time
}

func (c TelegramCursor) Validate() error {
	if strings.TrimSpace(c.BotKey) == "" { return fmt.Errorf("telegram bot key is required") }
	if c.NextUpdateID < 0 { return fmt.Errorf("telegram next update id cannot be negative") }
	if c.UpdatedAt.IsZero() { return fmt.Errorf("telegram cursor updated_at is required") }
	return nil
}

type TelegramPrincipal struct {
	UserID   int64
	Role     TelegramRole
	Enabled  bool
	PairedAt time.Time
}

func (p TelegramPrincipal) Validate() error {
	if p.UserID == 0 { return fmt.Errorf("telegram principal user id is required") }
	if !p.Role.Valid() { return fmt.Errorf("invalid telegram role %q", p.Role) }
	if p.PairedAt.IsZero() { return fmt.Errorf("telegram principal paired_at is required") }
	return nil
}

type TelegramPairing struct {
	TokenHash        string
	Role             TelegramRole
	IntendedChatID   int64
	CreatedAt        time.Time
	ExpiresAt        time.Time
	ConsumedAt       *time.Time
	ConsumedByUserID int64
}

func (p TelegramPairing) Validate() error {
	if strings.TrimSpace(p.TokenHash) == "" { return fmt.Errorf("telegram pairing token hash is required") }
	if !p.Role.Valid() { return fmt.Errorf("invalid telegram pairing role %q", p.Role) }
	if p.CreatedAt.IsZero() || p.ExpiresAt.IsZero() || !p.ExpiresAt.After(p.CreatedAt) { return fmt.Errorf("invalid telegram pairing lifetime") }
	return nil
}
