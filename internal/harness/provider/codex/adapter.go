package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessprovider "github.com/homiakus/agctl/internal/harness/provider"
)

const (
	maxProtocolBytes = 1 << 20
	maxNativeIDBytes = 256
	legacyBucketID   = "legacy"
)

var (
	ErrRateLimitsNotObserved = errors.New("Codex rate limits have not been observed")
	ErrStaleObservation      = errors.New("Codex observation is older than current state")
)

// Adapter is the provider-neutral observation state fed by Codex App Server
// JSON-RPC responses and notifications. Transport/process lifecycle is
// intentionally outside this package; execution remains owned by the existing
// runtime until TaskEnvelope work defines a portable execution contract.
type Adapter struct {
	account harnessmodel.ProviderAccount

	mu               sync.RWMutex
	rateBuckets      map[string]rateLimitSnapshot
	rateObservedAt   time.Time
	models           map[harnessmodel.ProviderModelID]harnessmodel.ProviderModelDescriptor
	modelsObservedAt time.Time
}

var _ harnessprovider.Adapter = (*Adapter)(nil)
var _ harnessprovider.SnapshotSource = (*Adapter)(nil)

func NewAdapter(account harnessmodel.ProviderAccount) (*Adapter, error) {
	if account.ID == "" || account.Provider != harnessmodel.ProviderCodex || !account.State.Valid() {
		return nil, fmt.Errorf("invalid Codex provider account")
	}
	if account.CreatedAt.IsZero() || account.UpdatedAt.IsZero() || account.UpdatedAt.Before(account.CreatedAt) {
		return nil, fmt.Errorf("invalid Codex provider account timestamps")
	}
	return &Adapter{
		account:     account,
		rateBuckets: make(map[string]rateLimitSnapshot),
		models:      make(map[harnessmodel.ProviderModelID]harnessmodel.ProviderModelDescriptor),
	}, nil
}

func (a *Adapter) Kind() harnessmodel.ProviderKind { return harnessmodel.ProviderCodex }
func (a *Adapter) Account() harnessmodel.ProviderAccount { return a.account }

// ApplyRateLimitsRead replaces the rate-limit baseline from
// account/rateLimits/read. It accepts either the method result itself or a
// JSON-RPC response envelope containing that result.
func (a *Adapter) ApplyRateLimitsRead(payload []byte, observedAt time.Time) error {
	if observedAt.IsZero() {
		return fmt.Errorf("Codex rate-limit observedAt is required")
	}
	var response getAccountRateLimitsResponse
	if err := decodeResult(payload, &response); err != nil {
		return fmt.Errorf("decode Codex account/rateLimits/read: %w", err)
	}
	if response.RateLimits == nil {
		return fmt.Errorf("Codex account/rateLimits/read missing rateLimits")
	}

	buckets := make(map[string]rateLimitSnapshot)
	if len(response.RateLimitsByLimitID) > 0 {
		keys := make([]string, 0, len(response.RateLimitsByLimitID))
		for key := range response.RateLimitsByLimitID {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			snapshot := response.RateLimitsByLimitID[key]
			if snapshot == nil {
				return fmt.Errorf("Codex rate-limit bucket %q is null", key)
			}
			id, err := resolveBucketID(key, snapshot.LimitID)
			if err != nil {
				return err
			}
			if err := validateRateLimitSnapshot(id, *snapshot); err != nil {
				return err
			}
			if _, exists := buckets[id]; exists {
				return fmt.Errorf("duplicate Codex rate-limit bucket %q", id)
			}
			buckets[id] = cloneRateLimitSnapshot(*snapshot)
		}
	} else {
		id, err := resolveBucketID("", response.RateLimits.LimitID)
		if err != nil {
			return err
		}
		if err := validateRateLimitSnapshot(id, *response.RateLimits); err != nil {
			return err
		}
		buckets[id] = cloneRateLimitSnapshot(*response.RateLimits)
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.rateObservedAt.IsZero() && observedAt.Before(a.rateObservedAt) {
		return ErrStaleObservation
	}
	a.rateBuckets = buckets
	a.rateObservedAt = observedAt.UTC()
	return nil
}

// ApplyRateLimitsUpdated merges account/rateLimits/updated. The App Server
// contract explicitly defines this notification as sparse: nil/unavailable
// metadata does not clear previously observed values.
func (a *Adapter) ApplyRateLimitsUpdated(payload []byte, observedAt time.Time) error {
	if observedAt.IsZero() {
		return fmt.Errorf("Codex rate-limit observedAt is required")
	}
	var notification accountRateLimitsUpdatedNotification
	if err := decodeNotification(payload, "account/rateLimits/updated", &notification); err != nil {
		return fmt.Errorf("decode Codex account/rateLimits/updated: %w", err)
	}
	if notification.RateLimits == nil {
		return fmt.Errorf("Codex account/rateLimits/updated missing rateLimits")
	}
	if err := validateRateLimitSnapshot("update", *notification.RateLimits); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.rateObservedAt.IsZero() && observedAt.Before(a.rateObservedAt) {
		return ErrStaleObservation
	}
	if len(a.rateBuckets) == 0 {
		return ErrRateLimitsNotObserved
	}

	id, err := a.resolveUpdateBucketLocked(notification.RateLimits.LimitID)
	if err != nil {
		return err
	}
	current, exists := a.rateBuckets[id]
	if !exists {
		current = rateLimitSnapshot{}
	}
	merged := mergeSparseRateLimit(current, *notification.RateLimits)
	if err := validateRateLimitSnapshot(id, merged); err != nil {
		return err
	}
	a.rateBuckets[id] = merged
	a.rateObservedAt = observedAt.UTC()
	return nil
}

func (a *Adapter) resolveUpdateBucketLocked(limitID *string) (string, error) {
	if id := cleanOptional(limitID); id != "" {
		if len(id) > maxNativeIDBytes {
			return "", fmt.Errorf("Codex limit id exceeds %d bytes", maxNativeIDBytes)
		}
		return id, nil
	}
	if _, ok := a.rateBuckets[legacyBucketID]; ok {
		return legacyBucketID, nil
	}
	if len(a.rateBuckets) == 1 {
		for id := range a.rateBuckets {
			return id, nil
		}
	}
	return "", fmt.Errorf("Codex sparse rate-limit update has no limitId and baseline has %d buckets", len(a.rateBuckets))
}

// ReplaceModelCatalog atomically replaces the dynamic model catalog from one
// complete model/list pagination cycle. Each page may be either a result body
// or a JSON-RPC response. The final page must have nextCursor=null so a partial
// catalog cannot silently replace a complete one.
func (a *Adapter) ReplaceModelCatalog(pages [][]byte, observedAt time.Time) error {
	if observedAt.IsZero() {
		return fmt.Errorf("Codex model catalog observedAt is required")
	}
	if len(pages) == 0 {
		return fmt.Errorf("Codex model catalog requires at least one page")
	}
	models := make(map[harnessmodel.ProviderModelID]harnessmodel.ProviderModelDescriptor)
	for i, payload := range pages {
		var page modelListResponse
		if err := decodeResult(payload, &page); err != nil {
			return fmt.Errorf("decode Codex model/list page %d: %w", i, err)
		}
		if i < len(pages)-1 && page.NextCursor == nil {
			return fmt.Errorf("Codex model/list page %d ended pagination before supplied final page", i)
		}
		if i == len(pages)-1 && page.NextCursor != nil {
			return fmt.Errorf("Codex model/list catalog is incomplete; nextCursor=%q", *page.NextCursor)
		}
		for _, raw := range page.Data {
			descriptor, err := normalizeModel(a.account.ID, raw)
			if err != nil {
				return err
			}
			if _, exists := models[descriptor.ID]; exists {
				return fmt.Errorf("duplicate Codex model id %q across catalog pages", descriptor.ID)
			}
			models[descriptor.ID] = descriptor
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.modelsObservedAt.IsZero() && observedAt.Before(a.modelsObservedAt) {
		return ErrStaleObservation
	}
	a.models = models
	a.modelsObservedAt = observedAt.UTC()
	return nil
}

func (a *Adapter) Observe(context.Context) (harnessprovider.Observation, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	capacity, err := a.capacityLocked()
	if err != nil {
		return harnessprovider.Observation{}, err
	}
	return harnessprovider.Observation{
		Capacity: capacity,
		Models:   a.modelsLocked(),
		// Codex thread/list + thread/tokenUsage/updated do not expose the
		// selected model id. ProviderSessionSnapshot requires ModelID, so the
		// adapter fails closed rather than inventing a thread->model binding.
		Sessions: nil,
	}, nil
}

func (a *Adapter) Capacity(context.Context) (harnessmodel.ProviderCapacitySnapshot, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.capacityLocked()
}

func (a *Adapter) Models(context.Context) ([]harnessmodel.ProviderModelDescriptor, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.modelsLocked(), nil
}

func (a *Adapter) Sessions(context.Context) ([]harnessmodel.ProviderSessionSnapshot, error) {
	return nil, nil
}

func (a *Adapter) modelsLocked() []harnessmodel.ProviderModelDescriptor {
	ids := make([]string, 0, len(a.models))
	for id := range a.models {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	out := make([]harnessmodel.ProviderModelDescriptor, 0, len(ids))
	for _, id := range ids {
		model := a.models[harnessmodel.ProviderModelID(id)]
		model.Capabilities = append([]string(nil), model.Capabilities...)
		out = append(out, model)
	}
	return out
}

func (a *Adapter) capacityLocked() (harnessmodel.ProviderCapacitySnapshot, error) {
	if a.rateObservedAt.IsZero() {
		return harnessmodel.ProviderCapacitySnapshot{}, ErrRateLimitsNotObserved
	}
	capacity := harnessmodel.ProviderCapacitySnapshot{
		AccountID:  a.account.ID,
		Provider:   harnessmodel.ProviderCodex,
		Health:     harnessmodel.ProviderHealthUnknown,
		ObservedAt: a.rateObservedAt,
	}
	ids := make([]string, 0, len(a.rateBuckets))
	for id := range a.rateBuckets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	allBucketsExhausted := len(ids) > 0
	for _, id := range ids {
		snapshot := a.rateBuckets[id]
		windows := 0
		allWindowsZero := true
		if snapshot.Primary != nil {
			window := normalizeWindow(id+"/primary", *snapshot.Primary, a.rateObservedAt)
			capacity.Windows = append(capacity.Windows, window)
			windows++
			if window.RemainingFraction == nil || *window.RemainingFraction != 0 {
				allWindowsZero = false
			}
		}
		if snapshot.Secondary != nil {
			window := normalizeWindow(id+"/secondary", *snapshot.Secondary, a.rateObservedAt)
			capacity.Windows = append(capacity.Windows, window)
			windows++
			if window.RemainingFraction == nil || *window.RemainingFraction != 0 {
				allWindowsZero = false
			}
		}
		bucketExhausted := explicitExhausted(snapshot) || (windows > 0 && allWindowsZero)
		if !bucketExhausted {
			allBucketsExhausted = false
		}
	}
	if allBucketsExhausted {
		capacity.Health = harnessmodel.ProviderHealthExhausted
	}
	if err := capacity.Validate(); err != nil {
		return harnessmodel.ProviderCapacitySnapshot{}, fmt.Errorf("validate normalized Codex capacity: %w", err)
	}
	return capacity, nil
}

func normalizeWindow(id string, raw rateLimitWindow, observedAt time.Time) harnessmodel.QuotaWindow {
	remaining := (100 - raw.UsedPercent) / 100
	window := harnessmodel.QuotaWindow{
		ID:                id,
		Metric:            harnessmodel.QuotaMetricFraction,
		RemainingFraction: &remaining,
		ObservedAt:        observedAt,
		Confidence:        1,
	}
	if raw.ResetsAt != nil {
		reset := time.Unix(*raw.ResetsAt, 0).UTC()
		window.ResetAt = &reset
	}
	return window
}

func explicitExhausted(snapshot rateLimitSnapshot) bool {
	if snapshot.SpendControlReached != nil && *snapshot.SpendControlReached {
		return true
	}
	return cleanOptional(snapshot.RateLimitReachedType) != ""
}

func resolveBucketID(mapKey string, limitID *string) (string, error) {
	key := strings.TrimSpace(mapKey)
	id := cleanOptional(limitID)
	if len(key) > maxNativeIDBytes || len(id) > maxNativeIDBytes {
		return "", fmt.Errorf("Codex limit id exceeds %d bytes", maxNativeIDBytes)
	}
	if key != "" && id != "" && key != id {
		return "", fmt.Errorf("Codex rate-limit map key %q conflicts with limitId %q", key, id)
	}
	if key != "" {
		return key, nil
	}
	if id != "" {
		return id, nil
	}
	return legacyBucketID, nil
}

func validateRateLimitSnapshot(id string, snapshot rateLimitSnapshot) error {
	if snapshot.Primary != nil {
		if err := validateRateLimitWindow(id+"/primary", *snapshot.Primary); err != nil {
			return err
		}
	}
	if snapshot.Secondary != nil {
		if err := validateRateLimitWindow(id+"/secondary", *snapshot.Secondary); err != nil {
			return err
		}
	}
	return nil
}

func validateRateLimitWindow(id string, window rateLimitWindow) error {
	if math.IsNaN(window.UsedPercent) || math.IsInf(window.UsedPercent, 0) || window.UsedPercent < 0 || window.UsedPercent > 100 {
		return fmt.Errorf("Codex rate-limit %s usedPercent must be within [0,100]", id)
	}
	if window.WindowDurationMins != nil && *window.WindowDurationMins < 0 {
		return fmt.Errorf("Codex rate-limit %s windowDurationMins must be non-negative", id)
	}
	if window.ResetsAt != nil && *window.ResetsAt < 0 {
		return fmt.Errorf("Codex rate-limit %s resetsAt must be non-negative", id)
	}
	return nil
}

func mergeSparseRateLimit(current, update rateLimitSnapshot) rateLimitSnapshot {
	out := cloneRateLimitSnapshot(current)
	if update.LimitID != nil {
		out.LimitID = cloneString(update.LimitID)
	}
	if update.LimitName != nil {
		out.LimitName = cloneString(update.LimitName)
	}
	if update.Primary != nil {
		v := *update.Primary
		out.Primary = &v
	}
	if update.Secondary != nil {
		v := *update.Secondary
		out.Secondary = &v
	}
	if update.SpendControlReached != nil {
		v := *update.SpendControlReached
		out.SpendControlReached = &v
	}
	if update.RateLimitReachedType != nil {
		out.RateLimitReachedType = cloneString(update.RateLimitReachedType)
	}
	return out
}

func cloneRateLimitSnapshot(in rateLimitSnapshot) rateLimitSnapshot {
	out := in
	out.LimitID = cloneString(in.LimitID)
	out.LimitName = cloneString(in.LimitName)
	out.RateLimitReachedType = cloneString(in.RateLimitReachedType)
	if in.Primary != nil {
		v := *in.Primary
		out.Primary = &v
	}
	if in.Secondary != nil {
		v := *in.Secondary
		out.Secondary = &v
	}
	if in.SpendControlReached != nil {
		v := *in.SpendControlReached
		out.SpendControlReached = &v
	}
	return out
}

func cloneString(v *string) *string {
	if v == nil {
		return nil
	}
	x := *v
	return &x
}

func cleanOptional(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

type getAccountRateLimitsResponse struct {
	RateLimits          *rateLimitSnapshot            `json:"rateLimits"`
	RateLimitsByLimitID map[string]*rateLimitSnapshot `json:"rateLimitsByLimitId"`
}

type accountRateLimitsUpdatedNotification struct {
	RateLimits *rateLimitSnapshot `json:"rateLimits"`
}

type rateLimitSnapshot struct {
	LimitID                *string          `json:"limitId"`
	LimitName              *string          `json:"limitName"`
	Primary                *rateLimitWindow `json:"primary"`
	Secondary              *rateLimitWindow `json:"secondary"`
	SpendControlReached     *bool            `json:"spendControlReached"`
	RateLimitReachedType    *string          `json:"rateLimitReachedType"`
}

type rateLimitWindow struct {
	UsedPercent        float64 `json:"usedPercent"`
	WindowDurationMins *int64  `json:"windowDurationMins"`
	ResetsAt           *int64  `json:"resetsAt"`
}

type modelListResponse struct {
	Data       []modelPayload `json:"data"`
	NextCursor *string        `json:"nextCursor"`
}

type modelPayload struct {
	ID                    string          `json:"id"`
	Model                 string          `json:"model"`
	DisplayName           string          `json:"displayName"`
	Hidden                bool            `json:"hidden"`
	InputModalities       []string        `json:"inputModalities"`
	SupportsPersonality   bool            `json:"supportsPersonality"`
	MultiAgentVersion     json.RawMessage `json:"multiAgentVersion"`
}

func normalizeModel(accountID harnessmodel.ProviderAccountID, raw modelPayload) (harnessmodel.ProviderModelDescriptor, error) {
	id := strings.TrimSpace(raw.ID)
	if id == "" {
		return harnessmodel.ProviderModelDescriptor{}, fmt.Errorf("Codex model id is required")
	}
	if len(id) > maxNativeIDBytes {
		return harnessmodel.ProviderModelDescriptor{}, fmt.Errorf("Codex model id exceeds %d bytes", maxNativeIDBytes)
	}
	display := strings.TrimSpace(raw.DisplayName)
	if display == "" {
		display = id
	}
	capSet := make(map[string]struct{})
	for _, modality := range raw.InputModalities {
		modality = strings.ToLower(strings.TrimSpace(modality))
		if modality != "" {
			capSet["input:"+modality] = struct{}{}
		}
	}
	if raw.SupportsPersonality {
		capSet["personality"] = struct{}{}
	}
	if len(raw.MultiAgentVersion) > 0 && strings.TrimSpace(string(raw.MultiAgentVersion)) != "null" {
		capSet["multi-agent"] = struct{}{}
	}
	caps := make([]string, 0, len(capSet))
	for capability := range capSet {
		caps = append(caps, capability)
	}
	sort.Strings(caps)
	return harnessmodel.ProviderModelDescriptor{
		AccountID:    accountID,
		Provider:     harnessmodel.ProviderCodex,
		ID:           harnessmodel.ProviderModelID(id),
		DisplayName:  display,
		Capabilities: caps,
		Enabled:      !raw.Hidden,
	}, nil
}

func decodeResult(payload []byte, target any) error {
	if err := validatePayload(payload); err != nil {
		return err
	}
	var envelope struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return err
	}
	body := payload
	if len(envelope.Error) > 0 && strings.TrimSpace(string(envelope.Error)) != "null" {
		return fmt.Errorf("JSON-RPC response contains error")
	}
	if len(envelope.Result) > 0 {
		body = envelope.Result
	}
	return json.Unmarshal(body, target)
}

func decodeNotification(payload []byte, expectedMethod string, target any) error {
	if err := validatePayload(payload); err != nil {
		return err
	}
	var envelope struct {
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return err
	}
	body := payload
	if envelope.Method != "" {
		if envelope.Method != expectedMethod {
			return fmt.Errorf("unexpected JSON-RPC method %q", envelope.Method)
		}
		if len(envelope.Params) == 0 {
			return fmt.Errorf("JSON-RPC notification %s missing params", expectedMethod)
		}
		body = envelope.Params
	}
	return json.Unmarshal(body, target)
}

func validatePayload(payload []byte) error {
	if len(payload) == 0 {
		return fmt.Errorf("Codex protocol payload is empty")
	}
	if len(payload) > maxProtocolBytes {
		return fmt.Errorf("Codex protocol payload exceeds %d bytes", maxProtocolBytes)
	}
	return nil
}

// ThreadUsageObservation preserves the App Server token/context signal without
// fabricating ProviderSessionSnapshot.ModelID. A later execution/session broker
// may bind this thread to a model when authoritative linkage exists.
type ThreadUsageObservation struct {
	ThreadID           string
	TurnID             string
	TotalTokens        int64
	InputTokens        int64
	CachedInputTokens  int64
	CacheWriteTokens   int64
	OutputTokens       int64
	ReasoningTokens    int64
	ModelContextWindow int64
	ObservedAt         time.Time
}

type threadTokenUsageUpdatedNotification struct {
	ThreadID   string `json:"threadId"`
	TurnID     string `json:"turnId"`
	TokenUsage struct {
		Total tokenUsageBreakdown `json:"total"`
		Last  tokenUsageBreakdown `json:"last"`
		ModelContextWindow *int64 `json:"modelContextWindow"`
	} `json:"tokenUsage"`
}

type tokenUsageBreakdown struct {
	TotalTokens           int64 `json:"totalTokens"`
	InputTokens           int64 `json:"inputTokens"`
	CachedInputTokens     int64 `json:"cachedInputTokens"`
	CacheWriteInputTokens int64 `json:"cacheWriteInputTokens"`
	OutputTokens          int64 `json:"outputTokens"`
	ReasoningOutputTokens int64 `json:"reasoningOutputTokens"`
}

func ParseThreadTokenUsageUpdated(payload []byte, observedAt time.Time) (ThreadUsageObservation, error) {
	if observedAt.IsZero() {
		return ThreadUsageObservation{}, fmt.Errorf("Codex thread token usage observedAt is required")
	}
	var notification threadTokenUsageUpdatedNotification
	if err := decodeNotification(payload, "thread/tokenUsage/updated", &notification); err != nil {
		return ThreadUsageObservation{}, fmt.Errorf("decode Codex thread/tokenUsage/updated: %w", err)
	}
	threadID := strings.TrimSpace(notification.ThreadID)
	turnID := strings.TrimSpace(notification.TurnID)
	if threadID == "" || turnID == "" {
		return ThreadUsageObservation{}, fmt.Errorf("Codex threadId and turnId are required")
	}
	if err := validateTokenBreakdown(notification.TokenUsage.Total); err != nil {
		return ThreadUsageObservation{}, fmt.Errorf("Codex total token usage: %w", err)
	}
	if err := validateTokenBreakdown(notification.TokenUsage.Last); err != nil {
		return ThreadUsageObservation{}, fmt.Errorf("Codex last token usage: %w", err)
	}
	contextWindow := int64(0)
	if notification.TokenUsage.ModelContextWindow != nil {
		if *notification.TokenUsage.ModelContextWindow < 0 {
			return ThreadUsageObservation{}, fmt.Errorf("Codex modelContextWindow must be non-negative")
		}
		contextWindow = *notification.TokenUsage.ModelContextWindow
	}
	total := notification.TokenUsage.Total
	return ThreadUsageObservation{
		ThreadID:           threadID,
		TurnID:             turnID,
		TotalTokens:        total.TotalTokens,
		InputTokens:        total.InputTokens,
		CachedInputTokens:  total.CachedInputTokens,
		CacheWriteTokens:   total.CacheWriteInputTokens,
		OutputTokens:       total.OutputTokens,
		ReasoningTokens:    total.ReasoningOutputTokens,
		ModelContextWindow: contextWindow,
		ObservedAt:         observedAt.UTC(),
	}, nil
}

func validateTokenBreakdown(v tokenUsageBreakdown) error {
	for _, value := range []int64{v.TotalTokens, v.InputTokens, v.CachedInputTokens, v.CacheWriteInputTokens, v.OutputTokens, v.ReasoningOutputTokens} {
		if value < 0 {
			return fmt.Errorf("token counts must be non-negative")
		}
	}
	return nil
}
