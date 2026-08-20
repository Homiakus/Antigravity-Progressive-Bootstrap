package compiler

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/homiakus/agctl/internal/harness/ir"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func action(id harnessmodel.NodeID, deps ...harnessmodel.NodeID) harnessmodel.NodeSpec {
	return harnessmodel.NodeSpec{ID: id, Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: deps, CachePolicy: harnessmodel.CacheDisabled}
}

func deterministicOpts() Options {
	return Options{
		IDs: harnessmodel.TimeSortableIDGenerator{
			Now:    func() time.Time { return time.UnixMilli(1) },
			Random: bytes.NewReader(make([]byte, 10)),
		},
		Now: func() time.Time { return time.Unix(2, 0) },
	}
}

func TestCompileValidDAG(t *testing.T) {
	got, err := Compile(ir.Definition{Name: "test", Nodes: []harnessmodel.NodeSpec{action("a"), action("b", "a"), action("c", "a"), action("d", "b", "c")}}, deterministicOpts())
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.CompilerVersion != Version || len(got.Nodes) != 4 {
		t.Fatalf("unexpected compiled definition: %+v", got)
	}
	if got.ID == "" {
		t.Fatal("missing generated definition id")
	}
}

func TestValidationFailures(t *testing.T) {
	missingRetryRef := action("a")
	missingRetryRef.RetryPolicyRef = "missing"
	cases := map[string]ir.Definition{
		"duplicate id":       {Name: "x", Nodes: []harnessmodel.NodeSpec{action("a"), action("a")}},
		"missing dependency": {Name: "x", Nodes: []harnessmodel.NodeSpec{action("a", "missing")}},
		"self cycle":         {Name: "x", Nodes: []harnessmodel.NodeSpec{action("a", "a")}},
		"multi cycle":        {Name: "x", Nodes: []harnessmodel.NodeSpec{action("a", "b"), action("b", "a")}},
		"bad executor":       {Name: "x", Nodes: []harnessmodel.NodeSpec{{ID: "a", Kind: harnessmodel.NodeKindAction, ExecutorKind: "NOPE"}}},
		"negative resource":  {Name: "x", Nodes: []harnessmodel.NodeSpec{{ID: "a", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Resources: harnessmodel.ResourceSpec{GPUCount: -1}}}},
		"unreachable":        {Name: "x", EntryNodes: []harnessmodel.NodeID{"a"}, Nodes: []harnessmodel.NodeSpec{action("a"), action("b")}},
		"missing retry ref":  {Name: "x", Nodes: []harnessmodel.NodeSpec{missingRetryRef}},
		"bad max attempts": {Name: "x", RetryPolicies: map[string]harnessmodel.RetryPolicySpec{"p": {MaxAttempts: 0}}, Nodes: []harnessmodel.NodeSpec{action("a")}},
		"negative retry delay": {Name: "x", RetryPolicies: map[string]harnessmodel.RetryPolicySpec{"p": {MaxAttempts: 2, InitialDelay: -time.Second}}, Nodes: []harnessmodel.NodeSpec{action("a")}},
		"bad retry class": {Name: "x", RetryPolicies: map[string]harnessmodel.RetryPolicySpec{"p": {MaxAttempts: 2, RetryableClasses: []harnessmodel.ErrorClass{"NOPE"}}}, Nodes: []harnessmodel.NodeSpec{action("a")}},
		"overlapping retry class": {Name: "x", RetryPolicies: map[string]harnessmodel.RetryPolicySpec{"p": {MaxAttempts: 2, RetryableClasses: []harnessmodel.ErrorClass{harnessmodel.ErrorTimeout}, NonRetryableClasses: []harnessmodel.ErrorClass{harnessmodel.ErrorTimeout}}}, Nodes: []harnessmodel.NodeSpec{action("a")}},
	}
	for name, def := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(def, deterministicOpts()); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCompileRetryPolicySnapshotAndReference(t *testing.T) {
	node := action("a")
	node.RetryPolicyRef = " transient "
	def := ir.Definition{
		Name: "retry-snapshot",
		RetryPolicies: map[string]harnessmodel.RetryPolicySpec{
			"transient": {
				MaxAttempts: 4, InitialDelay: time.Second, BackoffFactor: 2,
				RetryableClasses: []harnessmodel.ErrorClass{harnessmodel.ErrorTimeout, harnessmodel.ErrorInfraTransient},
			},
		},
		Nodes: []harnessmodel.NodeSpec{node},
	}
	compiled, err := Compile(def, deterministicOpts())
	if err != nil {
		t.Fatal(err)
	}
	if compiled.Nodes[0].RetryPolicyRef != "transient" {
		t.Fatalf("retry policy ref not normalized: %q", compiled.Nodes[0].RetryPolicyRef)
	}
	policy := compiled.RetryPolicies["transient"]
	if policy.MaxAttempts != 4 || len(policy.RetryableClasses) != 2 || policy.RetryableClasses[0] != harnessmodel.ErrorInfraTransient || policy.RetryableClasses[1] != harnessmodel.ErrorTimeout {
		t.Fatalf("unexpected compiled retry snapshot: %+v", policy)
	}
	// Mutating the authoring IR after compilation must not mutate durable policy.
	original := def.RetryPolicies["transient"]
	original.RetryableClasses[0] = harnessmodel.ErrorProtocol
	def.RetryPolicies["transient"] = original
	if compiled.RetryPolicies["transient"].RetryableClasses[0] != harnessmodel.ErrorInfraTransient {
		t.Fatal("compiled retry policy aliases mutable input")
	}
}

func TestNodeSpecContainsNoRuntimeFields(t *testing.T) {
	forbidden := map[string]bool{"StartedAt": true, "FinishedAt": true, "PID": true, "ProcessID": true, "Attempts": true, "Status": true, "Attempt": true}
	typ := reflect.TypeOf(harnessmodel.NodeSpec{})
	for i := 0; i < typ.NumField(); i++ {
		if forbidden[typ.Field(i).Name] {
			t.Fatalf("NodeSpec contains runtime field %s", typ.Field(i).Name)
		}
	}
}

func TestCompileDoesNotAliasMutableInput(t *testing.T) {
	def := ir.Definition{Name: "x", Metadata: map[string]string{"k": "v"}, Nodes: []harnessmodel.NodeSpec{{ID: "a", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess, Dependencies: []harnessmodel.NodeID{}, Metadata: map[string]string{"x": "y"}, Resources: harnessmodel.ResourceSpec{Capabilities: []string{"z", "a"}}}}}
	compiled, err := Compile(def, deterministicOpts())
	if err != nil {
		t.Fatal(err)
	}
	def.Metadata["k"] = "changed"
	def.Nodes[0].Metadata["x"] = "changed"
	def.Nodes[0].Resources.Capabilities[0] = "changed"
	if compiled.Metadata["k"] != "v" || compiled.Nodes[0].Metadata["x"] != "y" || strings.Join(compiled.Nodes[0].Resources.Capabilities, ",") != "a,z" {
		t.Fatalf("compiled definition aliases input: %+v", compiled)
	}
}
