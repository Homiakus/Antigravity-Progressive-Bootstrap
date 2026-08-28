package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/homiakus/agctl/internal/antigravityide"
	"github.com/homiakus/agctl/internal/cockpit"
	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/paths"
	remoteaccount "github.com/homiakus/agctl/internal/remote/account"
	remotecommand "github.com/homiakus/agctl/internal/remote/command"
	remotedaemon "github.com/homiakus/agctl/internal/remote/daemon"
	"github.com/homiakus/agctl/internal/remote/mirror"
	"github.com/homiakus/agctl/internal/remote/model"
	remotesession "github.com/homiakus/agctl/internal/remote/session"
	remotesqlite "github.com/homiakus/agctl/internal/remote/store/sqlite"
	"github.com/homiakus/agctl/internal/telegram"
)

type repeatedString []string

func (r *repeatedString) String() string { return strings.Join(*r, ",") }
func (r *repeatedString) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value is empty")
	}
	*r = append(*r, value)
	return nil
}

type bootstrapOpen struct {
	RepositoryID model.RepositoryID
	AccountID    string
}

func runRemoteDaemon(p paths.Paths, args []string) error {
	fs := flag.NewFlagSet("remote daemon", flag.ContinueOnError)
	cockpitBin := fs.String("cockpit-bin", "cockpit-control", "Cockpit control-plane CLI")
	tokenEnv := fs.String("telegram-token-env", "AGCTL_TELEGRAM_BOT_TOKEN", "environment variable containing Telegram bot token")
	tick := fs.Duration("tick", 750*time.Millisecond, "fast control-loop interval")
	longPoll := fs.Duration("long-poll", 30*time.Second, "Telegram long-poll duration")
	recoverRunning := fs.Bool("recover-running", false, "explicitly stop and managed-restart persisted running IDE instances that lack daemon-owned Bridge credentials")
	recoveryTimeout := fs.Duration("recovery-timeout", 45*time.Second, "timeout per managed Bridge recovery/start")
	var opens repeatedString
	fs.Var(&opens, "open", "idempotently ensure a live session at startup, format REPOSITORY_ID:ACCOUNT_ID; repeatable")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("usage: agctl remote daemon [--recover-running] [--open REPOSITORY_ID:ACCOUNT_ID ...]")
	}

	envName := strings.TrimSpace(*tokenEnv)
	if envName == "" {
		return fmt.Errorf("Telegram token environment variable name is empty")
	}
	token := strings.TrimSpace(os.Getenv(envName))
	if token == "" {
		return fmt.Errorf("Telegram bot token is missing from environment variable %s", envName)
	}
	bootstrap := make([]bootstrapOpen, 0, len(opens))
	for _, raw := range opens {
		spec, err := parseBootstrapOpen(raw)
		if err != nil {
			return err
		}
		bootstrap = append(bootstrap, spec)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := harnesssqlite.Open(ctx, p.HarnessDB, harnesssqlite.Options{})
	if err != nil {
		return err
	}
	defer db.Close()
	store, err := remotesqlite.New(db.SQLDB())
	if err != nil {
		return err
	}

	cockpitClient, err := cockpit.NewCLI(*cockpitBin, nil)
	if err != nil {
		return err
	}
	probeCtx, cancelProbe := context.WithTimeout(ctx, 10*time.Second)
	_, err = cockpitClient.Protocol(probeCtx)
	cancelProbe()
	if err != nil {
		return fmt.Errorf("Cockpit control-plane probe failed: %w", err)
	}

	bridgeRegistry := filepath.Join(p.StateRoot, "remote", "bridges")
	profileRoot := filepath.Join(p.ProfilesRoot, "remote-ide")
	for _, dir := range []string{bridgeRegistry, profileRoot} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create remote daemon directory %s: %w", dir, err)
		}
	}
	resolver, err := remotesession.NewResolver(store)
	if err != nil {
		return err
	}
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "localhost"
	}
	sessionService, err := remotesession.New(remotesession.Options{
		Store: store, Cockpit: cockpitClient, Locator: antigravityide.Discovery{Root: bridgeRegistry}, Resolver: resolver,
		HostID: model.HostID(hostname), ProfileRoot: profileRoot, BridgeRegistry: bridgeRegistry,
	})
	if err != nil {
		return err
	}

	recoverPersistedSessions(ctx, store, sessionService, *recoverRunning, *recoveryTimeout)
	for _, spec := range bootstrap {
		if err := ensureBootstrapSession(ctx, store, sessionService, spec, *recoveryTimeout); err != nil {
			return err
		}
	}

	api, err := telegram.NewAPIClient(token, telegram.APIOptions{})
	if err != nil {
		return err
	}
	pairing, err := telegram.NewPairingService(store, nil, nil)
	if err != nil {
		return err
	}
	accounts, err := remoteaccount.New(store, cockpitClient)
	if err != nil {
		return err
	}
	ui, err := telegram.NewUI(api, store, accounts)
	if err != nil {
		return err
	}
	poller, err := telegram.NewPoller(telegram.PollerOptions{
		BotKey: telegramBotKey(token), API: api, Store: store, Pairing: pairing, UI: ui, LongPoll: *longPoll,
	})
	if err != nil {
		return err
	}
	commands := &remotecommand.Worker{Store: store, Bridges: sessionService}
	ingestor := &mirror.Ingestor{Store: store}
	delivery := &mirror.TelegramWorker{Store: store, API: api}
	supervisor, err := remotedaemon.New(remotedaemon.Options{
		Telegram: poller, Commands: commands, Delivery: delivery, Ingestor: ingestor, Store: store, Bridges: sessionService, Tick: *tick,
		ReportError: func(component string, err error) {
			fmt.Fprintf(os.Stderr, "remote daemon %s: %v\n", component, err)
		},
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "remote daemon started; database=%s bridgeRegistry=%s\n", db.Path(), bridgeRegistry)
	if err := supervisor.Run(ctx); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}

func parseBootstrapOpen(raw string) (bootstrapOpen, error) {
	parts := strings.SplitN(strings.TrimSpace(raw), ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return bootstrapOpen{}, fmt.Errorf("invalid --open %q; want REPOSITORY_ID:ACCOUNT_ID", raw)
	}
	if err := model.ValidateGeneratedID(parts[0], model.IDRepository); err != nil {
		return bootstrapOpen{}, err
	}
	return bootstrapOpen{RepositoryID: model.RepositoryID(parts[0]), AccountID: strings.TrimSpace(parts[1])}, nil
}

func recoverPersistedSessions(ctx context.Context, store *remotesqlite.Store, service *remotesession.Service, restartRunning bool, timeout time.Duration) {
	instances, err := store.ListInstances(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "remote recovery list instances:", err)
		return
	}
	for _, instance := range instances {
		sessions, err := store.ListSessionsByInstance(ctx, instance.ID, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "remote recovery instance %s: %v\n", instance.ID, err)
			continue
		}
		for _, item := range sessions {
			if item.DesiredState == model.SessionDesiredClosed || item.ObservedState == model.SessionClosed {
				continue
			}
			if _, ok := service.Bridge(string(item.CockpitInstanceID)); ok {
				continue
			}
			recoverCtx, cancel := context.WithTimeout(ctx, timeout)
			_, err := service.Recover(recoverCtx, item.ID, restartRunning)
			cancel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "remote recovery session %s: %v\n", item.ID, err)
			}
		}
	}
}

func ensureBootstrapSession(ctx context.Context, store *remotesqlite.Store, service *remotesession.Service, spec bootstrapOpen, timeout time.Duration) error {
	instances, err := store.ListInstances(ctx)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		sessions, err := store.ListSessionsByInstance(ctx, instance.ID, false)
		if err != nil {
			return err
		}
		for _, item := range sessions {
			if item.RepositoryID != spec.RepositoryID || item.CockpitAccountID != spec.AccountID || item.DesiredState == model.SessionDesiredClosed || item.ObservedState == model.SessionClosed {
				continue
			}
			if _, ok := service.Bridge(string(item.CockpitInstanceID)); !ok {
				return fmt.Errorf("existing session %s matches --open but has no daemon-owned Bridge; use --recover-running or stop that IDE instance", item.ID)
			}
			fmt.Fprintf(os.Stderr, "remote session already live: %s\n", item.ID)
			return nil
		}
	}
	openCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	item, err := service.Provision(openCtx, remotesession.Spec{
		RepositoryID: spec.RepositoryID, AccountID: spec.AccountID, InstanceStrategy: remotesession.InstanceAuto,
		ConversationStrategy: remotesession.ConversationNew, IsolationMode: model.IsolationExclusiveWrite,
	})
	if err != nil {
		return fmt.Errorf("bootstrap remote session %s: %w", spec.RepositoryID, err)
	}
	fmt.Fprintf(os.Stderr, "remote session opened: %s repository=%s account=%s\n", item.ID, spec.RepositoryID, spec.AccountID)
	return nil
}

func telegramBotKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "telegram:" + hex.EncodeToString(sum[:8])
}

func runRemoteTelegram(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: agctl remote telegram pair|bind ...")
	}
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, p.HarnessDB, harnesssqlite.Options{})
	if err != nil {
		return err
	}
	defer db.Close()
	store, err := remotesqlite.New(db.SQLDB())
	if err != nil {
		return err
	}
	switch args[0] {
	case "pair":
		fs := flag.NewFlagSet("remote telegram pair", flag.ContinueOnError)
		roleText := fs.String("role", "OWNER", "OWNER, OPERATOR or VIEWER")
		chatID := fs.Int64("chat", 0, "optional chat id the code is restricted to")
		ttl := fs.Duration("ttl", 10*time.Minute, "pairing code lifetime")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("usage: agctl remote telegram pair [--role OWNER] [--chat CHAT_ID] [--ttl 10m]")
		}
		role := model.TelegramRole(strings.ToUpper(strings.TrimSpace(*roleText)))
		if !role.Valid() {
			return fmt.Errorf("invalid Telegram role %q", *roleText)
		}
		service, err := telegram.NewPairingService(store, nil, nil)
		if err != nil {
			return err
		}
		code, err := service.Create(ctx, role, *chatID, *ttl)
		if err != nil {
			return err
		}
		printJSON(struct {
			Code   string             `json:"code"`
			Role   model.TelegramRole `json:"role"`
			ChatID int64              `json:"chatId,omitempty"`
			TTL    string             `json:"ttl"`
		}{Code: code, Role: role, ChatID: *chatID, TTL: ttl.String()})
		return nil
	case "bind":
		fs := flag.NewFlagSet("remote telegram bind", flag.ContinueOnError)
		sessionID := fs.String("session", "", "remote session id")
		chatID := fs.Int64("chat", 0, "Telegram chat id")
		threadID := fs.Int64("thread", 0, "Telegram topic/thread id; zero for ordinary chat")
		ownerID := fs.Int64("owner", 0, "paired Telegram owner user id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 || *sessionID == "" || *chatID == 0 || *ownerID == 0 {
			return fmt.Errorf("usage: agctl remote telegram bind --session ID --chat CHAT_ID [--thread THREAD_ID] --owner USER_ID")
		}
		if err := model.ValidateGeneratedID(*sessionID, model.IDRemoteSession); err != nil {
			return err
		}
		if _, err := store.GetSession(ctx, model.RemoteSessionID(*sessionID)); err != nil {
			return err
		}
		principal, err := store.GetTelegramPrincipal(ctx, *ownerID)
		if err != nil {
			return fmt.Errorf("Telegram owner must be paired first: %w", err)
		}
		if !principal.Enabled || principal.Role != model.TelegramRoleOwner {
			return fmt.Errorf("Telegram binding owner %d is not an enabled OWNER", *ownerID)
		}
		generator := model.NewIDGenerator()
		id, err := generator.New(model.IDTelegramBinding)
		if err != nil {
			return err
		}
		binding := model.TelegramBinding{
			ID: model.TelegramBindingID(id), SessionID: model.RemoteSessionID(*sessionID), ChatID: *chatID,
			ThreadID: *threadID, OwnerUserID: *ownerID, Enabled: true, CreatedAt: time.Now().UTC(),
		}
		if err := store.ReplaceTelegramBinding(ctx, binding); err != nil {
			return err
		}
		printJSON(binding)
		return nil
	default:
		return fmt.Errorf("usage: agctl remote telegram pair|bind ...")
	}
}
