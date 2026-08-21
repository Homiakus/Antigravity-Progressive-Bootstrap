package compiler

import (
	"strings"
	"testing"

	"github.com/homiakus/agctl/internal/harness/ir"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestRetryPolicyNameMustBeCanonical(t *testing.T) {
	def := ir.Definition{
		Name: "retry-policy-name",
		RetryPolicies: map[string]harnessmodel.RetryPolicySpec{
			" transient ": {MaxAttempts: 2},
		},
		Nodes: []harnessmodel.NodeSpec{action("a")},
	}
	_, err := Compile(def, deterministicOpts())
	if err == nil || !strings.Contains(err.Error(), "surrounding whitespace") {
		t.Fatalf("non-canonical retry policy name error=%v", err)
	}
}
