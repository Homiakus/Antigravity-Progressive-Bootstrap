package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	harnesssqlite "github.com/homiakus/agctl/internal/harness/store/sqlite"
	"github.com/homiakus/agctl/internal/paths"
	"github.com/homiakus/agctl/internal/remote/model"
	"github.com/homiakus/agctl/internal/remote/repository"
	remotesqlite "github.com/homiakus/agctl/internal/remote/store/sqlite"
)

const remoteUsage = "usage: agctl remote status | daemon [OPTIONS] | telegram pair|bind ... | repo add [--name NAME] PATH | repo list [--all] | repo enable --id ID | repo disable --id ID"

// init is a temporary strangler entrypoint for the remote-control line. It
// mirrors the durable Harness entrypoint and keeps the legacy main switch
// untouched while Remote Control is brought up incrementally.
func init() {
	if len(os.Args) < 2 || os.Args[1] != "remote" {
		return
	}
	p, err := paths.Detect()
	if err == nil {
		err = p.Ensure()
	}
	if err == nil {
		err = runRemote(p, os.Args[2:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

type remoteStatus struct {
	DatabasePath        string `json:"databasePath"`
	SchemaVersion       int    `json:"schemaVersion"`
	Repositories        int    `json:"repositories"`
	EnabledRepositories int    `json:"enabledRepositories"`
	Instances           int    `json:"instances"`
	Conversations       int    `json:"conversations"`
	Sessions            int    `json:"sessions"`
	TelegramBindings    int    `json:"telegramBindings"`
	PendingCommands     int    `json:"pendingCommands"`
	PendingOutbox       int    `json:"pendingOutbox"`
}

func runRemote(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(remoteUsage)
	}
	switch args[0] {
	case "status":
		if len(args) != 1 {
			return fmt.Errorf(remoteUsage)
		}
		return runRemoteStatus(p)
	case "daemon":
		return runRemoteDaemon(p, args[1:])
	case "telegram":
		return runRemoteTelegram(p, args[1:])
	case "repo":
		return runRemoteRepo(p, args[1:])
	default:
		return fmt.Errorf(remoteUsage)
	}
}

func runRemoteStatus(p paths.Paths) error {
	ctx := context.Background()
	db, err := harnesssqlite.Open(ctx, p.HarnessDB, harnesssqlite.Options{})
	if err != nil {
		return err
	}
	defer db.Close()
	status := remoteStatus{DatabasePath: db.Path()}
	if status.SchemaVersion, err = db.SchemaVersion(ctx); err != nil {
		return err
	}
	queries := []struct {
		query string
		out   *int
	}{
		{"SELECT COUNT(*) FROM remote_repositories", &status.Repositories},
		{"SELECT COUNT(*) FROM remote_repositories WHERE enabled=1", &status.EnabledRepositories},
		{"SELECT COUNT(*) FROM remote_instances", &status.Instances},
		{"SELECT COUNT(*) FROM remote_conversations", &status.Conversations},
		{"SELECT COUNT(*) FROM remote_sessions WHERE observed_state <> 'CLOSED'", &status.Sessions},
		{"SELECT COUNT(*) FROM telegram_bindings WHERE enabled=1", &status.TelegramBindings},
		{"SELECT COUNT(*) FROM remote_commands WHERE state IN ('PENDING','RUNNING')", &status.PendingCommands},
		{"SELECT COUNT(*) FROM remote_outbox WHERE delivered_at IS NULL", &status.PendingOutbox},
	}
	for _, query := range queries {
		if err := db.SQLDB().QueryRowContext(ctx, query.query).Scan(query.out); err != nil {
			return fmt.Errorf("remote status: %w", err)
		}
	}
	printJSON(status)
	return nil
}

func runRemoteRepo(p paths.Paths, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(remoteUsage)
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
	case "add":
		fs := flag.NewFlagSet("remote repo add", flag.ContinueOnError)
		name := fs.String("name", "", "repository display name")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: agctl remote repo add [--name NAME] PATH")
		}
		registry, err := repository.New(store, repository.Options{})
		if err != nil {
			return err
		}
		record, err := registry.Add(ctx, fs.Arg(0), *name)
		if err != nil {
			return err
		}
		printJSON(record)
		return nil
	case "list":
		fs := flag.NewFlagSet("remote repo list", flag.ContinueOnError)
		all := fs.Bool("all", false, "include disabled repositories")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return fmt.Errorf("usage: agctl remote repo list [--all]")
		}
		records, err := store.ListRepositories(ctx, !*all)
		if err != nil {
			return err
		}
		printJSON(records)
		return nil
	case "enable", "disable":
		fs := flag.NewFlagSet("remote repo "+args[0], flag.ContinueOnError)
		id := fs.String("id", "", "registered repository id")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *id == "" || fs.NArg() != 0 {
			return fmt.Errorf("usage: agctl remote repo %s --id ID", args[0])
		}
		if err := model.ValidateGeneratedID(*id, model.IDRepository); err != nil {
			return err
		}
		enabled := args[0] == "enable"
		if err := store.SetRepositoryEnabled(ctx, model.RepositoryID(*id), enabled); err != nil {
			return err
		}
		record, err := store.GetRepository(ctx, model.RepositoryID(*id))
		if err != nil {
			return err
		}
		printJSON(record)
		return nil
	default:
		return fmt.Errorf(remoteUsage)
	}
}
