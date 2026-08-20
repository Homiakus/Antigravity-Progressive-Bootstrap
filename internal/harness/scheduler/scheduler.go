package scheduler

import (
	"context"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/resource"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type Options struct {
	Capacity              resource.Capacity
	LaneLimit             int
	CandidateLimit        int
	ClassificationBatches int
	Now                   func() time.Time
}

type Scheduler struct {
	store                 harnessstore.Store
	capacity              resource.Capacity
	laneLimit             int
	candidateLimit        int
	classificationBatches int
	now                   func() time.Time
}

type Decision struct {
	Node harnessmodel.ReadyNode `json:"node"`
}

func New(store harnessstore.Store, opts Options) (*Scheduler, error) {
	if store == nil {
		return nil, fmt.Errorf("harness store is required")
	}
	laneLimit := opts.LaneLimit
	if laneLimit <= 0 {
		laneLimit = 256
	}
	candidateLimit := opts.CandidateLimit
	if candidateLimit <= 0 {
		candidateLimit = 64
	}
	classificationBatches := opts.ClassificationBatches
	if classificationBatches <= 0 {
		classificationBatches = 16
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Scheduler{
		store: store, capacity: opts.Capacity, laneLimit: laneLimit,
		candidateLimit: candidateLimit, classificationBatches: classificationBatches, now: now,
	}, nil
}

func (s *Scheduler) SetWorkflowWeight(ctx context.Context, runID harnessmodel.WorkflowRunID, weight int) error {
	if runID == "" {
		return fmt.Errorf("workflow run id is required")
	}
	if weight <= 0 {
		return fmt.Errorf("workflow weight must be > 0")
	}
	now := s.now().UTC()
	return s.store.Update(ctx, func(tx harnessstore.Tx) error {
		return tx.SetWorkflowScheduleWeight(ctx, runID, weight, now)
	})
}

// Next returns one feasible READY node. Selection is two-level: first choose a
// workflow lane by normalized durable service (service_count/weight), then the
// highest effective priority feasible node inside that lane. Resource
// classification is performed in bounded batches so a prefix of infeasible
// high-priority nodes cannot permanently hide feasible work below it. The
// decision is not an execution claim; Stage 5 adds leases/fencing.
func (s *Scheduler) Next(ctx context.Context) (Decision, bool, error) {
	now := s.now().UTC()
	var decision Decision
	found := false
	err := s.store.Update(ctx, func(tx harnessstore.Tx) error {
		lanes, err := tx.ListReadyWorkflowLanes(ctx, now, s.laneLimit)
		if err != nil {
			return err
		}
		for _, lane := range lanes {
			for batch := 0; batch < s.classificationBatches; batch++ {
				nodes, err := tx.ListReadyNodes(ctx, lane.WorkflowRunID, now, s.candidateLimit)
				if err != nil {
					return err
				}
				if len(nodes) == 0 {
					break
				}
				classifiedFresh := false
				for _, node := range nodes {
					fits, failures := resource.Fits(s.capacity, node.Resources)
					if !fits {
						detail := resource.ExplainFailures(failures)
						if node.WaitReason != harnessmodel.WaitResource || node.WaitDetail != detail {
							if err := tx.SetReadyWait(ctx, node.NodeRunID, harnessmodel.WaitResource, detail, now); err != nil {
								return err
							}
							classifiedFresh = true
						}
						continue
					}
					if node.WaitReason != harnessmodel.WaitNone || node.WaitDetail != "" {
						if err := tx.SetReadyWait(ctx, node.NodeRunID, harnessmodel.WaitNone, "", now); err != nil {
							return err
						}
						node.WaitReason = harnessmodel.WaitNone
						node.WaitDetail = ""
					}
					if err := tx.RecordWorkflowService(ctx, lane.WorkflowRunID, now); err != nil {
						return err
					}
					decision = Decision{Node: node}
					found = true
					return nil
				}
				// Because unclassified rows sort before rows with wait_reason, if a
				// full batch contains only already-classified blocked nodes there is
				// no hidden unclassified candidate behind this batch.
				if !classifiedFresh {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return Decision{}, false, err
	}
	return decision, found, nil
}

func (s *Scheduler) ExplainNode(ctx context.Context, nodeRunID harnessmodel.NodeRunID) (harnessmodel.NodeExplanation, error) {
	now := s.now().UTC()
	var explanation harnessmodel.NodeExplanation
	err := s.store.View(ctx, func(reader harnessstore.Reader) error {
		nr, err := reader.GetNodeRun(ctx, nodeRunID)
		if err != nil {
			return err
		}
		explanation = harnessmodel.NodeExplanation{NodeRunID: nr.ID, State: nr.State, RemainingDependencies: nr.RemainingDependencies}
		switch nr.State {
		case harnessmodel.NodePendingDependencies:
			explanation.Reason = harnessmodel.WaitDependency
			explanation.Detail = fmt.Sprintf("remaining required dependencies: %d", nr.RemainingDependencies)
			return nil
		case harnessmodel.NodeReady:
			ready, err := reader.GetReadyNode(ctx, nr.ID)
			if err != nil {
				return fmt.Errorf("READY node %s missing scheduler projection: %w", nr.ID, err)
			}
			if !ready.NotBefore.IsZero() && ready.NotBefore.After(now) {
				explanation.Reason = harnessmodel.WaitNotBefore
				explanation.Detail = "eligible at " + ready.NotBefore.UTC().Format(time.RFC3339Nano)
				return nil
			}
			if ready.WaitReason != harnessmodel.WaitNone {
				explanation.Reason = ready.WaitReason
				explanation.Detail = ready.WaitDetail
				return nil
			}
			fits, failures := resource.Fits(s.capacity, ready.Resources)
			if !fits {
				explanation.Reason = harnessmodel.WaitResource
				explanation.Detail = resource.ExplainFailures(failures)
				return nil
			}
			explanation.Reason = harnessmodel.WaitFairness
			explanation.Detail = "READY and feasible; waiting for fair scheduler selection"
			return nil
		default:
			return nil
		}
	})
	return explanation, err
}
