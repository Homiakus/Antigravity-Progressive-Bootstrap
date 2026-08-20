package engine

import (
	"context"
	"strings"
	"testing"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func TestStartWorkflowRejectsInvalidCompiledDefinitionBeforePersistence(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*harnessmodel.WorkflowDefinition)
		want   string
	}{
		{
			name: "cycle",
			mutate: func(def *harnessmodel.WorkflowDefinition) {
				def.Nodes[0].Dependencies = []harnessmodel.NodeID{"b"}
				def.Nodes[1].Dependencies = []harnessmodel.NodeID{"a"}
				def.Nodes = def.Nodes[:2]
			},
			want: "cycle",
		},
		{
			name: "duplicate dependency",
			mutate: func(def *harnessmodel.WorkflowDefinition) {
				def.Nodes[1].Dependencies = []harnessmodel.NodeID{"a", "a"}
				def.Nodes = def.Nodes[:2]
			},
			want: "duplicate dependency",
		},
		{
			name: "negative resource",
			mutate: func(def *harnessmodel.WorkflowDefinition) {
				def.Nodes[0].Resources.CPUWeight = -1
				def.Nodes = def.Nodes[:1]
			},
			want: "negative resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			eng, db, _, _ := newTestEngine(t)
			def := dagDefinition()
			tt.mutate(&def)
			if _, err := eng.StartWorkflow(ctx, def); err == nil || !strings.Contains(strings.ToLower(err.Error()), tt.want) {
				t.Fatalf("StartWorkflow error=%v want substring %q", err, tt.want)
			}
			for _, table := range []string{"workflow_definitions", "workflow_runs", "workflow_progress", "graph_revisions", "node_runs", "events"} {
				var count int
				if err := db.SQLDB().QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatalf("invalid definition leaked %d rows into %s", count, table)
				}
			}
		})
	}
}
