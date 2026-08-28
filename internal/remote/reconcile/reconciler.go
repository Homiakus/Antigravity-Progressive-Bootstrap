package reconcile

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

type DriftKind string

const (
	DriftMissingCockpitInstance DriftKind = "missing_cockpit_instance"
	DriftAccountMismatch        DriftKind = "account_mismatch"
	DriftWorkingDirMismatch     DriftKind = "working_dir_mismatch"
	DriftProcessStopped         DriftKind = "process_stopped"
	DriftPIDChanged             DriftKind = "pid_changed"
	DriftBridgeUnavailable      DriftKind = "bridge_unavailable"
	DriftBridgePIDMismatch      DriftKind = "bridge_pid_mismatch"
	DriftWorkspaceMissing       DriftKind = "workspace_missing"
	DriftConversationMissing    DriftKind = "conversation_missing"
)

type Drift struct {
	Kind       DriftKind             `json:"kind"`
	InstanceID model.InstanceID      `json:"instanceId"`
	SessionID  model.RemoteSessionID `json:"sessionId,omitempty"`
	Detail     string                `json:"detail"`
}

type ConversationObservation struct {
	Busy bool
}

type BridgeObservation struct {
	Authenticated    bool
	PID              int
	BootNonce        string
	WorkspaceFolders []string
	Conversations    map[string]ConversationObservation
}

type BridgeObserver interface {
	Observe(context.Context, string) (BridgeObservation, error)
}

type Store interface {
	ListInstances(context.Context) ([]model.InstanceMirror, error)
	UpsertInstance(context.Context, model.InstanceMirror) error
	ListSessionsByInstance(context.Context, model.InstanceID, bool) ([]model.RemoteSession, error)
	GetConversation(context.Context, model.ConversationID) (model.Conversation, error)
	UpdateSessionStates(context.Context, model.RemoteSessionID, model.SessionDesiredState, model.SessionObservedState, time.Time) error
}

type CockpitClient interface {
	ListInstances(context.Context) ([]cockpit.Instance, error)
}

type Options struct {
	Store    Store
	Cockpit  CockpitClient
	Bridge   BridgeObserver
	Now      func() time.Time
}

type Reconciler struct {
	store   Store
	cockpit CockpitClient
	bridge  BridgeObserver
	now     func() time.Time
}

type Report struct {
	Instances int     `json:"instances"`
	Sessions  int     `json:"sessions"`
	Drifts    []Drift `json:"drifts"`
}

func New(opts Options) (*Reconciler, error) {
	if opts.Store == nil || opts.Cockpit == nil || opts.Bridge == nil {
		return nil, fmt.Errorf("remote reconciler store, Cockpit client and Bridge observer are required")
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Reconciler{store: opts.Store, cockpit: opts.Cockpit, bridge: opts.Bridge, now: now}, nil
}

func (r *Reconciler) ReconcileAll(ctx context.Context) (Report, error) {
	mirrors, err := r.store.ListInstances(ctx)
	if err != nil {
		return Report{}, err
	}
	observed, err := r.cockpit.ListInstances(ctx)
	if err != nil {
		return Report{}, err
	}
	byID := make(map[string]cockpit.Instance, len(observed))
	for _, instance := range observed {
		byID[instance.ID] = instance
	}
	sort.Slice(mirrors, func(i, j int) bool { return mirrors[i].ID < mirrors[j].ID })
	report := Report{Instances: len(mirrors)}
	for _, mirror := range mirrors {
		sessions, err := r.store.ListSessionsByInstance(ctx, mirror.ID, false)
		if err != nil {
			return report, err
		}
		report.Sessions += len(sessions)
		drifts, err := r.reconcileInstance(ctx, mirror, sessions, byID)
		if err != nil {
			return report, err
		}
		report.Drifts = append(report.Drifts, drifts...)
	}
	return report, nil
}

func (r *Reconciler) reconcileInstance(ctx context.Context, mirror model.InstanceMirror, sessions []model.RemoteSession, cockpitByID map[string]cockpit.Instance) ([]Drift, error) {
	now := r.now().UTC()
	actual, ok := cockpitByID[string(mirror.ID)]
	if !ok {
		drift := Drift{Kind: DriftMissingCockpitInstance, InstanceID: mirror.ID, Detail: "instance is persisted but absent from Cockpit"}
		mirror.ObservedState = model.InstanceStopped
		mirror.PID = 0
		mirror.LastError = drift.Detail
		mirror.LastReconciledAt = now
		if err := r.store.UpsertInstance(ctx, mirror); err != nil {
			return nil, err
		}
		if err := r.setSessions(ctx, sessions, model.SessionDegraded, now); err != nil {
			return nil, err
		}
		return []Drift{drift}, nil
	}

	drifts := make([]Drift, 0, 4)
	fatalIdentityDrift := false
	if actual.BindAccountID == nil || *actual.BindAccountID != mirror.AccountID {
		got := "<unbound>"
		if actual.BindAccountID != nil {
			got = *actual.BindAccountID
		}
		drifts = append(drifts, Drift{Kind: DriftAccountMismatch, InstanceID: mirror.ID, Detail: fmt.Sprintf("expected account %q, observed %q", mirror.AccountID, got)})
		fatalIdentityDrift = true
	}
	if !samePath(actual.WorkingDir, mirror.WorkingDir) {
		drifts = append(drifts, Drift{Kind: DriftWorkingDirMismatch, InstanceID: mirror.ID, Detail: fmt.Sprintf("expected working_dir %q, observed %q", mirror.WorkingDir, actual.WorkingDir)})
		fatalIdentityDrift = true
	}
	if fatalIdentityDrift {
		mirror.ObservedState = model.InstanceDegraded
		mirror.LastError = "Cockpit instance identity drift"
		mirror.LastReconciledAt = now
		mirror.PID = instancePID(actual)
		if err := r.store.UpsertInstance(ctx, mirror); err != nil {
			return nil, err
		}
		if err := r.setSessions(ctx, sessions, model.SessionNeedsAttention, now); err != nil {
			return nil, err
		}
		return drifts, nil
	}

	if mirror.DesiredState == model.InstanceDesiredRunning && !actual.Running {
		drift := Drift{Kind: DriftProcessStopped, InstanceID: mirror.ID, Detail: "instance is desired RUNNING but Cockpit reports stopped"}
		drifts = append(drifts, drift)
		mirror.ObservedState = model.InstanceStopped
		mirror.PID = 0
		mirror.LastError = drift.Detail
		mirror.LastReconciledAt = now
		if err := r.store.UpsertInstance(ctx, mirror); err != nil {
			return nil, err
		}
		if err := r.setSessions(ctx, sessions, model.SessionDegraded, now); err != nil {
			return nil, err
		}
		return drifts, nil
	}
	if !actual.Running {
		mirror.ObservedState = model.InstanceStopped
		mirror.PID = 0
		mirror.LastError = ""
		mirror.LastReconciledAt = now
		return drifts, r.store.UpsertInstance(ctx, mirror)
	}

	actualPID := instancePID(actual)
	if mirror.PID != 0 && actualPID != 0 && mirror.PID != actualPID {
		drifts = append(drifts, Drift{Kind: DriftPIDChanged, InstanceID: mirror.ID, Detail: fmt.Sprintf("persisted pid %d changed to %d", mirror.PID, actualPID)})
	}
	mirror.PID = actualPID
	mirror.ObservedState = model.InstanceProcessRunning
	mirror.LastError = ""

	bridge, bridgeErr := r.bridge.Observe(ctx, actual.ID)
	if bridgeErr != nil || !bridge.Authenticated {
		detail := "authenticated Bridge is unavailable"
		if bridgeErr != nil {
			detail = bridgeErr.Error()
		}
		drifts = append(drifts, Drift{Kind: DriftBridgeUnavailable, InstanceID: mirror.ID, Detail: detail})
		mirror.ObservedState = model.InstanceDegraded
		mirror.LastError = detail
		mirror.LastReconciledAt = now
		if err := r.store.UpsertInstance(ctx, mirror); err != nil {
			return nil, err
		}
		if err := r.setSessions(ctx, sessions, model.SessionDegraded, now); err != nil {
			return nil, err
		}
		return drifts, nil
	}
	if bridge.PID != 0 && actualPID != 0 && bridge.PID != actualPID {
		drift := Drift{Kind: DriftBridgePIDMismatch, InstanceID: mirror.ID, Detail: fmt.Sprintf("Bridge pid %d does not match Cockpit pid %d", bridge.PID, actualPID)}
		drifts = append(drifts, drift)
		mirror.ObservedState = model.InstanceDegraded
		mirror.LastError = drift.Detail
		mirror.LastReconciledAt = now
		if err := r.store.UpsertInstance(ctx, mirror); err != nil {
			return nil, err
		}
		if err := r.setSessions(ctx, sessions, model.SessionNeedsAttention, now); err != nil {
			return nil, err
		}
		return drifts, nil
	}

	mirror.ObservedState = model.InstanceReady
	mirror.BridgeID = bridge.BootNonce
	mirror.LastError = ""
	mirror.LastReconciledAt = now
	if err := r.store.UpsertInstance(ctx, mirror); err != nil {
		return nil, err
	}
	for _, session := range sessions {
		state := desiredObservedState(session)
		if !containsPath(bridge.WorkspaceFolders, session.WorkspacePath) {
			drifts = append(drifts, Drift{Kind: DriftWorkspaceMissing, InstanceID: mirror.ID, SessionID: session.ID, Detail: fmt.Sprintf("workspace %q is not open in IDE", session.WorkspacePath)})
			state = model.SessionNeedsAttention
		} else {
			conversation, err := r.store.GetConversation(ctx, session.ConversationID)
			if err != nil {
				if !errors.Is(err, remotestore.ErrNotFound) {
					return nil, err
				}
				drifts = append(drifts, Drift{Kind: DriftConversationMissing, InstanceID: mirror.ID, SessionID: session.ID, Detail: "conversation mirror is missing"})
				state = model.SessionNeedsAttention
			} else if observedConversation, ok := bridge.Conversations[conversation.ProviderConversationID]; !ok {
				drifts = append(drifts, Drift{Kind: DriftConversationMissing, InstanceID: mirror.ID, SessionID: session.ID, Detail: fmt.Sprintf("provider conversation %q is absent", conversation.ProviderConversationID)})
				state = model.SessionNeedsAttention
			} else if session.DesiredState == model.SessionDesiredReady && observedConversation.Busy {
				state = model.SessionRunning
			}
		}
		if err := r.store.UpdateSessionStates(ctx, session.ID, session.DesiredState, state, now); err != nil {
			return nil, err
		}
	}
	return drifts, nil
}

func (r *Reconciler) setSessions(ctx context.Context, sessions []model.RemoteSession, state model.SessionObservedState, now time.Time) error {
	for _, session := range sessions {
		next := state
		if session.DesiredState == model.SessionDesiredClosed {
			next = model.SessionClosed
		}
		if err := r.store.UpdateSessionStates(ctx, session.ID, session.DesiredState, next, now); err != nil {
			return err
		}
	}
	return nil
}

func desiredObservedState(session model.RemoteSession) model.SessionObservedState {
	switch session.DesiredState {
	case model.SessionDesiredPaused:
		return model.SessionPaused
	case model.SessionDesiredClosed:
		return model.SessionClosed
	default:
		return model.SessionReady
	}
}

func instancePID(instance cockpit.Instance) int {
	if instance.LastPID == nil {
		return 0
	}
	return int(*instance.LastPID)
}

func containsPath(items []string, want string) bool {
	for _, item := range items {
		if samePath(item, want) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	if strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
