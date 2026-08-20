package ir

import (
	"reflect"
	"testing"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestDefinitionJSONRoundTrip(t *testing.T) {
	in := Definition{
		ID:         "wfd_0000000000000_00000000000000000000",
		Version:    3,
		Name:       "round-trip",
		EntryNodes: []harnessmodel.NodeID{"a"},
		Nodes: []harnessmodel.NodeSpec{{
			ID:           "a",
			Kind:         harnessmodel.NodeKindAction,
			ExecutorKind: harnessmodel.ExecutorProcess,
			CachePolicy:  harnessmodel.CacheDisabled,
			Metadata:     map[string]string{"purpose": "test"},
		}},
		Metadata: map[string]string{"owner": "harness"},
	}
	data, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round-trip mismatch:\nin=%+v\nout=%+v", in, out)
	}
}
