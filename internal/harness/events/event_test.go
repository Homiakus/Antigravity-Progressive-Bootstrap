package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestEventValidation(t *testing.T) {
	valid := Event{ID: "evt_test", WorkflowRunID: "wfr_test", Type: "WorkflowStarted", Timestamp: time.Unix(1, 0).UTC(), EntityType: "workflow_run", EntityID: "wfr_test", PayloadVersion: 1, Payload: json.RawMessage(`{"ok":true}`)}
	if err := valid.ValidateForAppend(); err != nil {
		t.Fatal(err)
	}
	cases := []Event{
		{},
		{ID: "evt", WorkflowRunID: "run", WorkflowSeq: 1, Type: "x", Timestamp: valid.Timestamp, EntityType: "workflow_run", EntityID: "run", PayloadVersion: 1, Payload: json.RawMessage(`{}`)},
		{ID: "evt", WorkflowRunID: "run", Type: "x", Timestamp: valid.Timestamp, EntityType: "workflow_run", EntityID: "run", PayloadVersion: 1, Payload: json.RawMessage(`{broken`)},
	}
	for i, tc := range cases {
		if err := tc.ValidateForAppend(); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}
