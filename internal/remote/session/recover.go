package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
)

type recoveryStore interface {
	GetSession(context.Context, model.RemoteSessionID) (model.RemoteSession, error)
	GetConversation(context.Context, model.ConversationID) (model.Conversation, error)
	UpsertInstance(context.Context, model.InstanceMirror) error
	UpdateSessionStates(context.Context, model.RemoteSessionID, model.SessionDesiredState, model.SessionObservedState, time.Time) error
}

// Recover re-establishes an authenticated live Bridge for a persisted session
// without persisting Bridge credentials. A running instance without an owned
// token is fail-closed unless restartRunning is explicitly enabled, in which
// case the exact Cockpit instance is stopped and managed-started with fresh
// credentials after account/workspace checks.
func (s *Service) Recover(ctx context.Context, sessionID model.RemoteSessionID, restartRunning bool) (model.RemoteSession, error) {
	if s == nil {
		return model.RemoteSession{}, fmt.Errorf("remote session service is nil")
	}
	store, ok := s.store.(recoveryStore)
	if !ok {
		return model.RemoteSession{}, fmt.Errorf("remote session store does not support recovery")
	}
	session, err := store.GetSession(ctx, sessionID)
	if err != nil {
		return model.RemoteSession{}, err
	}
	if session.DesiredState == model.SessionDesiredClosed || session.ObservedState == model.SessionClosed {
		return session, fmt.Errorf("session %s is closed", session.ID)
	}
	conversation, err := store.GetConversation(ctx, session.ConversationID)
	if err != nil {
		return model.RemoteSession{}, err
	}

	if live, ok := s.liveBridge(string(session.CockpitInstanceID)); ok && live.Client != nil {
		if health, healthErr := live.Client.Health(ctx); healthErr == nil && (health.InstanceID == "" || health.InstanceID == string(session.CockpitInstanceID)) {
			return session, nil
		}
	}

	instance, err := s.lookupCockpitInstance(ctx, string(session.CockpitInstanceID), session.CockpitAccountID)
	if err != nil {
		s.markRecoveryAttention(ctx, store, session, err)
		return model.RemoteSession{}, err
	}
	if !samePath(instance.WorkingDir, session.WorkspacePath) {
		err := fmt.Errorf("Cockpit workspace %q does not match persisted session %q: %w", instance.WorkingDir, session.WorkspacePath, ErrWorkspaceMismatch)
		s.markRecoveryAttention(ctx, store, session, err)
		return model.RemoteSession{}, err
	}
	if instance.Running {
		if !restartRunning {
			err := fmt.Errorf("instance %s is running without daemon-owned Bridge credentials: %w", instance.ID, ErrBridgeCredentialsUnavailable)
			s.markRecoveryAttention(ctx, store, session, err)
			return model.RemoteSession{}, err
		}
		if _, err := s.cockpit.StopInstance(ctx, instance.ID); err != nil {
			s.markRecoveryAttention(ctx, store, session, err)
			return model.RemoteSession{}, fmt.Errorf("stop unowned Cockpit instance %s: %w", instance.ID, err)
		}
		instance.Running = false
		instance.LastPID = nil
	}

	bootNonce, err := s.secrets.NewSecret(16)
	if err != nil {
		return model.RemoteSession{}, err
	}
	bridgeToken, err := s.secrets.NewSecret(32)
	if err != nil {
		return model.RemoteSession{}, err
	}
	started, err := s.cockpit.StartManagedInstance(ctx, instance.ID, cockpit.LaunchContext{InstanceID: instance.ID, BootNonce: bootNonce, BridgeToken: bridgeToken, BridgeRegistry: s.bridgeRegistry})
	if err != nil {
		s.markRecoveryAttention(ctx, store, session, err)
		return model.RemoteSession{}, err
	}
	rollback := true
	defer func() {
		if rollback {
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = s.cockpit.StopInstance(stopCtx, started.ID)
		}
	}()

	located, err := s.locator.Wait(ctx, instance.ID, bridgeToken)
	if err != nil {
		s.markRecoveryAttention(ctx, store, session, err)
		return model.RemoteSession{}, err
	}
	if err := validateRecoveredBridge(ctx, located, instance.ID, session.WorkspacePath, conversation.ProviderConversationID); err != nil {
		s.markRecoveryAttention(ctx, store, session, err)
		return model.RemoteSession{}, err
	}

	now := s.now().UTC()
	mirror := instanceMirror(started, session.CockpitAccountID, model.InstanceDesiredRunning, model.InstanceReady, now)
	mirror.BridgeID = located.Registration.BootNonce
	mirror.LastError = ""
	if err := store.UpsertInstance(ctx, mirror); err != nil {
		return model.RemoteSession{}, err
	}
	observed := model.SessionReady
	if session.DesiredState == model.SessionDesiredPaused {
		observed = model.SessionPaused
	}
	if err := store.UpdateSessionStates(ctx, session.ID, session.DesiredState, observed, now); err != nil {
		return model.RemoteSession{}, err
	}
	s.rememberBridge(instance.ID, located)
	rollback = false
	session.ObservedState = observed
	session.UpdatedAt = now
	return session, nil
}

func validateRecoveredBridge(ctx context.Context, located antigravityide.LocatedBridge, instanceID, workspacePath, providerConversationID string) error {
	if located.Client == nil {
		return fmt.Errorf("recovered Bridge has no client")
	}
	health, err := located.Client.Health(ctx)
	if err != nil {
		return err
	}
	if health.InstanceID != "" && health.InstanceID != instanceID {
		return fmt.Errorf("recovered Bridge instance %q does not match %q", health.InstanceID, instanceID)
	}
	caps, err := located.Client.Capabilities(ctx)
	if err != nil {
		return err
	}
	if !caps.ConversationList || !caps.ConversationFocus || !caps.ConversationSend {
		return fmt.Errorf("recovered Bridge lacks conversation control: %w", ErrCapability)
	}
	ideContext, err := located.Client.Context(ctx)
	if err != nil {
		return err
	}
	if !containsPath(ideContext.WorkspaceFolders, workspacePath) {
		return fmt.Errorf("recovered IDE does not own workspace %s: %w", workspacePath, ErrWorkspaceMismatch)
	}
	conversations, err := located.Client.ListConversations(ctx)
	if err != nil {
		return err
	}
	providerConversationID = strings.TrimSpace(providerConversationID)
	for _, conversation := range conversations {
		if conversation.ID == providerConversationID {
			return nil
		}
	}
	return fmt.Errorf("persisted conversation %q is not present in recovered IDE", providerConversationID)
}

func (s *Service) markRecoveryAttention(ctx context.Context, store recoveryStore, session model.RemoteSession, cause error) {
	if cause == nil || errors.Is(cause, context.Canceled) || errors.Is(cause, context.DeadlineExceeded) {
		return
	}
	_ = store.UpdateSessionStates(ctx, session.ID, session.DesiredState, model.SessionNeedsAttention, s.now().UTC())
}
