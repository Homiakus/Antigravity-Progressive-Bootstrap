package session

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/remote/model"
)

type ResolveAction string

const (
	ResolveReuse     ResolveAction = "REUSE"
	ResolveCreateNew ResolveAction = "CREATE_NEW"
)

type ResolveRequest struct {
	RepositoryID              model.RepositoryID
	AccountID                 string
	WorkspacePath             string
	AllowWorkspaceReplacement bool
}

type ResolveDecision struct {
	Action     ResolveAction
	InstanceID model.InstanceID
	Reason     string
}

type ResolverStore interface {
	ListInstances(context.Context) ([]model.InstanceMirror, error)
	ListSessionsByInstance(context.Context, model.InstanceID, bool) ([]model.RemoteSession, error)
}

type Resolver struct {
	store ResolverStore
}

func NewResolver(store ResolverStore) (*Resolver, error) {
	if store == nil {
		return nil, fmt.Errorf("session resolver store is required")
	}
	return &Resolver{store: store}, nil
}

// Resolve implements the default one-active-workspace-per-instance policy.
// It consumes reconciled instance mirrors only; it never trusts a global
// current instance and never mutates Cockpit itself.
func (r *Resolver) Resolve(ctx context.Context, request ResolveRequest) (ResolveDecision, error) {
	request.AccountID = strings.TrimSpace(request.AccountID)
	request.WorkspacePath = strings.TrimSpace(request.WorkspacePath)
	if request.AccountID == "" || request.WorkspacePath == "" {
		return ResolveDecision{}, fmt.Errorf("resolver account id and workspace path are required")
	}
	instances, err := r.store.ListInstances(ctx)
	if err != nil {
		return ResolveDecision{}, err
	}
	sort.Slice(instances, func(i, j int) bool { return instances[i].ID < instances[j].ID })

	// Exact account+workspace wins even if the instance already hosts other
	// conversations: they share the same process identity and workspace.
	for _, instance := range instances {
		if !reusableInstance(instance, request.AccountID) {
			continue
		}
		if sameResolverPath(instance.WorkingDir, request.WorkspacePath) {
			return ResolveDecision{Action: ResolveReuse, InstanceID: instance.ID, Reason: "ready instance already owns requested account and workspace"}, nil
		}
	}

	// A different workspace can be placed into an existing process only when
	// explicitly allowed and there are no live child sessions pinned to it.
	if request.AllowWorkspaceReplacement {
		for _, instance := range instances {
			if !reusableInstance(instance, request.AccountID) {
				continue
			}
			sessions, err := r.store.ListSessionsByInstance(ctx, instance.ID, false)
			if err != nil {
				return ResolveDecision{}, err
			}
			if hasPinnedSession(sessions) {
				continue
			}
			return ResolveDecision{Action: ResolveReuse, InstanceID: instance.ID, Reason: "idle ready instance can safely replace workspace"}, nil
		}
	}
	return ResolveDecision{Action: ResolveCreateNew, Reason: "no safe reconciled instance matches requested account/workspace"}, nil
}

func reusableInstance(instance model.InstanceMirror, accountID string) bool {
	return instance.DesiredState == model.InstanceDesiredRunning &&
		instance.ObservedState == model.InstanceReady &&
		instance.AccountID == accountID
}

func hasPinnedSession(sessions []model.RemoteSession) bool {
	for _, session := range sessions {
		if session.DesiredState == model.SessionDesiredClosed || session.ObservedState == model.SessionClosed {
			continue
		}
		return true
	}
	return false
}

func sameResolverPath(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
