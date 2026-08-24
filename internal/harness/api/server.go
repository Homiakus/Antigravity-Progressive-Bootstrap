package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	harnessengine "github.com/homiakus/agctl/internal/harness/engine"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type Server struct {
	engine *harnessengine.Engine
	store  harnessstore.Store
	mux    *http.ServeMux
}

func NewServer(eng *harnessengine.Engine, store harnessstore.Store) *Server {
	s := &Server{
		engine: eng,
		store:  store,
		mux:    http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/v1/workflows", s.handleWorkflows)
	s.mux.HandleFunc("/v1/workflows/", s.handleWorkflowByID)
	s.mux.HandleFunc("/v1/signals", s.handleSignals)
	s.mux.HandleFunc("/v1/events/stream", s.handleEventStream)
}

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodPost:
		var def harnessmodel.WorkflowDefinition
		if err := json.NewDecoder(r.Body).Decode(&def); err != nil {
			http.Error(w, fmt.Sprintf("invalid workflow definition json: %v", err), http.StatusBadRequest)
			return
		}
		if def.CreatedAt.IsZero() {
			def.CreatedAt = time.Now().UTC()
		}
		run, err := s.engine.StartWorkflow(ctx, def)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to start workflow: %v", err), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusCreated, run)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleWorkflowByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	path := strings.TrimPrefix(r.URL.Path, "/v1/workflows/")
	parts := strings.Split(path, "/")
	runID := harnessmodel.WorkflowRunID(parts[0])

	if len(parts) == 1 && r.Method == http.MethodGet {
		var run harnessmodel.WorkflowRun
		err := s.store.View(ctx, func(reader harnessstore.Reader) error {
			var err error
			run, err = reader.GetWorkflowRun(ctx, runID)
			return err
		})
		if err != nil {
			http.Error(w, "workflow not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, run)
		return
	}

	if len(parts) == 2 {
		action := parts[1]
		switch action {
		case "pause":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			res, err := s.engine.PauseWorkflow(ctx, runID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, res)
			return
		case "resume":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			res, err := s.engine.ResumeWorkflow(ctx, runID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, res)
			return
		case "cancel":
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			stats, err := s.engine.CancelWorkflow(ctx, runID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, stats)
			return
		}
	}

	http.Error(w, "not found", http.StatusNotFound)
}

func (s *Server) handleSignals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		WorkflowRunID string `json:"workflowRunId"`
		Name          string `json:"name"`
		Payload       string `json:"payload"`
		DedupeKey     string `json:"dedupeKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	sigRes, err := s.engine.SendSignal(r.Context(), harnessmodel.WorkflowRunID(req.WorkflowRunID), req.Name, req.DedupeKey, []byte(req.Payload))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, sigRes)
}

func (s *Server) handleEventStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	runID := harnessmodel.WorkflowRunID(r.URL.Query().Get("workflowRunId"))
	if runID == "" {
		http.Error(w, "workflowRunId parameter required", http.StatusBadRequest)
		return
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var lastSeq int64 = -1
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			var eventsList []any
			_ = s.store.View(r.Context(), func(reader harnessstore.Reader) error {
				evs, err := reader.ListEvents(r.Context(), runID, lastSeq, 50)
				if err == nil {
					for _, ev := range evs {
						if ev.WorkflowSeq > lastSeq {
							lastSeq = ev.WorkflowSeq
						}
						eventsList = append(eventsList, ev)
					}
				}
				return nil
			})

			for _, ev := range eventsList {
				payload, _ := json.Marshal(ev)
				fmt.Fprintf(w, "data: %s\n\n", payload)
				flusher.Flush()
			}
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
