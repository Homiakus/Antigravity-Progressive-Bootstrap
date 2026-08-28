package antigravity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessprovider "github.com/homiakus/agctl/internal/harness/provider"
)

const maxObservationBytes = 1 << 20

// StatusLineSource returns one official Antigravity status-line JSON payload
// together with the time at which it was observed. The source may be backed by
// a status-line collector, file, socket, or other transport; this package only
// owns provider-specific normalization.
type StatusLineSource interface {
	StatusLine(context.Context) ([]byte, time.Time, error)
}

type StatusLineSourceFunc func(context.Context) ([]byte, time.Time, error)

func (f StatusLineSourceFunc) StatusLine(ctx context.Context) ([]byte, time.Time, error) {
	return f(ctx)
}

// Observation is one coherent provider-neutral view derived from one
// Antigravity status-line payload.
type Observation struct {
	Capacity harnessmodel.ProviderCapacitySnapshot
	Models   []harnessmodel.ProviderModelDescriptor
	Sessions []harnessmodel.ProviderSessionSnapshot
}

// Adapter exposes Antigravity status-line observations through the generic
// provider observation contracts. It intentionally does not execute prompts.
type Adapter struct {
	account harnessmodel.ProviderAccount
	source  StatusLineSource
}

var _ harnessprovider.Adapter = (*Adapter)(nil)

func NewAdapter(account harnessmodel.ProviderAccount, source StatusLineSource) (*Adapter, error) {
	if account.ID == "" || account.Provider != harnessmodel.ProviderAntigravity || !account.State.Valid() {
		return nil, fmt.Errorf("invalid Antigravity provider account")
	}
	if account.CreatedAt.IsZero() || account.UpdatedAt.IsZero() || account.UpdatedAt.Before(account.CreatedAt) {
		return nil, fmt.Errorf("invalid Antigravity provider account timestamps")
	}
	if source == nil {
		return nil, fmt.Errorf("Antigravity status-line source is required")
	}
	return &Adapter{account: account, source: source}, nil
}

func (a *Adapter) Kind() harnessmodel.ProviderKind { return harnessmodel.ProviderAntigravity }

func (a *Adapter) Account() harnessmodel.ProviderAccount { return a.account }

func (a *Adapter) Snapshot(ctx context.Context) (Observation, error) {
	payload, observedAt, err := a.source.StatusLine(ctx)
	if err != nil {
		return Observation{}, fmt.Errorf("read Antigravity status line: %w", err)
	}
	return ParseStatusLine(a.account, payload, observedAt)
}

func (a *Adapter) Capacity(ctx context.Context) (harnessmodel.ProviderCapacitySnapshot, error) {
	obs, err := a.Snapshot(ctx)
	return obs.Capacity, err
}

func (a *Adapter) Models(ctx context.Context) ([]harnessmodel.ProviderModelDescriptor, error) {
	obs, err := a.Snapshot(ctx)
	return obs.Models, err
}

func (a *Adapter) Sessions(ctx context.Context) ([]harnessmodel.ProviderSessionSnapshot, error) {
	obs, err := a.Snapshot(ctx)
	return obs.Sessions, err
}

type statusLinePayload struct {
	SessionID      string                     `json:"session_id"`
	ConversationID string                     `json:"conversation_id"`
	Product        string                     `json:"product"`
	Model          *statusLineModel           `json:"model"`
	Workspace      *statusLineWorkspace       `json:"workspace"`
	ContextWindow  *statusLineContextWindow   `json:"context_window"`
	Quota          map[string]statusLineQuota `json:"quota"`
}

type statusLineModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type statusLineWorkspace struct {
	CurrentDir string `json:"current_dir"`
	ProjectDir string `json:"project_dir"`
}

type statusLineContextWindow struct {
	ContextWindowSize   int64    `json:"context_window_size"`
	UsedPercentage      *float64 `json:"used_percentage"`
	RemainingPercentage *float64 `json:"remaining_percentage"`
}

type statusLineQuota struct {
	RemainingFraction *float64 `json:"remaining_fraction"`
	ResetTime         string   `json:"reset_time"`
}

// ParseStatusLine normalizes the documented Antigravity status-line JSON.
// Unknown JSON fields are deliberately ignored for forward compatibility.
// Sensitive account fields such as email and transcript_path are never copied
// into provider-domain observations.
func ParseStatusLine(account harnessmodel.ProviderAccount, payload []byte, observedAt time.Time) (Observation, error) {
	if account.ID == "" || account.Provider != harnessmodel.ProviderAntigravity || !account.State.Valid() {
		return Observation{}, fmt.Errorf("invalid Antigravity provider account")
	}
	if observedAt.IsZero() {
		return Observation{}, fmt.Errorf("Antigravity observedAt is required")
	}
	if len(payload) == 0 {
		return Observation{}, fmt.Errorf("Antigravity status-line payload is empty")
	}
	if len(payload) > maxObservationBytes {
		return Observation{}, fmt.Errorf("Antigravity status-line payload exceeds %d bytes", maxObservationBytes)
	}

	var raw statusLinePayload
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Observation{}, fmt.Errorf("decode Antigravity status line: %w", err)
	}
	if raw.Product != "" && !strings.EqualFold(raw.Product, "antigravity") {
		return Observation{}, fmt.Errorf("unexpected Antigravity product %q", raw.Product)
	}
	if raw.ConversationID != "" && raw.SessionID != "" && raw.ConversationID != raw.SessionID {
		return Observation{}, fmt.Errorf("Antigravity conversation_id/session_id mismatch")
	}

	obs := Observation{Capacity: harnessmodel.ProviderCapacitySnapshot{
		AccountID:  account.ID,
		Provider:   harnessmodel.ProviderAntigravity,
		Health:     harnessmodel.ProviderHealthUnknown,
		ObservedAt: observedAt.UTC(),
	}}

	modelID := harnessmodel.ProviderModelID("")
	contextLimit := int64(0)
	if raw.ContextWindow != nil {
		if raw.ContextWindow.ContextWindowSize < 0 {
			return Observation{}, fmt.Errorf("Antigravity context_window_size must be non-negative")
		}
		contextLimit = raw.ContextWindow.ContextWindowSize
		if err := validatePercentage("used_percentage", raw.ContextWindow.UsedPercentage); err != nil {
			return Observation{}, err
		}
		if err := validatePercentage("remaining_percentage", raw.ContextWindow.RemainingPercentage); err != nil {
			return Observation{}, err
		}
	}

	if raw.Model != nil {
		id := strings.TrimSpace(raw.Model.ID)
		if id == "" {
			return Observation{}, fmt.Errorf("Antigravity model.id is empty")
		}
		modelID = harnessmodel.ProviderModelID(id)
		display := strings.TrimSpace(raw.Model.DisplayName)
		if display == "" {
			display = id
		}
		obs.Models = []harnessmodel.ProviderModelDescriptor{{
			AccountID:    account.ID,
			Provider:     harnessmodel.ProviderAntigravity,
			ID:           modelID,
			DisplayName:  display,
			ContextLimit: contextLimit,
			Enabled:      true,
		}}
	}

	keys := make([]string, 0, len(raw.Quota))
	for id := range raw.Quota {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	allKnownZero := len(keys) > 0
	for _, id := range keys {
		bucketID := strings.TrimSpace(id)
		if bucketID == "" {
			return Observation{}, fmt.Errorf("Antigravity quota bucket id is empty")
		}
		q := raw.Quota[id]
		window := harnessmodel.QuotaWindow{
			ID:         bucketID,
			Metric:     harnessmodel.QuotaMetricOpaque,
			ObservedAt: observedAt.UTC(),
			Confidence: 0,
		}
		if q.RemainingFraction != nil {
			if *q.RemainingFraction < 0 || *q.RemainingFraction > 1 {
				return Observation{}, fmt.Errorf("Antigravity quota %s remaining_fraction must be within [0,1]", bucketID)
			}
			v := *q.RemainingFraction
			window.Metric = harnessmodel.QuotaMetricFraction
			window.RemainingFraction = &v
			window.Confidence = 1
			if v != 0 {
				allKnownZero = false
			}
		} else {
			allKnownZero = false
		}
		if strings.TrimSpace(q.ResetTime) != "" {
			resetAt, err := time.Parse(time.RFC3339, strings.TrimSpace(q.ResetTime))
			if err != nil {
				return Observation{}, fmt.Errorf("parse Antigravity quota %s reset_time: %w", bucketID, err)
			}
			resetAt = resetAt.UTC()
			window.ResetAt = &resetAt
		}
		// The official payload describes keys as model/bucket IDs. Do not guess
		// that a bucket belongs to the active model; preserve the native ID and
		// leave ModelID unset until provider evidence establishes that mapping.
		obs.Capacity.Windows = append(obs.Capacity.Windows, window)
	}
	if allKnownZero {
		obs.Capacity.Health = harnessmodel.ProviderHealthExhausted
	}

	sessionID := strings.TrimSpace(raw.ConversationID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(raw.SessionID)
	}
	if sessionID != "" && modelID != "" {
		contextUsed := int64(0)
		if contextLimit > 0 && raw.ContextWindow != nil {
			switch {
			case raw.ContextWindow.UsedPercentage != nil:
				contextUsed = percentageOf(contextLimit, *raw.ContextWindow.UsedPercentage)
			case raw.ContextWindow.RemainingPercentage != nil:
				remaining := percentageOf(contextLimit, *raw.ContextWindow.RemainingPercentage)
				contextUsed = contextLimit - remaining
			}
			if contextUsed < 0 {
				contextUsed = 0
			}
			if contextUsed > contextLimit {
				contextUsed = contextLimit
			}
		}
		state := harnessmodel.ProviderSessionActive
		if contextLimit > 0 && contextUsed >= contextLimit {
			state = harnessmodel.ProviderSessionExhausted
		}
		workspace := ""
		if raw.Workspace != nil {
			workspace = strings.TrimSpace(raw.Workspace.ProjectDir)
			if workspace == "" {
				workspace = strings.TrimSpace(raw.Workspace.CurrentDir)
			}
		}
		obs.Sessions = []harnessmodel.ProviderSessionSnapshot{{
			ID:                   harnessmodel.ProviderSessionID(sessionID),
			Provider:             harnessmodel.ProviderAntigravity,
			AccountID:            account.ID,
			ModelID:              modelID,
			State:                state,
			ContextUsed:          contextUsed,
			ContextLimit:         contextLimit,
			LastUsedAt:           observedAt.UTC(),
			WorkspaceFingerprint: fingerprint(workspace),
		}}
	}

	if err := obs.Capacity.Validate(); err != nil {
		return Observation{}, fmt.Errorf("validate normalized Antigravity capacity: %w", err)
	}
	for i := range obs.Sessions {
		if err := obs.Sessions[i].Validate(); err != nil {
			return Observation{}, fmt.Errorf("validate normalized Antigravity session: %w", err)
		}
	}
	return obs, nil
}

func validatePercentage(name string, v *float64) error {
	if v != nil && (*v < 0 || *v > 100 || math.IsNaN(*v) || math.IsInf(*v, 0)) {
		return fmt.Errorf("Antigravity %s must be within [0,100]", name)
	}
	return nil
}

func percentageOf(limit int64, percentage float64) int64 {
	return int64(math.Round(float64(limit) * percentage / 100))
}

func fingerprint(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// HeadlessUsage is the documented terminal usage envelope emitted by
// `agy --output-format json` and the terminal `result` stream-json event.
// It is intentionally not interpreted as quota or current context occupancy:
// the official counters are cumulative over a resumed session.
type HeadlessUsage struct {
	InputTokens     int64
	OutputTokens    int64
	ThinkingTokens  int64
	CacheReadTokens int64
	TotalTokens     int64
}

type HeadlessObservation struct {
	ConversationID string
	Status         string
	Usage          HeadlessUsage
	ObservedAt     time.Time
}

type headlessResult struct {
	ConversationID string `json:"conversation_id"`
	Status         string `json:"status"`
	Usage          struct {
		InputTokens     int64 `json:"input_tokens"`
		OutputTokens    int64 `json:"output_tokens"`
		ThinkingTokens  int64 `json:"thinking_tokens"`
		CacheReadTokens int64 `json:"cache_read_tokens"`
		TotalTokens     int64 `json:"total_tokens"`
	} `json:"usage"`
}

// ParseHeadlessResult accepts either the one-shot JSON result envelope or the
// terminal stream-json {"event":"result","result":...} object. Usage is
// retained losslessly for T-005 settlement/usage persistence; it is not folded
// into capacity or session context because those are different semantics.
func ParseHeadlessResult(payload []byte, observedAt time.Time) (HeadlessObservation, error) {
	if observedAt.IsZero() {
		return HeadlessObservation{}, fmt.Errorf("Antigravity headless observedAt is required")
	}
	if len(payload) == 0 {
		return HeadlessObservation{}, fmt.Errorf("Antigravity headless payload is empty")
	}
	if len(payload) > maxObservationBytes {
		return HeadlessObservation{}, fmt.Errorf("Antigravity headless payload exceeds %d bytes", maxObservationBytes)
	}
	var envelope struct {
		Event  string          `json:"event"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return HeadlessObservation{}, fmt.Errorf("decode Antigravity headless payload: %w", err)
	}
	body := payload
	if envelope.Event != "" {
		if envelope.Event != "result" || len(envelope.Result) == 0 {
			return HeadlessObservation{}, fmt.Errorf("Antigravity headless event %q is not a terminal result", envelope.Event)
		}
		body = envelope.Result
	}
	var result headlessResult
	if err := json.Unmarshal(body, &result); err != nil {
		return HeadlessObservation{}, fmt.Errorf("decode Antigravity headless result: %w", err)
	}
	if strings.TrimSpace(result.Status) == "" {
		return HeadlessObservation{}, fmt.Errorf("Antigravity headless status is required")
	}
	counts := []int64{result.Usage.InputTokens, result.Usage.OutputTokens, result.Usage.ThinkingTokens, result.Usage.CacheReadTokens, result.Usage.TotalTokens}
	for _, count := range counts {
		if count < 0 {
			return HeadlessObservation{}, fmt.Errorf("Antigravity headless token counts must be non-negative")
		}
	}
	return HeadlessObservation{
		ConversationID: strings.TrimSpace(result.ConversationID),
		Status:         strings.TrimSpace(result.Status),
		Usage: HeadlessUsage{
			InputTokens:     result.Usage.InputTokens,
			OutputTokens:    result.Usage.OutputTokens,
			ThinkingTokens:  result.Usage.ThinkingTokens,
			CacheReadTokens: result.Usage.CacheReadTokens,
			TotalTokens:     result.Usage.TotalTokens,
		},
		ObservedAt: observedAt.UTC(),
	}, nil
}
