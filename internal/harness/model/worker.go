package model

import "time"

type WorkerState string

const (
	WorkerActive   WorkerState = "ACTIVE"
	WorkerDraining WorkerState = "DRAINING"
	WorkerLost     WorkerState = "LOST"
)

type WorkerTrust string

const (
	WorkerTrustedLocal   WorkerTrust = "TRUSTED_LOCAL"
	WorkerTrustedRemote  WorkerTrust = "TRUSTED_REMOTE"
	WorkerUntrustedRemote WorkerTrust = "UNTRUSTED_REMOTE"
)

type WorkerResources struct {
	CPUWeight    int   `json:"cpuWeight,omitempty"`
	MemoryBytes  int64 `json:"memoryBytes,omitempty"`
	GPUCount     int   `json:"gpuCount,omitempty"`
	MaxVRAMBytes int64 `json:"maxVramBytes,omitempty"`
	DiskBytes    int64 `json:"diskBytes,omitempty"`
	BuildSlots   int   `json:"buildSlots,omitempty"`
	BrowserSlots int   `json:"browserSlots,omitempty"`
}

type Worker struct {
	ID           WorkerID        `json:"id"`
	Name         string          `json:"name,omitempty"`
	State        WorkerState     `json:"state"`
	Trust        WorkerTrust     `json:"trust"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Resources    WorkerResources `json:"resources"`
	CreatedAt    time.Time       `json:"createdAt"`
	LastSeenAt   time.Time       `json:"lastSeenAt"`
}
