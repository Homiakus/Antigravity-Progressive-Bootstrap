package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	harnessengine "github.com/homiakus/agctl/internal/harness/engine"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
)

func TestAPIServerWorkflowLifecycleAndSignals(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	db, err := sqlitestore.Open(ctx, filepath.Join(tempDir, "state.db"), sqlitestore.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Unix(1000, 0).UTC()
	eng, err := harnessengine.New(db, harnessengine.Options{
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServer(eng, db)
	handler := srv.Handler()

	// 1. POST /v1/workflows
	def := harnessmodel.WorkflowDefinition{
		ID:              "wfd_api_test",
		Version:         1,
		Name:            "api-test",
		CompilerVersion: "test",
		Nodes: []harnessmodel.NodeSpec{
			{ID: "step1", Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorProcess},
		},
	}
	body, _ := json.Marshal(def)
	req := httptest.NewRequest(http.MethodPost, "/v1/workflows", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var run harnessmodel.WorkflowRun
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if run.ID == "" {
		t.Fatal("expected non-empty workflow run id")
	}

	// 2. GET /v1/workflows/:id
	reqGet := httptest.NewRequest(http.MethodGet, "/v1/workflows/"+string(run.ID), nil)
	recGet := httptest.NewRecorder()
	handler.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recGet.Code)
	}

	// 3. POST /v1/workflows/:id/pause
	reqPause := httptest.NewRequest(http.MethodPost, "/v1/workflows/"+string(run.ID)+"/pause", nil)
	recPause := httptest.NewRecorder()
	handler.ServeHTTP(recPause, reqPause)

	if recPause.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for pause, got %d", recPause.Code)
	}

	// 4. POST /v1/signals
	sigReq := map[string]string{
		"workflowRunId": string(run.ID),
		"name":          "deploy_approved",
		"payload":       `{"approved":true}`,
		"dedupeKey":     "deploy_1",
	}
	sigBody, _ := json.Marshal(sigReq)
	reqSig := httptest.NewRequest(http.MethodPost, "/v1/signals", bytes.NewReader(sigBody))
	recSig := httptest.NewRecorder()
	handler.ServeHTTP(recSig, reqSig)

	if recSig.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for signal, got %d", recSig.Code)
	}
}
