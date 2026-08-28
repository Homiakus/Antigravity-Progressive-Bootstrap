package account

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/cockpit"
	"github.com/homiakus/agctl/internal/remote/model"
	remotestore "github.com/homiakus/agctl/internal/remote/store"
)

var (
	ErrAccountNotFound = errors.New("remote account: account not found")
	ErrAccountDisabled = errors.New("remote account: account disabled")
	ErrInstanceNotFound = errors.New("remote account: instance not found")
	ErrActiveSessions = errors.New("remote account: active sessions require handoff")
	ErrObservedMismatch = errors.New("remote account: persisted and Cockpit account mismatch")
)

type Store interface {
	GetInstance(context.Context, model.InstanceID) (model.InstanceMirror, error)
	ListSessionsByInstance(context.Context, model.InstanceID, bool) ([]model.RemoteSession, error)
}

type CockpitClient interface {
	ListAccounts(context.Context) ([]cockpit.Account, error)
	ListInstances(context.Context) ([]cockpit.Instance, error)
}

type Impact struct {
	SessionID      model.RemoteSessionID `json:"sessionId"`
	RepositoryID   model.RepositoryID    `json:"repositoryId"`
	ConversationID model.ConversationID  `json:"conversationId"`
	WorkspacePath  string                `json:"workspacePath"`
	ObservedState  model.SessionObservedState `json:"observedState"`
}

type SwitchPlan struct {
	InstanceID              model.InstanceID `json:"instanceId"`
	CurrentAccountID        string           `json:"currentAccountId,omitempty"`
	TargetAccount           cockpit.Account  `json:"targetAccount"`
	Running                  bool             `json:"running"`
	NoOp                     bool             `json:"noOp"`
	RequiresColdRestart      bool             `json:"requiresColdRestart"`
	RequiresCheckpoint       bool             `json:"requiresCheckpoint"`
	RequiresHandoff          bool             `json:"requiresHandoff"`
	PersistedAccountMismatch bool             `json:"persistedAccountMismatch"`
	Impacts                  []Impact         `json:"impacts"`
}

type Service struct {
	store   Store
	cockpit CockpitClient
}

func New(store Store, cockpitClient CockpitClient) (*Service, error) {
	if store == nil || cockpitClient == nil {
		return nil, fmt.Errorf("remote account store and Cockpit client are required")
	}
	return &Service{store: store, cockpit: cockpitClient}, nil
}

func (s *Service) Accounts(ctx context.Context) ([]cockpit.Account, error) {
	accounts, err := s.cockpit.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(accounts, func(i, j int) bool {
		if accounts[i].Email == accounts[j].Email {
			return accounts[i].ID < accounts[j].ID
		}
		return accounts[i].Email < accounts[j].Email
	})
	return accounts, nil
}

func (s *Service) PlanSwitch(ctx context.Context, instanceID model.InstanceID, targetAccountID string) (SwitchPlan, error) {
	if strings.TrimSpace(string(instanceID)) == "" || strings.TrimSpace(targetAccountID) == "" {
		return SwitchPlan{}, fmt.Errorf("instance id and target account id are required")
	}
	accounts, err := s.cockpit.ListAccounts(ctx)
	if err != nil {
		return SwitchPlan{}, err
	}
	var target cockpit.Account
	foundTarget := false
	for _, account := range accounts {
		if account.ID == targetAccountID {
			target = account
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		return SwitchPlan{}, fmt.Errorf("account %s: %w", targetAccountID, ErrAccountNotFound)
	}
	if target.Disabled {
		return SwitchPlan{}, fmt.Errorf("account %s: %w", targetAccountID, ErrAccountDisabled)
	}

	instances, err := s.cockpit.ListInstances(ctx)
	if err != nil {
		return SwitchPlan{}, err
	}
	var actual cockpit.Instance
	foundInstance := false
	for _, instance := range instances {
		if instance.ID == string(instanceID) {
			actual = instance
			foundInstance = true
			break
		}
	}
	if !foundInstance {
		return SwitchPlan{}, fmt.Errorf("instance %s: %w", instanceID, ErrInstanceNotFound)
	}

	persisted, err := s.store.GetInstance(ctx, instanceID)
	if err != nil && !errors.Is(err, remotestore.ErrNotFound) {
		return SwitchPlan{}, err
	}
	current := ""
	if actual.BindAccountID != nil {
		current = *actual.BindAccountID
	}
	mismatch := err == nil && persisted.AccountID != "" && persisted.AccountID != current

	sessions, err := s.store.ListSessionsByInstance(ctx, instanceID, false)
	if err != nil {
		return SwitchPlan{}, err
	}
	impacts := make([]Impact, 0, len(sessions))
	for _, session := range sessions {
		if session.DesiredState == model.SessionDesiredClosed || session.ObservedState == model.SessionClosed {
			continue
		}
		impacts = append(impacts, Impact{
			SessionID: session.ID, RepositoryID: session.RepositoryID, ConversationID: session.ConversationID,
			WorkspacePath: session.WorkspacePath, ObservedState: session.ObservedState,
		})
	}
	sort.Slice(impacts, func(i, j int) bool { return impacts[i].SessionID < impacts[j].SessionID })
	noOp := current == targetAccountID && !mismatch
	return SwitchPlan{
		InstanceID: instanceID,
		CurrentAccountID: current,
		TargetAccount: target,
		Running: actual.Running,
		NoOp: noOp,
		RequiresColdRestart: !noOp && actual.Running,
		RequiresCheckpoint: !noOp && len(impacts) > 0,
		RequiresHandoff: !noOp && len(impacts) > 0,
		PersistedAccountMismatch: mismatch,
		Impacts: impacts,
	}, nil
}

func (p SwitchPlan) ValidateExecution(allowHandoff bool) error {
	if p.NoOp {
		return nil
	}
	if p.PersistedAccountMismatch {
		return ErrObservedMismatch
	}
	if len(p.Impacts) > 0 && !allowHandoff {
		return fmt.Errorf("%d active sessions affected: %w", len(p.Impacts), ErrActiveSessions)
	}
	return nil
}
