package resource

import harnessmodel "github.com/homiakus/agctl/internal/harness/model"

type Capacity struct {
	CPUWeight int
	MemoryBytes int64
	GPUCount int
	MaxVRAMBytes int64
	DiskBytes int64
	BuildSlots int
	BrowserSlots int
}

type ConstraintFailure struct {
	Resource string `json:"resource"`
	Required int64 `json:"required"`
	Available int64 `json:"available"`
}

func Fits(cap Capacity, req harnessmodel.ResourceSpec) (bool, []ConstraintFailure) {
	failures := make([]ConstraintFailure, 0, 7)
	check := func(name string, required, available int64) {
		if required > available {
			failures = append(failures, ConstraintFailure{Resource: name, Required: required, Available: available})
		}
	}
	check("cpu_weight", int64(req.CPUWeight), int64(cap.CPUWeight))
	check("memory_bytes", req.MemoryBytes, cap.MemoryBytes)
	check("gpu_count", int64(req.GPUCount), int64(cap.GPUCount))
	check("vram_bytes", req.MinVRAMBytes, cap.MaxVRAMBytes)
	check("disk_bytes", req.DiskBytes, cap.DiskBytes)
	check("build_slots", int64(req.BuildSlots), int64(cap.BuildSlots))
	check("browser_slots", int64(req.BrowserSlots), int64(cap.BrowserSlots))
	return len(failures) == 0, failures
}
