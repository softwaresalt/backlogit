package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	mcpinternal "github.com/backlogit/backlogit/internal/mcp"
)

// NewRootCommand creates the backlogit CLI root command.
func NewRootCommand() *cobra.Command {
	var cwd string
	var logLevel string

	root := &cobra.Command{
		Use:   "backlogit",
		Short: "Backlogit — AI-native agile workspace",
		Long: `backlogit manages a project-local work item workspace under .backlogit.

	It stores active work in .backlogit\queue, terminal work in .backlogit\archive,
	per-item history in .backlogit\logs\{item-id}.jsonl, and deferred planning work
	in .backlogit\stash.jsonl.

Use backlogit to initialize a workspace, create and update artifacts, query the
SQLite cache, migrate from supported backlog sources, manage the work queue, and
stash follow-up work for later planning.`,
		Example: `  backlogit init
  backlogit add --type feature --title "Authentication hardening"
  backlogit list --status active
  backlogit get 001-F --format json
  backlogit queue view --group-by status
  backlogit stash add "Defer audit dashboard split" --kind feature
  backlogit migrate --source .\.backlog --adapter backlog-md --dry-run
  backlogit mcp`,
		SilenceUsage: true,
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if logLevel != "" {
				applyLogLevel(logLevel)
			}
		},
	}
	root.PersistentFlags().StringVar(&cwd, "cwd", ".", "workspace directory")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)")

	root.AddCommand(newInitCommand(&cwd))
	root.AddCommand(newSyncCommand(&cwd))
	root.AddCommand(newMCPCommand(&cwd))
	root.AddCommand(newAddCommand(&cwd))
	root.AddCommand(newListCommand(&cwd))
	root.AddCommand(newGetCommand(&cwd))
	root.AddCommand(newUpdateCommand(&cwd))
	root.AddCommand(newMoveCommand(&cwd))
	root.AddCommand(newDeleteCommand(&cwd))
	root.AddCommand(newSearchCommand(&cwd))
	root.AddCommand(newQueryCommand(&cwd))
	root.AddCommand(newStatusCommand(&cwd))
	root.AddCommand(NewDepCmd())
	root.AddCommand(NewQueueCmd())
	root.AddCommand(NewStashCmd(&cwd))
	root.AddCommand(NewShipmentCmd())
	root.AddCommand(newDeliberateCommand(&cwd))
	root.AddCommand(NewMetadataCmd(&cwd))
	root.AddCommand(newArchiveCommand(&cwd))
	root.AddCommand(newMigrateCommand(&cwd))
	root.AddCommand(newAdoptCommand(&cwd))
	root.AddCommand(NewTelemetryCmd(&cwd))

	return root
}

// applyLogLevel reconfigures the global slog handler at the given level.
func applyLogLevel(level string) {
	format := strings.ToLower(os.Getenv("BACKLOGIT_LOG_FORMAT"))
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	slog.SetDefault(slog.New(h))
}

func newInitCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new backlogit workspace",
		Long: `Initialize a backlogit workspace in the target directory.

This creates the .backlogit storage root, logs directory, canonical stash JSONL
file, default YAML configuration files, and default artifact templates.`,
		Example: `  backlogit init
  backlogit init D:\Source\MyProject`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := *cwd
			if len(args) > 0 {
				root = args[0]
			}
			dir := filepath.Join(root, ".backlogit")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("create workspace dir: %w", err)
			}
			if err := os.MkdirAll(filepath.Join(dir, "logs"), 0o755); err != nil {
				return fmt.Errorf("create logs dir: %w", err)
			}
			if err := config.WriteDefaults(dir); err != nil {
				return fmt.Errorf("write defaults: %w", err)
			}
			if err := config.WriteMigrationDefaults(dir); err != nil {
				return fmt.Errorf("write migration defaults: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Initialized backlogit workspace at %s\n", dir)
			return nil
		},
	}
}

func newSyncCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Rehydrate the SQLite index from Markdown source files",
		Long: `Rebuild the backlogit SQLite cache from the Markdown and JSONL files in
the workspace.

Use this after making manual changes to .backlogit files or when you want to
force the disposable cache to match the file-backed source of truth.`,
		Example: `  backlogit sync
  backlogit --cwd D:\Source\MyProject sync`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			count, err := db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
			if err != nil {
				return fmt.Errorf("sync: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Indexed %d artifacts\n", count)
			return nil
		},
	}
}

func newMCPCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the backlogit MCP stdio server",
		Long: `Start the backlogit Model Context Protocol server over stdio.

Use this command from MCP-capable clients such as GitHub Copilot CLI, Claude
Code, or Cursor to expose backlogit workspace tools to agents.`,
		Example: `  backlogit mcp
  backlogit --cwd D:\Source\MyProject mcp`,
		RunE: func(_ *cobra.Command, _ []string) error {
			s, err := openMCPServer(context.Background(), *cwd)
			if err != nil {
				return err
			}
			return mcpinternal.RunStdio(s)
		},
	}
}

func openMCPServer(ctx context.Context, rootPath string) (*mcpinternal.Server, error) {
	ws, err := core.NewWorkspace(ctx, rootPath)
	if err == nil {
		return mcpinternal.NewServer(ws), nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return mcpinternal.NewServerForRoot(rootPath), nil
	}
	return nil, fmt.Errorf("open workspace: %w", err)
}
