package migratelegacy

import (
	"context"
	"errors"
	"fmt"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type ApplyResult struct {
	SourceID        string `json:"sourceId"`
	Imported        bool   `json:"imported"`
	AlreadyImported bool   `json:"alreadyImported"`
}

func Apply(ctx context.Context, target harnessstore.Store, bundle Bundle) (ApplyResult, error) {
	result := ApplyResult{SourceID: bundle.SourceID}
	err := target.Update(ctx, func(tx harnessstore.Tx) error {
		existing, err := tx.GetWorkflowDefinition(ctx, bundle.Definition.ID, bundle.Definition.Version)
		if err == nil {
			if existing.Metadata["legacySourceFingerprint"] != bundle.SourceFingerprint {
				return fmt.Errorf("legacy source %s conflicts with an existing durable definition", bundle.SourceID)
			}
			complete, err := hasImportMarker(ctx, tx, bundle)
			if err != nil {
				return err
			}
			if !complete {
				return fmt.Errorf("legacy source %s has partial durable state without LegacyImportCompleted marker", bundle.SourceID)
			}
			result.AlreadyImported = true
			return nil
		}
		if !errors.Is(err, harnessstore.ErrNotFound) {
			return err
		}
		if err := tx.CreateWorkflowDefinition(ctx, bundle.Definition); err != nil {
			return err
		}
		if err := tx.CreateWorkflowRun(ctx, bundle.Run); err != nil {
			return err
		}
		terminal, failed := importedProgress(bundle.NodeRuns)
		if err := tx.CreateWorkflowProgress(ctx, harnessmodel.WorkflowProgress{WorkflowRunID: bundle.Run.ID, TotalNodes: len(bundle.NodeRuns), TerminalNodes: terminal, FailedNodes: failed, UpdatedAt: bundle.Run.UpdatedAt}); err != nil {
			return err
		}
		for _, rev := range bundle.Revisions {
			if err := tx.CreateGraphRevision(ctx, rev); err != nil {
				return err
			}
		}
		for _, nr := range bundle.NodeRuns {
			if err := tx.CreateNodeRun(ctx, nr); err != nil {
				return err
			}
		}
		for _, event := range bundle.Events {
			if _, err := tx.AppendEvent(ctx, event, nil); err != nil {
				return err
			}
		}
		result.Imported = true
		return nil
	})
	return result, err
}

func importedProgress(nodes []harnessmodel.NodeRun) (terminal, failed int) {
	for _, nr := range nodes {
		if nr.State.Terminal() {
			terminal++
		}
		if nr.State == harnessmodel.NodeFailed || nr.State == harnessmodel.NodeTimedOut {
			failed++
		}
	}
	return terminal, failed
}

func hasImportMarker(ctx context.Context, reader harnessstore.Reader, bundle Bundle) (bool, error) {
	var after int64
	for {
		events, err := reader.ListEvents(ctx, bundle.Run.ID, after, 10000)
		if err != nil {
			return false, err
		}
		if len(events) == 0 {
			return false, nil
		}
		for _, e := range events {
			if e.Type == "LegacyImportCompleted" {
				return true, nil
			}
		}
		after = events[len(events)-1].WorkflowSeq
		if len(events) < 10000 {
			return false, nil
		}
	}
}
