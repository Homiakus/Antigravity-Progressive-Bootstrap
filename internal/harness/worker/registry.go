package worker

import (
	"context"
	"errors"
	"fmt"
	"time"

	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
	harnessstore "github.com/homiakus/agctl/internal/harness/store"
)

type Options struct {
	IDs harnessmodel.IDGenerator
	Now func() time.Time
}

type Registry struct {
	store harnessstore.Store
	ids   harnessmodel.IDGenerator
	now   func() time.Time
}

func NewRegistry(store harnessstore.Store, opts Options) (*Registry, error) {
	if store == nil {
		return nil, fmt.Errorf("harness store is required")
	}
	ids := opts.IDs
	if ids == nil {
		g := harnessmodel.NewIDGenerator()
		ids = g
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Registry{store: store, ids: ids, now: now}, nil
}

func (r *Registry) Register(ctx context.Context, worker harnessmodel.Worker) (harnessmodel.Worker, error) {
	now := r.now().UTC()
	if worker.ID == "" {
		raw, err := r.ids.New(harnessmodel.IDWorker)
		if err != nil {
			return harnessmodel.Worker{}, fmt.Errorf("generate worker id: %w", err)
		}
		worker.ID = harnessmodel.WorkerID(raw)
	}
	if worker.State == "" {
		worker.State = harnessmodel.WorkerActive
	}
	if worker.Trust == "" {
		worker.Trust = harnessmodel.WorkerTrustedLocal
	}
	if !validState(worker.State) {
		return harnessmodel.Worker{}, fmt.Errorf("invalid worker state %q", worker.State)
	}
	if !validTrust(worker.Trust) {
		return harnessmodel.Worker{}, fmt.Errorf("invalid worker trust %q", worker.Trust)
	}

	// Worker identity is durable. Re-registering the same ID may refresh
	// capabilities/resources and liveness, but it must never rewrite the
	// original creation timestamp or return a value that disagrees with the DB.
	if worker.CreatedAt.IsZero() {
		existing, err := r.Get(ctx, worker.ID)
		switch {
		case err == nil:
			worker.CreatedAt = existing.CreatedAt
		case errors.Is(err, harnessstore.ErrNotFound):
			worker.CreatedAt = now
		default:
			return harnessmodel.Worker{}, fmt.Errorf("read existing worker %s: %w", worker.ID, err)
		}
	}
	worker.LastSeenAt = now
	if err := r.store.Update(ctx, func(tx harnessstore.Tx) error { return tx.UpsertWorker(ctx, worker) }); err != nil {
		return harnessmodel.Worker{}, err
	}
	return worker, nil
}

func (r *Registry) Heartbeat(ctx context.Context, workerID harnessmodel.WorkerID) error {
	if workerID == "" {
		return fmt.Errorf("worker id is required")
	}
	now := r.now().UTC()
	return r.store.Update(ctx, func(tx harnessstore.Tx) error { return tx.TouchWorker(ctx, workerID, now) })
}

func (r *Registry) Get(ctx context.Context, workerID harnessmodel.WorkerID) (harnessmodel.Worker, error) {
	var worker harnessmodel.Worker
	err := r.store.View(ctx, func(reader harnessstore.Reader) error {
		var err error
		worker, err = reader.GetWorker(ctx, workerID)
		return err
	})
	return worker, err
}

func validState(state harnessmodel.WorkerState) bool {
	switch state {
	case harnessmodel.WorkerActive, harnessmodel.WorkerDraining, harnessmodel.WorkerLost:
		return true
	default:
		return false
	}
}

func validTrust(trust harnessmodel.WorkerTrust) bool {
	switch trust {
	case harnessmodel.WorkerTrustedLocal, harnessmodel.WorkerTrustedRemote, harnessmodel.WorkerUntrustedRemote:
		return true
	default:
		return false
	}
}
