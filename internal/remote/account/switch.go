package account

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
	runtimectl "github.com/homiakus/agctl/internal/remote/runtime"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

var (
	ErrRuntimeUnavailable    = errors.New("remote account: managed runtime unavailable")
	ErrCheckpointUnavailable = errors.New("remote account: conversation checkpoint unavailable")
	ErrPostSwitchWorkspace   = errors.New("remote account: workspace missing after account switch")
)

type MutationClient interface {
	BindAccount(context.Context, string, string) (cockpit.Instance, error)
}

type ExecutionStore interface {
	GetInstance(context.Context, model.InstanceID) (model.InstanceMirror, error)
	UpsertInstance(context.Context, model.InstanceMirror) error
	GetSession(context.Context, model.RemoteSessionID) (model.RemoteSession, error)
	GetConversation(context.Context, model.ConversationID) (model.Conversation, error)
	UpsertConversation(context.Context, model.Conversation) error
	UpdateSessionStates(context.Context, model.RemoteSessionID, model.SessionDesiredState, model.SessionObservedState, time.Time) error
	UpdateSessionAccount(context.Context, model.RemoteSessionID, string, time.Time) error
}

type Checkpoint struct{ Summary string }

type Checkpointer interface {
	Checkpoint(context.Context, model.RemoteSession, model.Conversation, antigravityide.Client) (Checkpoint, error)
}

type SwitchOptions struct {
	AllowHandoff bool
	Checkpointer Checkpointer
}

type Continuation struct {
	SessionID               model.RemoteSessionID      `json:"sessionId"`
	OldProviderConversation string                     `json:"oldProviderConversation"`
	NewProviderConversation string                     `json:"newProviderConversation,omitempty"`
	State                   model.SessionObservedState `json:"state"`
}

type SwitchResult struct {
	Plan          SwitchPlan     `json:"plan"`
	Continuations []Continuation `json:"continuations,omitempty"`
}

type savedCheckpoint struct {
	session      model.RemoteSession
	conversation model.Conversation
	checkpoint   Checkpoint
}

func (s *Service) WithRuntime(manager runtimectl.Manager) *Service {
	s.runtime = manager
	return s
}

func (s *Service) Switch(ctx context.Context, instanceID model.InstanceID, targetAccountID string, options SwitchOptions) (SwitchResult, error) {
	plan, err := s.PlanSwitch(ctx, instanceID, targetAccountID)
	if err != nil { return SwitchResult{}, err }
	if err := plan.ValidateExecution(options.AllowHandoff); err != nil { return SwitchResult{Plan: plan}, err }
	if plan.NoOp { return SwitchResult{Plan: plan}, nil }
	if s.runtime == nil { return SwitchResult{Plan: plan}, ErrRuntimeUnavailable }
	mutation, ok := s.cockpit.(MutationClient)
	if !ok { return SwitchResult{Plan: plan}, fmt.Errorf("Cockpit client cannot bind accounts") }
	execStore, ok := s.store.(ExecutionStore)
	if !ok { return SwitchResult{Plan: plan}, fmt.Errorf("remote account store lacks execution methods") }
	actual, err := s.findCockpitInstance(ctx, instanceID)
	if err != nil { return SwitchResult{Plan: plan}, err }

	checkpoints := make([]savedCheckpoint, 0, len(plan.Impacts))
	if len(plan.Impacts) > 0 {
		if options.Checkpointer == nil { return SwitchResult{Plan: plan}, ErrCheckpointUnavailable }
		bridge, ok := s.runtime.Bridge(string(instanceID))
		if !ok { return SwitchResult{Plan: plan}, fmt.Errorf("instance %s: %w", instanceID, runtimectl.ErrBridgeCredentialsUnavailable) }
		for _, impact := range plan.Impacts {
			session, err := execStore.GetSession(ctx, impact.SessionID)
			if err != nil { return SwitchResult{Plan: plan}, err }
			conversation, err := execStore.GetConversation(ctx, impact.ConversationID)
			if err != nil { return SwitchResult{Plan: plan}, err }
			checkpoint, err := options.Checkpointer.Checkpoint(ctx, session, conversation, bridge.Client)
			if err != nil { return SwitchResult{Plan: plan}, fmt.Errorf("checkpoint session %s: %w", session.ID, err) }
			if strings.TrimSpace(checkpoint.Summary) == "" { return SwitchResult{Plan: plan}, fmt.Errorf("session %s produced empty checkpoint: %w", session.ID, ErrCheckpointUnavailable) }
			checkpoints = append(checkpoints, savedCheckpoint{session: session, conversation: conversation, checkpoint: checkpoint})
		}
	}

	wasRunning := actual.Running
	if wasRunning {
		if _, err := s.runtime.Stop(ctx, actual.ID); err != nil { return SwitchResult{Plan: plan}, fmt.Errorf("stop instance before account switch: %w", err) }
	}
	bound, err := mutation.BindAccount(ctx, actual.ID, targetAccountID)
	if err != nil { return SwitchResult{Plan: plan}, fmt.Errorf("bind Cockpit account: %w", err) }

	now := time.Now().UTC()
	mirror, mirrorErr := execStore.GetInstance(ctx, instanceID)
	if mirrorErr != nil && !errors.Is(mirrorErr, remotestore.ErrNotFound) { return SwitchResult{Plan: plan}, mirrorErr }
	if mirrorErr != nil { mirror = model.InstanceMirror{ID: instanceID, Name: bound.Name, UserDataDir: bound.UserDataDir, WorkingDir: bound.WorkingDir} }
	mirror.AccountID = targetAccountID
	mirror.PID = 0
	mirror.BridgeID = ""
	mirror.LastError = ""
	mirror.LastReconciledAt = now
	mirror.ObservedState = model.InstanceStopped
	if wasRunning || len(checkpoints) > 0 { mirror.DesiredState = model.InstanceDesiredRunning } else { mirror.DesiredState = model.InstanceDesiredStopped }
	if err := execStore.UpsertInstance(ctx, mirror); err != nil { return SwitchResult{Plan: plan}, err }
	if !wasRunning && len(checkpoints) == 0 { return SwitchResult{Plan: plan}, nil }

	started, bridge, err := s.runtime.Start(ctx, bound)
	if err != nil {
		mirror.ObservedState = model.InstanceDegraded
		mirror.LastError = err.Error()
		mirror.LastReconciledAt = time.Now().UTC()
		_ = execStore.UpsertInstance(context.Background(), mirror)
		s.markNeedsAttention(context.Background(), execStore, checkpoints)
		return SwitchResult{Plan: plan}, fmt.Errorf("restart instance after account switch: %w", err)
	}

	ideContext, err := bridge.Client.Context(ctx)
	if err != nil { s.markNeedsAttention(context.Background(), execStore, checkpoints); return SwitchResult{Plan: plan}, err }
	for _, saved := range checkpoints {
		if !containsWorkspace(ideContext.WorkspaceFolders, saved.session.WorkspacePath) {
			s.markNeedsAttention(context.Background(), execStore, checkpoints)
			return SwitchResult{Plan: plan}, fmt.Errorf("workspace %s: %w", saved.session.WorkspacePath, ErrPostSwitchWorkspace)
		}
	}

	mirror.ObservedState = model.InstanceReady
	mirror.LastError = ""
	mirror.LastReconciledAt = time.Now().UTC()
	if started.LastPID != nil { mirror.PID = int(*started.LastPID) }
	mirror.BridgeID = bridge.Registration.BootNonce
	if err := execStore.UpsertInstance(ctx, mirror); err != nil { return SwitchResult{Plan: plan}, err }

	result := SwitchResult{Plan: plan, Continuations: make([]Continuation, 0, len(checkpoints))}
	for _, saved := range checkpoints {
		continuation := Continuation{SessionID: saved.session.ID, OldProviderConversation: saved.conversation.ProviderConversationID}
		if saved.session.DesiredState == model.SessionDesiredPaused {
			now := time.Now().UTC()
			_ = execStore.UpdateSessionAccount(ctx, saved.session.ID, targetAccountID, now)
			_ = execStore.UpdateSessionStates(ctx, saved.session.ID, saved.session.DesiredState, model.SessionNeedsAttention, now)
			continuation.State = model.SessionNeedsAttention
			result.Continuations = append(result.Continuations, continuation)
			continue
		}
		newConversation, err := bridge.Client.CreateConversation(ctx)
		if err != nil { s.markOneNeedsAttention(context.Background(), execStore, saved.session); return result, fmt.Errorf("create continuation conversation for %s: %w", saved.session.ID, err) }
		if err := bridge.Client.FocusConversation(ctx, newConversation.ID); err != nil { s.markOneNeedsAttention(context.Background(), execStore, saved.session); return result, fmt.Errorf("focus continuation conversation for %s: %w", saved.session.ID, err) }
		bootstrap := "Continue this remote session from the following checkpoint. Preserve the established task intent and verify current repository state before making new changes.\n\n" + saved.checkpoint.Summary
		if err := bridge.Client.SendMessage(ctx, newConversation.ID, bootstrap); err != nil { s.markOneNeedsAttention(context.Background(), execStore, saved.session); return result, fmt.Errorf("bootstrap continuation conversation for %s: %w", saved.session.ID, err) }
		now := time.Now().UTC()
		saved.conversation.ProviderConversationID = newConversation.ID
		if strings.TrimSpace(newConversation.Title) != "" { saved.conversation.Title = newConversation.Title }
		saved.conversation.State = model.ConversationActive
		saved.conversation.LastActivityAt = now
		saved.conversation.UpdatedAt = now
		if err := execStore.UpsertConversation(ctx, saved.conversation); err != nil { return result, err }
		if err := execStore.UpdateSessionAccount(ctx, saved.session.ID, targetAccountID, now); err != nil { return result, err }
		observed := model.SessionReady
		if saved.session.ObservedState == model.SessionRunning { observed = model.SessionRunning }
		if err := execStore.UpdateSessionStates(ctx, saved.session.ID, saved.session.DesiredState, observed, now); err != nil { return result, err }
		continuation.NewProviderConversation = newConversation.ID
		continuation.State = observed
		result.Continuations = append(result.Continuations, continuation)
	}
	return result, nil
}

func (s *Service) findCockpitInstance(ctx context.Context, instanceID model.InstanceID) (cockpit.Instance, error) {
	instances, err := s.cockpit.ListInstances(ctx)
	if err != nil { return cockpit.Instance{}, err }
	for _, instance := range instances { if instance.ID == string(instanceID) { return instance, nil } }
	return cockpit.Instance{}, ErrInstanceNotFound
}

func (s *Service) markNeedsAttention(ctx context.Context, store ExecutionStore, checkpoints []savedCheckpoint) {
	for _, saved := range checkpoints { s.markOneNeedsAttention(ctx, store, saved.session) }
}

func (s *Service) markOneNeedsAttention(ctx context.Context, store ExecutionStore, session model.RemoteSession) {
	_ = store.UpdateSessionStates(ctx, session.ID, session.DesiredState, model.SessionNeedsAttention, time.Now().UTC())
}

func containsWorkspace(items []string, want string) bool {
	want = filepath.Clean(want)
	for _, item := range items {
		item = filepath.Clean(item)
		if runtime.GOOS == "windows" { if strings.EqualFold(item, want) { return true } } else if item == want { return true }
	}
	return false
}
