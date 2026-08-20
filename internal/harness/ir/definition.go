package ir

import (
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

type Definition struct {
	ID            harnessmodel.WorkflowDefinitionID       `json:"id,omitempty"`
	Version       int                                     `json:"version,omitempty"`
	Name          string                                  `json:"name"`
	CreatedAt     time.Time                               `json:"createdAt,omitempty"`
	EntryNodes    []harnessmodel.NodeID                   `json:"entryNodes,omitempty"`
	Nodes         []harnessmodel.NodeSpec                 `json:"nodes"`
	RetryPolicies map[string]harnessmodel.RetryPolicySpec `json:"retryPolicies,omitempty"`
	Metadata      map[string]string                       `json:"metadata,omitempty"`
}
