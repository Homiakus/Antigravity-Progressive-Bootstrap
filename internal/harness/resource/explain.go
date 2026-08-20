package resource

import (
	"fmt"
	"strings"
)

func ExplainFailures(failures []ConstraintFailure) string {
	if len(failures) == 0 {
		return ""
	}
	parts := make([]string, 0, len(failures))
	for _, failure := range failures {
		parts = append(parts, fmt.Sprintf("%s required=%d available=%d", failure.Resource, failure.Required, failure.Available))
	}
	return strings.Join(parts, "; ")
}
