package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/cli/format"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
	"github.com/softwaresalt/backlogit/internal/version"
)

// jsonrpcInterceptor captures stdout during a command run so the output can be
// wrapped in a JSON-RPC 2.0 response envelope in PersistentPostRunE.
type jsonrpcInterceptor struct {
	enabled bool
	wrapped bool
	buf     *bytes.Buffer
	origOut io.Writer
	cmdPath string
}

// Execute creates the root command and runs it. When --jsonrpc is active and
// the command fails (including flag-parse and argument-validation failures),
// it writes a JSON-RPC 2.0 error envelope to stdout instead of letting Cobra
// print the error to stderr.
//
// main.go should call this rather than cli.NewRootCommand().Execute() so that
// the full error path is covered by the JSON-RPC contract.
func Execute() error {
	jctx := &jsonrpcInterceptor{}
	root := newRootCommandImpl(jctx)
	origOut := root.OutOrStdout()

	// Pre-scan os.Args to detect --jsonrpc before Cobra parses flags.
	// This lets us silence Cobra's own error output and write a JSON-RPC
	// error envelope even when PersistentPreRunE never runs (flag parse errors,
	// --help, --version).
	// pflag accepts boolean flags as --flag, --flag=true, and --flag=false, so
	// we must check for all non-false variants.
	jsonrpcRequested := false
	for _, arg := range os.Args[1:] {
		if arg == "--jsonrpc" ||
			(strings.HasPrefix(arg, "--jsonrpc=") &&
				arg != "--jsonrpc=false" && arg != "--jsonrpc=0") {
			jsonrpcRequested = true
			break
		}
	}
	if jsonrpcRequested {
		root.SilenceErrors = true
		capture := &bytes.Buffer{}
		root.SetOut(capture)
		root.SetErr(capture)
	}

	executed, err := root.ExecuteC()
	if !jsonrpcRequested {
		return err
	}
	if err != nil && jsonrpcRequested {
		cmdPath := jctx.cmdPath
		if cmdPath == "" {
			cmdPath = "backlogit"
		}
		b, wrapErr := format.WrapError(cmdPath, format.ErrCodeServerError, err.Error())
		if wrapErr != nil {
			// Marshaling failed; fall back to a minimal valid JSON-RPC error envelope.
			b = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"error":{"code":%d,"message":%q}}`,
				cmdPath, format.ErrCodeServerError, err.Error()))
		}
		fmt.Fprintf(origOut, "%s\n", b)
		return err
	}
	captured, _ := root.OutOrStdout().(*bytes.Buffer)
	if jctx.wrapped {
		if captured == nil {
			return nil
		}
		raw := bytes.TrimSpace(captured.Bytes())
		if len(raw) == 0 {
			return nil
		}
		_, err = fmt.Fprintf(origOut, "%s\n", raw)
		return err
	}
	cmdPath := jctx.cmdPath
	if cmdPath == "" && executed != nil {
		cmdPath = executed.CommandPath()
	}
	if cmdPath == "" {
		cmdPath = "backlogit"
	}
	return writeJSONRPCResult(origOut, cmdPath, captured.Bytes())
}

// NewRootCommand creates the backlogit CLI root command.
// Use Execute() from main.go for production use. NewRootCommand is kept for
// test-harness access where the caller controls SetArgs and SetOut directly.
func NewRootCommand() *cobra.Command {
	return newRootCommandImpl(&jsonrpcInterceptor{})
}

// newRootCommandImpl builds the root command wired to the supplied interceptor.
func newRootCommandImpl(jctx *jsonrpcInterceptor) *cobra.Command {
	var cwd string
	var logLevel string
	var jsonrpcFlag bool

	root := &cobra.Command{
		Use:     "backlogit",
		Version: version.Version,
		Short:   "Backlogit — AI-native agile workspace",
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
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if logLevel != "" {
				applyLogLevel(logLevel)
			}
			jctx.enabled = jsonrpcFlag
			if jctx.enabled {
				jctx.buf = &bytes.Buffer{}
				jctx.origOut = cmd.OutOrStdout()
				jctx.cmdPath = cmd.CommandPath()
				cmd.SetOut(jctx.buf)
			}
			return nil
		},
		PersistentPostRunE: func(_ *cobra.Command, _ []string) error {
			if !jctx.enabled || jctx.buf == nil {
				return nil
			}
			raw := bytes.TrimSpace(jctx.buf.Bytes())
			if len(raw) == 0 {
				return nil
			}
			var result any
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, &result); err != nil {
					result = string(raw)
				}
			}
			b, err := format.WrapResult(jctx.cmdPath, result)
			if err != nil {
				return err
			}
			if _, err = fmt.Fprintf(jctx.origOut, "%s\n", b); err != nil {
				return err
			}
			jctx.wrapped = true
			return nil
		},
	}
	root.PersistentFlags().StringVar(&cwd, "cwd", ".", "workspace directory")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)")
	root.PersistentFlags().BoolVar(&jsonrpcFlag, "jsonrpc", false, "wrap all output in a JSON-RPC 2.0 response envelope")

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
	root.AddCommand(newQueueCmd(&cwd))
	root.AddCommand(NewStashCmd(&cwd))
	root.AddCommand(NewShipmentCmd())
	root.AddCommand(newDeliberateCommand(&cwd))
	root.AddCommand(NewMetadataCmd(&cwd))
	root.AddCommand(newArchiveCommand(&cwd))
	root.AddCommand(newMigrateCommand(&cwd))
	root.AddCommand(newAdoptCommand(&cwd))
	root.AddCommand(NewTelemetryCmd(&cwd))
	root.AddCommand(NewCheckpointCmd(&cwd))
	root.AddCommand(newDoctorCommand(&cwd))
	root.AddCommand(newVersionCommand())
	root.AddCommand(newManifestCommand(&cwd))

	return root
}

func writeJSONRPCResult(w io.Writer, cmdPath string, raw []byte) error {
	raw = bytes.TrimSpace(raw)

	var result any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &result); err != nil {
			result = string(raw)
		}
	}

	b, err := format.WrapResult(cmdPath, result)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
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
