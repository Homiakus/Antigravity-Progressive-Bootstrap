package telemetry

import (
	"sync"
	"time"
)

type Snapshot struct {
	ActiveWorkflows        int64         `json:"activeWorkflows"`
	ReadyQueueDepth        int64         `json:"readyQueueDepth"`
	TotalStateTransitions  int64         `json:"totalStateTransitions"`
	TotalAttempts          int64         `json:"totalAttempts"`
	TotalRetries           int64         `json:"totalRetries"`
	TotalFailures          int64         `json:"totalFailures"`
	TotalLLMTokens         int64         `json:"totalLLMTokens"`
	TotalLLMCostUSD        float64       `json:"totalLLMCostUSD"`
	AverageNodeDuration    time.Duration `json:"averageNodeDuration"`
}

type MetricsCollector struct {
	mu                    sync.RWMutex
	activeWorkflows       int64
	readyQueueDepth       int64
	totalStateTransitions int64
	totalAttempts         int64
	totalRetries          int64
	totalFailures         int64
	totalLLMTokens        int64
	totalLLMCostUSD       float64
	durationsCount        int64
	durationsTotal        time.Duration
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

func (m *MetricsCollector) IncStateTransitions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalStateTransitions++
}

func (m *MetricsCollector) IncAttempts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalAttempts++
}

func (m *MetricsCollector) IncRetries() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalRetries++
}

func (m *MetricsCollector) IncFailures() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalFailures++
}

func (m *MetricsCollector) RecordLLMUsage(tokens int64, costUSD float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalLLMTokens += tokens
	m.totalLLMCostUSD += costUSD
}

func (m *MetricsCollector) RecordNodeDuration(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durationsCount++
	m.durationsTotal += d
}

func (m *MetricsCollector) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var avg time.Duration
	if m.durationsCount > 0 {
		avg = m.durationsTotal / time.Duration(m.durationsCount)
	}

	return Snapshot{
		ActiveWorkflows:       m.activeWorkflows,
		ReadyQueueDepth:       m.readyQueueDepth,
		TotalStateTransitions: m.totalStateTransitions,
		TotalAttempts:         m.totalAttempts,
		TotalRetries:          m.totalRetries,
		TotalFailures:         m.totalFailures,
		TotalLLMTokens:        m.totalLLMTokens,
		TotalLLMCostUSD:       m.totalLLMCostUSD,
		AverageNodeDuration:   avg,
	}
}
