package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type SignalResult struct {
	Signal             harnessmodel.Signal    `json:"signal"`
	DeliveredToNodeRun harnessmodel.NodeRunID `json:"deliveredToNodeRunId,omitempty"`
	Created            bool                   `json:"created"`
}

// WaitForSignal moves a WAIT node into durable WAITING state. If a matching
// signal arrived earlier, the same transaction consumes the oldest pending
// signal and restores the node to READY, so activation order cannot lose an
// external event.
func (e *Engine) WaitForSignal(ctx context.Context, nodeRunID harnessmodel.NodeRunID, signalName string) (harnessmodel.SignalWait, error) {
	if nodeRunID == "" || strings.TrimSpace(signalName) == "" {
		return harnessmodel.SignalWait{}, fmt.Errorf("node run id and signal name are required")
	}
	now := e.now().UTC()
	var result harnessmodel.SignalWait
	err := e.store.Update(ctx, func(tx harnessstore.Tx) error {
		nr, err := tx.GetNodeRun(ctx, nodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeReady {
			return fmt.Errorf("cannot wait for signal from node state %s", nr.State)
		}
		run, err := tx.GetWorkflowRun(ctx, nr.WorkflowRunID)
		if err != nil {
			return err
		}
		if run.State != harnessmodel.WorkflowRunning {
			return fmt.Errorf("cannot wait node %s while workflow %s is %s", nr.ID, run.ID, run.State)
		}
		def, err := tx.GetWorkflowDefinition(ctx, run.DefinitionID, run.DefinitionVersion)
		if err != nil {
			return err
		}
		node, ok := findNodeSpec(def, nr.NodeID)
		if !ok || node.Kind != harnessmodel.NodeKindWait {
			return fmt.Errorf("node %s is not a WAIT control-flow node", nr.NodeID)
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeWaiting, now); err != nil {
			return err
		}
		if err := tx.RemoveReadyNode(ctx, nr.ID); err != nil {
			return err
		}
		wait := harnessmodel.SignalWait{
			NodeRunID: nr.ID, WorkflowRunID: run.ID, SignalName: signalName,
			State: harnessmodel.SignalWaitWaiting, CreatedAt: now,
		}
		if err := tx.CreateSignalWait(ctx, wait); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "SignalWaitStarted", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "signalName": signalName}); err != nil {
			return err
		}

		pending, err := tx.ListPendingSignals(ctx, run.ID, signalName, 1)
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			result = wait
			return nil
		}
		if err := tx.DeliverSignal(ctx, nr.ID, pending[0].ID, now); err != nil {
			return err
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeReady, now); err != nil {
			return err
		}
		if err := tx.EnqueueReadyNode(ctx, nr.ID, now, time.Time{}, ""); err != nil {
			return err
		}
		wait.State = harnessmodel.SignalWaitDelivered
		wait.DeliveredSignalID = pending[0].ID
		wait.ResolvedAt = now
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "SignalDelivered", "signal", string(pending[0].ID), map[string]any{"nodeRunId": nr.ID, "signalName": signalName, "arrivedBeforeWait": true}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeReady", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "wakeReason": "signal", "signalId": pending[0].ID}); err != nil {
			return err
		}
		result = wait
		return nil
	})
	return result, err
}

// SendSignal persists the signal before attempting delivery. messageID is a
// producer idempotency key scoped by workflow and signal name; replaying the
// same payload recovers the original signal, while a different payload with
// the same key is rejected by the store.
func (e *Engine) SendSignal(ctx context.Context, runID harnessmodel.WorkflowRunID, signalName, messageID string, payload []byte) (SignalResult, error) {
	if runID == "" || strings.TrimSpace(signalName) == "" || strings.TrimSpace(messageID) == "" {
		return SignalResult{}, fmt.Errorf("workflow run id, signal name and message id are required")
	}
	now := e.now().UTC()
	rawSignalID, err := e.nextID(harnessmodel.IDSignal)
	if err != nil {
		return SignalResult{}, err
	}
	var result SignalResult
	err = e.store.Update(ctx, func(tx harnessstore.Tx) error {
		run, err := tx.GetWorkflowRun(ctx, runID)
		if err != nil {
			return err
		}
		signal := harnessmodel.Signal{
			ID: harnessmodel.SignalID(rawSignalID), WorkflowRunID: run.ID,
			Name: signalName, MessageID: messageID, Payload: append([]byte(nil), payload...),
			State: harnessmodel.SignalPending, ReceivedAt: now,
		}
		stored, created, err := tx.PutSignal(ctx, signal)
		if err != nil {
			return err
		}
		result.Signal = stored
		result.Created = created
		if created {
			if _, err := e.appendEvent(ctx, tx, run.ID, now, "SignalReceived", "signal", string(stored.ID), map[string]any{"signalName": signalName, "messageId": messageID}); err != nil {
				return err
			}
		}
		if stored.State == harnessmodel.SignalConsumed {
			result.DeliveredToNodeRun = stored.ConsumedByNodeRunID
			return nil
		}

		waits, err := tx.ListSignalWaits(ctx, run.ID, signalName, 1)
		if err != nil {
			return err
		}
		if len(waits) == 0 {
			return nil
		}
		wait := waits[0]
		if err := tx.DeliverSignal(ctx, wait.NodeRunID, stored.ID, now); err != nil {
			return err
		}
		nr, err := tx.GetNodeRun(ctx, wait.NodeRunID)
		if err != nil {
			return err
		}
		if nr.State != harnessmodel.NodeWaiting {
			return fmt.Errorf("signal waiter node %s is %s: %w", nr.ID, nr.State, harnessstore.ErrConflict)
		}
		if err := transitionNode(ctx, tx, &nr, harnessmodel.NodeReady, now); err != nil {
			return err
		}
		if err := tx.EnqueueReadyNode(ctx, nr.ID, now, time.Time{}, ""); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "SignalDelivered", "signal", string(stored.ID), map[string]any{"nodeRunId": nr.ID, "signalName": signalName, "arrivedBeforeWait": false}); err != nil {
			return err
		}
		if _, err := e.appendEvent(ctx, tx, run.ID, now, "NodeReady", "node_run", string(nr.ID), map[string]any{"nodeId": nr.NodeID, "wakeReason": "signal", "signalId": stored.ID}); err != nil {
			return err
		}
		result.Signal = stored
		result.Signal.State = harnessmodel.SignalConsumed
		result.Signal.ConsumedByNodeRunID = nr.ID
		result.Signal.ConsumedAt = now
		result.DeliveredToNodeRun = nr.ID
		return nil
	})
	return result, err
}
