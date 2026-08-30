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

type WorkflowDefinitionID string
type WorkflowRunID string
type NodeID string
type NodeRunID string
type AttemptID string
type WorkerID string
type LeaseID string
type ArtifactID string
type EventID string
type TimerID string
type SignalID string
type ApprovalID string
type EffectIntentID string
type ProviderAccountID string
type ProviderSessionID string
type ProviderAssignmentID string
type ProviderReservationID string
type TaskEnvelopeID string

type IDKind string

const (
	IDWorkflowDefinition  IDKind = "wfd"
	IDWorkflowRun         IDKind = "wfr"
	IDNodeRun             IDKind = "nr"
	IDAttempt             IDKind = "att"
	IDWorker              IDKind = "wrk"
	IDLease               IDKind = "lea"
	IDArtifact            IDKind = "art"
	IDEvent               IDKind = "evt"
	IDTimer               IDKind = "tmr"
	IDSignal              IDKind = "sig"
	IDApproval            IDKind = "apr"
	IDEffectIntent        IDKind = "eff"
	IDProviderAccount     IDKind = "pacc"
	IDProviderSession     IDKind = "pses"
	IDProviderAssignment  IDKind = "pasn"
	IDProviderReservation IDKind = "pres"
	IDTaskEnvelope        IDKind = "tenv"
)

var (
	generatedIDRE = regexp.MustCompile(`^[a-z]{2,4}_[0-9]{13}_[0-9a-f]{20}$`)
	nodeIDRE      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
)

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
		return "", fmt.Errorf("unknown id kind %q", kind)
	}
	now := g.Now
	if now == nil {
		now = time.Now
	}
	r := g.Random
	if r == nil {
		r = rand.Reader
	}
	var entropy [10]byte
	if _, err := io.ReadFull(r, entropy[:]); err != nil {
		return "", fmt.Errorf("generate %s id entropy: %w", kind, err)
	}
	return fmt.Sprintf("%s_%013d_%s", kind, now().UTC().UnixMilli(), hex.EncodeToString(entropy[:])), nil
}

func validIDKind(kind IDKind) bool {
	switch kind {
	case IDWorkflowDefinition, IDWorkflowRun, IDNodeRun, IDAttempt, IDWorker, IDLease, IDArtifact, IDEvent, IDTimer, IDSignal, IDApproval, IDEffectIntent, IDProviderAccount, IDProviderSession, IDProviderAssignment, IDProviderReservation:
		return true
	default:
		return false
	}
}

func ValidateGeneratedID(id string, kind IDKind) error {
	if !validIDKind(kind) {
		return fmt.Errorf("unknown id kind %q", kind)
	}
	if !generatedIDRE.MatchString(id) {
		return fmt.Errorf("invalid %s id %q", kind, id)
	}
	if !strings.HasPrefix(id, string(kind)+"_") {
		return fmt.Errorf("id %q has wrong kind; expected %s", id, kind)
	}
	return nil
}

func ValidateNodeID(id NodeID) error {
	if !nodeIDRE.MatchString(string(id)) {
		return fmt.Errorf("invalid node id %q", id)
	}
	return nil
}
