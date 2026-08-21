package main

import (
	"context"
	"fmt"

	harnessengine "github.com/homiakus/agctl/internal/harness/engine"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	sqlitestore "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/paths"
)

func openHarnessEngine(p paths.Paths) (*sqlitestore.DB, *harnessengine.Engine, error) {
	db, err := sqlitestore.Open(context.Background(), p.HarnessDB, sqlitestore.Options{})
	if err != nil {
		return nil, nil, err
	}
	eng, err := harnessengine.New(db, harnessengine.Options{})
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}
	return db, eng, nil
}

func runHarnessPause(p paths.Paths, runID harnessmodel.WorkflowRunID) error {
	db, eng, err := openHarnessEngine(p)
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := eng.PauseWorkflow(context.Background(), runID)
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runHarnessResume(p paths.Paths, runID harnessmodel.WorkflowRunID) error {
	db, eng, err := openHarnessEngine(p)
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := eng.ResumeWorkflow(context.Background(), runID)
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runHarnessCancel(p paths.Paths, runID harnessmodel.WorkflowRunID) error {
	db, eng, err := openHarnessEngine(p)
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := eng.CancelWorkflow(context.Background(), runID)
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runHarnessSignal(p paths.Paths, runID harnessmodel.WorkflowRunID, name, messageID, payload string) error {
	db, eng, err := openHarnessEngine(p)
	if err != nil {
		return err
	}
	defer db.Close()
	result, err := eng.SendSignal(context.Background(), runID, name, messageID, []byte(payload))
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runHarnessApprovalDecision(p paths.Paths, approvalID harnessmodel.ApprovalID, actor string, approve bool) error {
	db, eng, err := openHarnessEngine(p)
	if err != nil {
		return err
	}
	defer db.Close()
	var result harnessengine.ApprovalResult
	if approve {
		result, err = eng.Approve(context.Background(), approvalID, actor)
	} else {
		result, err = eng.Reject(context.Background(), approvalID, actor)
	}
	if err != nil {
		return err
	}
	printJSON(result)
	return nil
}

func runHarnessApprovals(p paths.Paths, runID harnessmodel.WorkflowRunID, limit int) error {
	db, err := sqlitestore.Open(context.Background(), p.HarnessDB, sqlitestore.Options{})
	if err != nil {
		return err
	}
	defer db.Close()
	var approvals []harnessmodel.Approval
	if err := db.View(context.Background(), func(r harnessstore.Reader) error {
		var err error
		approvals, err = r.ListPendingApprovals(context.Background(), runID, limit)
		return err
	}); err != nil {
		return err
	}
	printJSON(approvals)
	return nil
}

type harnessSweepResult struct {
	ExpiredApprovals []harnessmodel.ApprovalID `json:"expiredApprovals,omitempty"`
	TimerReadyNodes  []harnessmodel.NodeRunID  `json:"timerReadyNodes,omitempty"`
	RetryReadyNodes  []harnessmodel.NodeRunID  `json:"retryReadyNodes,omitempty"`
}

func runHarnessSweep(p paths.Paths, limit int) error {
	if limit <= 0 {
		return fmt.Errorf("--limit must be positive")
	}
	db, eng, err := openHarnessEngine(p)
	if err != nil {
		return err
	}
	defer db.Close()
	ctx := context.Background()
	result := harnessSweepResult{}
	if result.ExpiredApprovals, err = eng.ReleaseExpiredApprovals(ctx, limit); err != nil {
		return err
	}
	if result.TimerReadyNodes, err = eng.ReleaseDueTimers(ctx, limit); err != nil {
		return err
	}
	if result.RetryReadyNodes, err = eng.ReleaseDueRetries(ctx, limit); err != nil {
		return err
	}
	printJSON(result)
	return nil
}
