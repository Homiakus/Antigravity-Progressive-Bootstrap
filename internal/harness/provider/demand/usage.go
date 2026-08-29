package demand

import (
	"fmt"
	"strings"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

// UsageClassification supplies semantic dimensions that ProviderUsageSample
// intentionally does not persist itself. The caller must derive these from an
// authoritative execution/workspace linkage; this adapter never guesses them
// from IDs, names, or conversation text.
type UsageClassification struct {
	TaskClass    string
	Provider     harnessmodel.ProviderKind
	RepositoryID string
	ContextClass string
}

func (c UsageClassification) Validate() error {
	if strings.TrimSpace(c.TaskClass) == "" {
		return fmt.Errorf("usage demand task class is required")
	}
	if !c.Provider.Valid() {
		return fmt.Errorf("invalid usage demand provider %q", c.Provider)
	}
	if len(c.TaskClass) > 128 || len(c.RepositoryID) > 256 || len(c.ContextClass) > 128 {
		return fmt.Errorf("usage demand classification field exceeds size limit")
	}
	return nil
}

// SampleFromUsage converts one durable, idempotent provider-usage record into
// an estimator sample while preserving ModelID, Metric, Amount and ObservedAt
// exactly. It is deliberately side-effect free and performs no persistence.
func SampleFromUsage(classification UsageClassification, usage harnessmodel.ProviderUsageSample) (Sample, error) {
	if err := classification.Validate(); err != nil {
		return Sample{}, err
	}
	if err := usage.Validate(); err != nil {
		return Sample{}, fmt.Errorf("provider usage sample: %w", err)
	}
	sample := Sample{
		Key: Key{
			TaskClass:    strings.TrimSpace(classification.TaskClass),
			Provider:     classification.Provider,
			ModelID:      usage.ModelID,
			RepositoryID: strings.TrimSpace(classification.RepositoryID),
			ContextClass: strings.TrimSpace(classification.ContextClass),
			Metric:       usage.Metric,
		},
		Amount:     usage.Amount,
		ObservedAt: usage.ObservedAt,
	}
	if err := sample.Validate(); err != nil {
		return Sample{}, err
	}
	return sample, nil
}
