package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/remote/mirror"
	"github.com/homiakus/agctl/internal/remote/model"
)

type TelegramPoller interface {
	Run(context.Context) error
}

type CommandWorker interface {
	RunOnce(context.Context, int) (int, error)
}

type DeliveryWorker interface {
	RunOnce(context.Context, int) (int, error)
}

type MirrorIngestor interface {
	PollSession(context.Context, mirror.EventClient, model.RemoteSessionID) (int, error)
}

type SessionStore interface {
	ListInstances(context.Context) ([]model.InstanceMirror, error)
	ListSessionsByInstance(context.Context, model.InstanceID, bool) ([]model.RemoteSession, error)
}

type BridgeResolver interface {
	Bridge(string) (antigravityide.LocatedBridge, bool)
}

type ErrorReporter func(component string, err error)

type Options struct {
	Telegram       TelegramPoller
	Commands       CommandWorker
	Delivery       DeliveryWorker
	Ingestor       MirrorIngestor
	Store          SessionStore
	Bridges        BridgeResolver
	Tick           time.Duration
	CommandBatch   int
	DeliveryBatch  int
	ReportError    ErrorReporter
}

type Supervisor struct {
	telegram      TelegramPoller
	commands      CommandWorker
	delivery      DeliveryWorker
	ingestor      MirrorIngestor
	store         SessionStore
	bridges       BridgeResolver
	tick          time.Duration
	commandBatch  int
	deliveryBatch int
	report        ErrorReporter
}

func New(opts Options) (*Supervisor, error) {
	if opts.Telegram == nil || opts.Commands == nil || opts.Delivery == nil || opts.Ingestor == nil || opts.Store == nil || opts.Bridges == nil {
		return nil, fmt.Errorf("remote daemon requires Telegram, command, mirror, store and Bridge components")
	}
	tick := opts.Tick
	if tick <= 0 {
		tick = 750 * time.Millisecond
	}
	if tick < 100*time.Millisecond {
		return nil, fmt.Errorf("remote daemon tick below 100ms is not supported")
	}
	commandBatch := opts.CommandBatch
	if commandBatch <= 0 {
		commandBatch = 50
	}
	deliveryBatch := opts.DeliveryBatch
	if deliveryBatch <= 0 {
		deliveryBatch = 100
	}
	report := opts.ReportError
	if report == nil {
		report = func(string, error) {}
	}
	return &Supervisor{telegram: opts.Telegram, commands: opts.Commands, delivery: opts.Delivery, ingestor: opts.Ingestor, store: opts.Store, bridges: opts.Bridges, tick: tick, commandBatch: commandBatch, deliveryBatch: deliveryBatch, report: report}, nil
}

func (s *Supervisor) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	telegramErr := make(chan error, 1)
	go func() { telegramErr <- s.telegram.Run(ctx) }()

	if err := s.cycle(ctx); err != nil {
		s.report("cycle", err)
	}
	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-telegramErr:
			if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return ctx.Err()
			}
			return fmt.Errorf("Telegram poller stopped: %w", err)
		case <-ticker.C:
			if err := s.cycle(ctx); err != nil {
				s.report("cycle", err)
			}
		}
	}
}

func (s *Supervisor) cycle(ctx context.Context) error {
	var errs []error
	if _, err := s.commands.RunOnce(ctx, s.commandBatch); err != nil {
		errs = append(errs, fmt.Errorf("commands: %w", err))
	}
	if err := s.ingest(ctx); err != nil {
		errs = append(errs, err)
	}
	if _, err := s.delivery.RunOnce(ctx, s.deliveryBatch); err != nil {
		errs = append(errs, fmt.Errorf("delivery: %w", err))
	}
	return errors.Join(errs...)
}

func (s *Supervisor) ingest(ctx context.Context) error {
	instances, err := s.store.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("list remote instances: %w", err)
	}
	var errs []error
	for _, instance := range instances {
		bridge, ok := s.bridges.Bridge(string(instance.ID))
		if !ok || bridge.Client == nil {
			continue
		}
		eventClient, ok := bridge.Client.(mirror.EventClient)
		if !ok {
			continue
		}
		sessions, err := s.store.ListSessionsByInstance(ctx, instance.ID, false)
		if err != nil {
			errs = append(errs, fmt.Errorf("list sessions for %s: %w", instance.ID, err))
			continue
		}
		for _, session := range sessions {
			if session.DesiredState == model.SessionDesiredClosed || session.ObservedState == model.SessionClosed {
				continue
			}
			if _, err := s.ingestor.PollSession(ctx, eventClient, session.ID); err != nil && !errors.Is(err, mirror.ErrAgentEventsUnavailable) {
				errs = append(errs, fmt.Errorf("mirror session %s: %w", session.ID, err))
			}
		}
	}
	return errors.Join(errs...)
}
