package router

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	"github.com/homiakus/agctl/internal/harness/provider/fault"
	"github.com/homiakus/agctl/internal/harness/provider/selector"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
	"github.com/homiakus/agctl/internal/harness/task"
)

var (
	// ErrReadOnlyRequired indicates an attempt to route an envelope that allows writes.
	ErrReadOnlyRequired = errors.New("read-only routing violation: envelope workspace is not marked read-only")
	// ErrNoViableProvider indicates no eligible provider candidate passed hard and soft filters.
	ErrNoViableProvider = errors.New("provider routing failed: no viable provider candidate available")
	// ErrExecutionFailed indicates read-only execution returned an error.
	ErrExecutionFailed = errors.New("read-only execution failed")
	// ErrInvalidRouteState indicates the route result was not in an active routing state.
	ErrInvalidRouteState = errors.New("invalid route state for execution")
)

// ReadOnlyExecutorFunc defines the execution callback for a routed read-only task.
// Implementations MUST NOT mutate the local worktree or execute state-modifying effects.
type ReadOnlyExecutorFunc func(ctx context.Context, env harnessmodel.TaskEnvelope, assignment harnessmodel.ProviderAssignment) (output string, tokensUsed int64, err error)

// Options configures the read-only router.
type Options struct {
	Policy         selector.Policy
	FaultPolicy    fault.Policy
	ReservationTTL time.Duration
	Now            func() time.Time
	IDGen          harnessmodel.IDGenerator
}

func (o *Options) normalize() {
	if o.Now == nil {
		o.Now = time.Now
	}
	if o.ReservationTTL <= 0 {
		o.ReservationTTL = 15 * time.Minute
	}
	if o.IDGen == nil {
		o.IDGen = &harnessmodel.TimeSortableIDGenerator{Now: o.Now}
	}
	if o.FaultPolicy.Validate() != nil {
		o.FaultPolicy = fault.DefaultPolicy()
	}
}

// RouteResult captures the complete explainable routing decision and durable reservations.
type RouteResult struct {
	Envelope          harnessmodel.TaskEnvelope
	Assignment        harnessmodel.ProviderAssignment
	Reservation       *harnessmodel.ProviderReservation
	Decision          selector.Decision
	SelectedCandidate selector.CandidateEvaluation
	RoutedAt          time.Time
}

// ExecutionResult records the completed outcome of a read-only routed execution.
type ExecutionResult struct {
	Route         RouteResult
	Success       bool
	Output        string
	TokensUsed    int64
	Duration      time.Duration
	Error         string
	Fault         *fault.Classification
	RetryAction   fault.RetryAction
	FaultDecision *fault.Decision
}

// Router orchestrates automatic read-only provider routing.
type Router struct {
	store harnessstore.Store
	opts  Options
}

// NewRouter creates a new automatic read-only provider router.
func NewRouter(store harnessstore.Store, opts Options) *Router {
	opts.normalize()
	return &Router{
		store: store,
		opts:  opts,
	}
}

// Route selects an optimal provider candidate and atomically commits the assignment
// and reservation in the SQLite store for a validated read-only TaskEnvelope.
func (r *Router) Route(ctx context.Context, env harnessmodel.TaskEnvelope, currentPlan []byte) (RouteResult, error) {
	if err := env.Validate(); err != nil {
		return RouteResult{}, fmt.Errorf("validate task envelope: %w", err)
	}

	// Invariant I-032 & I-005: Enforce read-only constraint before write routing.
	if !env.Workspace.ReadOnly {
		return RouteResult{}, fmt.Errorf("%w: task=%s rootPath=%s", ErrReadOnlyRequired, env.TaskID, env.Workspace.RootPath)
	}

	// Invariant I-030: Enforce cryptographic living-plan consistency.
	if err := task.CheckPlanDrift(env, currentPlan); err != nil {
		return RouteResult{}, err
	}

	now := r.opts.Now().UTC()
	req := task.ToSelectorRequest(env)
	storeSource := selector.StoreSource{Store: r.store}

	policy := r.opts.Policy
	if policy.Validate() != nil {
		policy = selector.DefaultPolicy()
	}

	decision, err := storeSource.Select(ctx, req, now, policy)
	if err != nil {
		return RouteResult{}, fmt.Errorf("selector evaluation failed: %w", err)
	}

	if decision.Selected == nil {
		return RouteResult{
			Envelope: env,
			Decision: decision,
			RoutedAt: now,
		}, fmt.Errorf("%w: %s", ErrNoViableProvider, decision.Rationale)
	}

	winner := *decision.Selected

	asnIDRaw, err := r.opts.IDGen.New(harnessmodel.IDProviderAssignment)
	if err != nil {
		return RouteResult{}, fmt.Errorf("generate assignment id: %w", err)
	}
	assignmentID := harnessmodel.ProviderAssignmentID(asnIDRaw)

	attemptID := env.AttemptID
	if attemptID == "" {
		attIDRaw, err := r.opts.IDGen.New(harnessmodel.IDAttempt)
		if err != nil {
			return RouteResult{}, fmt.Errorf("generate attempt id: %w", err)
		}
		attemptID = harnessmodel.AttemptID(attIDRaw)
	}

	assignment := harnessmodel.ProviderAssignment{
		ID:         assignmentID,
		AttemptID:  attemptID,
		AccountID:  winner.AccountID,
		ModelID:    winner.ModelID,
		SessionID:  winner.SessionDecision.SessionID,
		PlanDigest: env.PlanDigest,
		State:      harnessmodel.ProviderAssignmentActive,
		Revision:   1,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	var reservation *harnessmodel.ProviderReservation
	if env.MaxTokens > 0 && winner.NormalizedCapacity != nil {
		for _, w := range winner.NormalizedCapacity.Windows {
			if (w.Metric == harnessmodel.QuotaMetricTokens || w.Metric == harnessmodel.QuotaMetricFraction) && w.ID != "" {
				resIDRaw, err := r.opts.IDGen.New(harnessmodel.IDProviderReservation)
				if err != nil {
					return RouteResult{}, fmt.Errorf("generate reservation id: %w", err)
				}
				amount := float64(env.MaxTokens)
				if w.Metric == harnessmodel.QuotaMetricFraction {
					amount = 0.05 // default conservative fractional claim
				}
				reservation = &harnessmodel.ProviderReservation{
					ID:           harnessmodel.ProviderReservationID(resIDRaw),
					AssignmentID: assignmentID,
					AccountID:    winner.AccountID,
					WindowID:     w.ID,
					ModelID:      winner.ModelID,
					Metric:       w.Metric,
					Amount:       amount,
					State:        harnessmodel.ProviderReservationActive,
					Revision:     1,
					CreatedAt:    now,
					ExpiresAt:    now.Add(r.opts.ReservationTTL),
					UpdatedAt:    now,
				}
				break
			}
		}
	}

	// Atomically commit assignment and reservation in a single store transaction
	if err := r.store.Update(ctx, func(tx harnessstore.Tx) error {
		// Ensure attempt exists to satisfy FOREIGN KEY(attempt_id) REFERENCES attempts(id)
		if _, err := tx.GetAttempt(ctx, assignment.AttemptID); errors.Is(err, harnessstore.ErrNotFound) {
			uniqueSuffix := string(assignment.AttemptID)
			defID := harnessmodel.WorkflowDefinitionID("wfd_" + uniqueSuffix)
			wfrID := harnessmodel.WorkflowRunID("wfr_" + uniqueSuffix)
			nrID := harnessmodel.NodeRunID("nr_" + uniqueSuffix)
			nodeID := harnessmodel.NodeID("node_" + uniqueSuffix)

			if err := tx.CreateWorkflowDefinition(ctx, harnessmodel.WorkflowDefinition{
				ID: defID, Version: 1, Name: "standalone", CreatedAt: now,
				Nodes: []harnessmodel.NodeSpec{{ID: nodeID, Kind: harnessmodel.NodeKindAction, ExecutorKind: harnessmodel.ExecutorAgent}},
			}); err != nil {
				return fmt.Errorf("create standalone workflow def: %w", err)
			}
			if err := tx.CreateWorkflowRun(ctx, harnessmodel.WorkflowRun{
				ID: wfrID, DefinitionID: defID, DefinitionVersion: 1, State: harnessmodel.WorkflowRunning, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("create standalone workflow run: %w", err)
			}
			if err := tx.CreateGraphRevision(ctx, harnessmodel.GraphRevision{
				WorkflowRunID: wfrID, Number: 1, CreatedAt: now, Reason: "standalone-route",
			}); err != nil {
				return fmt.Errorf("create standalone graph revision: %w", err)
			}
			if err := tx.CreateNodeRun(ctx, harnessmodel.NodeRun{
				ID: nrID, WorkflowRunID: wfrID, NodeID: nodeID, GraphRevision: 1, Generation: 1, State: harnessmodel.NodeReady, CreatedAt: now, UpdatedAt: now,
			}); err != nil {
				return fmt.Errorf("create standalone node run: %w", err)
			}
			if _, err := tx.CreateNextAttempt(ctx, harnessmodel.Attempt{
				ID: assignment.AttemptID, NodeRunID: nrID, State: harnessmodel.AttemptCreated, CreatedAt: now,
			}); err != nil {
				return fmt.Errorf("create standalone attempt: %w", err)
			}
		}

		if err := tx.CreateProviderAssignment(ctx, assignment); err != nil {
			return fmt.Errorf("commit provider assignment: %w", err)
		}
		if reservation != nil {
			if err := tx.CreateProviderReservation(ctx, *reservation); err != nil {
				return fmt.Errorf("commit provider reservation: %w", err)
			}
		}
		return nil
	}); err != nil {
		return RouteResult{}, fmt.Errorf("atomic routing commit failed: %w", err)
	}

	return RouteResult{
		Envelope:          env,
		Assignment:        assignment,
		Reservation:       reservation,
		Decision:          decision,
		SelectedCandidate: winner,
		RoutedAt:          now,
	}, nil
}

// Execute safely runs a read-only executor against a verified route result, updating the
// durable ledger upon completion.
func (r *Router) Execute(ctx context.Context, route RouteResult, exec ReadOnlyExecutorFunc) (ExecutionResult, error) {
	if route.Assignment.ID == "" || route.Assignment.State != harnessmodel.ProviderAssignmentActive {
		return ExecutionResult{Route: route}, ErrInvalidRouteState
	}
	if !route.Envelope.Workspace.ReadOnly {
		return ExecutionResult{Route: route}, ErrReadOnlyRequired
	}

	startTime := r.opts.Now()
	output, tokensUsed, execErr := exec(ctx, route.Envelope, route.Assignment)
	duration := r.opts.Now().Sub(startTime)
	now := r.opts.Now().UTC()

	res := ExecutionResult{
		Route:      route,
		Success:    execErr == nil,
		Output:     output,
		TokensUsed: tokensUsed,
		Duration:   duration,
	}
	if execErr != nil {
		res.Error = execErr.Error()
		c := fault.Classify(execErr)
		res.Fault = &c

		circuitMgr := fault.NewCircuitManager(r.store)
		var currentCircuit *harnessmodel.ProviderCircuitState
		if err := r.store.View(ctx, func(reader harnessstore.Reader) error {
			cState, err := reader.GetProviderCircuitState(ctx, route.Assignment.AccountID, route.Assignment.ModelID)
			if err == nil {
				currentCircuit = &cState
			}
			return nil
		}); err == nil {
			dec, err := fault.Decide(fault.DecisionInput{
				Fault:                c,
				TotalAttempts:        1,
				SameProviderAttempts: 1,
				Circuit:              currentCircuit,
				Policy:               r.opts.FaultPolicy,
				Now:                  now,
			})
			if err == nil {
				res.RetryAction = dec.Action
				res.FaultDecision = &dec
				if dec.TripCircuit {
					_, _ = circuitMgr.RecordFailure(ctx, route.Assignment.AccountID, route.Assignment.ModelID, r.opts.FaultPolicy.CircuitFailureThreshold, r.opts.FaultPolicy.CircuitCooldown, now)
				}
			}
		}
	} else {
		// On success, reset consecutive failures
		circuitMgr := fault.NewCircuitManager(r.store)
		_ = circuitMgr.RecordSuccess(ctx, route.Assignment.AccountID, route.Assignment.ModelID, now)
	}

	// Settle reservation FIRST, then assignment, maintaining active reservation invariants
	updateErr := r.store.Update(ctx, func(tx harnessstore.Tx) error {
		if route.Reservation != nil {
			updatedRes := *route.Reservation
			updatedRes.State = harnessmodel.ProviderReservationReleased
			updatedRes.Revision = route.Reservation.Revision + 1
			updatedRes.UpdatedAt = now

			if err := tx.CompareAndSwapProviderReservation(ctx, route.Reservation.Revision, updatedRes); err != nil {
				return fmt.Errorf("settle provider reservation CAS: %w", err)
			}
		}

		finalState := harnessmodel.ProviderAssignmentCompleted
		if execErr != nil {
			finalState = harnessmodel.ProviderAssignmentReleased
		}

		updatedAsn := route.Assignment
		updatedAsn.State = finalState
		updatedAsn.Revision = route.Assignment.Revision + 1
		updatedAsn.UpdatedAt = now

		if err := tx.CompareAndSwapProviderAssignment(ctx, route.Assignment.Revision, updatedAsn); err != nil {
			return fmt.Errorf("settle provider assignment CAS: %w", err)
		}

		if tokensUsed > 0 && route.Reservation != nil {
			sample := harnessmodel.ProviderUsageSample{
				Key:           fmt.Sprintf("usamp_%d_%s", now.UnixNano(), route.Assignment.ID),
				AssignmentID:  route.Assignment.ID,
				ReservationID: route.Reservation.ID,
				AccountID:     route.Assignment.AccountID,
				ModelID:       route.Assignment.ModelID,
				Metric:        route.Reservation.Metric,
				Amount:        float64(tokensUsed),
				ObservedAt:    now,
				CreatedAt:     now,
			}
			_, _, _ = tx.PutProviderUsageSample(ctx, sample)
		}
		return nil
	})

	if updateErr != nil {
		return res, fmt.Errorf("settle route execution ledger: %w", updateErr)
	}

	if execErr != nil {
		return res, fmt.Errorf("%w: %v", ErrExecutionFailed, execErr)
	}

	return res, nil
}
