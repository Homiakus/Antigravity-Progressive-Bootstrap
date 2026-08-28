package model

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type RepositoryID string
type InstanceID string
type WorkspaceID string
type ConversationID string
type RemoteSessionID string
type TelegramBindingID string
type RemoteCommandID string
type RemoteEventID string
type HostID string

type IDKind string

const (
	IDRepository      IDKind = "rep"
	IDRemoteSession   IDKind = "rsi"
	IDTelegramBinding IDKind = "tgb"
	IDRemoteCommand   IDKind = "rcm"
	IDRemoteEvent     IDKind = "rev"
)

var generatedIDRE = regexp.MustCompile(`^[a-z]{3}_[0-9]{13}_[0-9a-f]{20}$`)

type IDGenerator interface {
	New(kind IDKind) (string, error)
}

type TimeSortableIDGenerator struct {
	Now    func() time.Time
	Random io.Reader
}

func NewIDGenerator() TimeSortableIDGenerator {
	return TimeSortableIDGenerator{Now: time.Now, Random: rand.Reader}
}

func (g TimeSortableIDGenerator) New(kind IDKind) (string, error) {
	if !validIDKind(kind) {
		return "", fmt.Errorf("unknown remote id kind %q", kind)
	}
	now := g.Now
	if now == nil {
		now = time.Now
	}
	random := g.Random
	if random == nil {
		random = rand.Reader
	}
	var entropy [10]byte
	if _, err := io.ReadFull(random, entropy[:]); err != nil {
		return "", fmt.Errorf("generate %s id entropy: %w", kind, err)
	}
	return fmt.Sprintf("%s_%013d_%s", kind, now().UTC().UnixMilli(), hex.EncodeToString(entropy[:])), nil
}

func ValidateGeneratedID(id string, kind IDKind) error {
	if !validIDKind(kind) {
		return fmt.Errorf("unknown remote id kind %q", kind)
	}
	if !generatedIDRE.MatchString(id) || !strings.HasPrefix(id, string(kind)+"_") {
		return fmt.Errorf("invalid %s id %q", kind, id)
	}
	return nil
}

func validIDKind(kind IDKind) bool {
	switch kind {
	case IDRepository, IDRemoteSession, IDTelegramBinding, IDRemoteCommand, IDRemoteEvent:
		return true
	default:
		return false
	}
}
