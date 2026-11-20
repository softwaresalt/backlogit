package cli

import (
	"bufio"
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
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/softwaresalt/backlogit/internal/cli/format"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
	"github.com/softwaresalt/backlogit/internal/toolchain"
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

var versionTemplateOnce sync.Once

// Execute creates the root command and runs it. When --jsonrpc is active and
// the command fails (including flag-parse and argument-validation failures),
// it writes a JSON-RPC 2.0 error envelope to stdout instead of letting Cobra
// print the error to stderr.
//
// main.go should call this rather than cli.NewRootCommand().Execute() so that
// the full error path is covered by the JSON-RPC contract.
func Execute() error {
	jctx := &jsonrpcInterceptor{}
	root := newRootCommandImpl(jctx, defaultVersionLatestLookup)
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
		b, wrapErr := func() ([]byte, error) {
			var unknownField *corerrors.CheckpointUnknownFieldError
			if errors.As(err, &unknownField) {
				bounded := unknownField.BoundedFields()
				data := map[string]any{
					"unknown_fields":           bounded.Paths,
					"unknown_fields_truncated": bounded.Truncated,
					"unknown_fields_omitted":   bounded.OmittedPaths,
					"unknown_fields_shortened": bounded.TruncatedPaths,
				}
				return format.WrapErrorData(cmdPath, format.ErrCodeServerError, unknownField.Error(), data)
			}
			var blocked *soundnessBlockedError
			if errors.As(err, &blocked) {
				return format.WrapErrorData(cmdPath, format.ErrCodeServerError, blocked.Error(), blocked.Decision)
			}
			var policyBlocked *policyBlockedError
			if errors.As(err, &policyBlocked) {
				return format.WrapErrorData(cmdPath, format.ErrCodeServerError, policyBlocked.Error(), policyBlocked.Decision)
			}
			var custodyErr *custodyCLIError
			if errors.As(err, &custodyErr) {
				return format.WrapErrorData(cmdPath, format.ErrCodeServerError, custodyErr.Error(),
					map[string]any{"error": custodyErr.Code, "message": custodyErr.Message})
			}
			return format.WrapError(cmdPath, format.ErrCodeServerError, err.Error())
		}()
		if wrapErr != nil {
			// Marshaling failed; fall back to a minimal valid JSON-RPC error envelope.
			b = []byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":%q,"error":{"code":%d,"message":%q}}`,
				cmdPath, format.ErrCodeServerError, err.Error()))
		}
		if _, writeErr := fmt.Fprintf(origOut, "%s\n", b); writeErr != nil {
			return fmt.Errorf("write JSON-RPC error response: %w", writeErr)
		}
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
		if _, err = fmt.Fprintf(origOut, "%s\n", raw); err != nil {
			return fmt.Errorf("write JSON-RPC captured response: %w", err)
		}
		return nil
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
	return newRootCommandImpl(&jsonrpcInterceptor{}, defaultVersionLatestLookup)
}

// newRootCommandImpl builds the root command wired to the supplied interceptor.
func newRootCommandImpl(jctx *jsonrpcInterceptor, latestLookup latestVersionLookupFunc) *cobra.Command {
	if latestLookup == nil {
		latestLookup = defaultVersionLatestLookup
	}
	var cwd string
	var logLevel string
	var jsonrpcFlag bool
	var noUpdateCheck bool

	root := &cobra.Command{
		Use:     "backlogit",
		Version: version.Resolve(),
		Short:   "Backlogit — AI-native agile workspace",
		Long: `backlogit manages a project-local work item workspace under .backlog by default.

	It stores active work in .backlog\queue, terminal work in .backlog\archive,
	per-item history in .backlog\logs\{item-id}.jsonl, and deferred planning work
	in .backlog\stash.jsonl. Existing .backlogit workspaces remain supported.

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
				return fmt.Errorf("write JSON-RPC post-run response: %w", err)
			}
			jctx.wrapped = true
			return nil
		},
	}
	root.SetContext(context.WithValue(context.Background(), versionLatestLookupContextKey{}, latestLookup))
	versionTemplateOnce.Do(func() {
		cobra.AddTemplateFunc("backlogitVersionLine", formatRootVersionLine)
	})
	root.SetVersionTemplate("{{backlogitVersionLine .}}\n")
	root.PersistentFlags().StringVar(&cwd, "cwd", ".", "workspace directory")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: debug, info, warn, error (overrides BACKLOGIT_LOG_LEVEL)")
	root.PersistentFlags().BoolVar(&jsonrpcFlag, "jsonrpc", false, "wrap all output in a JSON-RPC 2.0 response envelope")
	root.PersistentFlags().BoolVar(&noUpdateCheck, "no-update-check", false, "skip the remote latest-release check")

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
	root.AddCommand(NewCustodyCmd(&cwd))
	root.AddCommand(newDeliberateCommand(&cwd))
	root.AddCommand(NewMetadataCmd(&cwd))
	root.AddCommand(newArchiveCommand(&cwd))
	root.AddCommand(newMigrateCommand(&cwd))
	root.AddCommand(newAdoptCommand(&cwd))
	root.AddCommand(newLinkCmd(&cwd))
	root.AddCommand(newHooksCmd(&cwd))
	root.AddCommand(newMemoryCmd(&cwd))
	root.AddCommand(newCommentCmd(&cwd))
	root.AddCommand(NewTelemetryCmd(&cwd))
	root.AddCommand(NewCheckpointCmd(&cwd))
	root.AddCommand(newDoctorCommand(&cwd))
	root.AddCommand(newReconcileCommand(&cwd))
	root.AddCommand(newDocsCommand(&cwd))
	root.AddCommand(newSoundnessCommand(&cwd))
	root.AddCommand(newPolicyCommand(&cwd))
	root.AddCommand(newTrustAnchorCommand(&cwd))
	root.AddCommand(newVersionCommandWithLookup(latestLookup))
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
	if _, err = fmt.Fprintf(w, "%s\n", b); err != nil {
		return fmt.Errorf("write JSON-RPC result: %w", err)
	}
	return nil
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

func writeBootstrapFileIfNotExistsInRoot(dstHandle *os.Root, dstDirAbs, relName string, data []byte) error {
	targetAbs := filepath.Join(dstDirAbs, relName)
	info, statErr := dstHandle.Lstat(relName)
	if statErr == nil {
		isLink, linkErr := core.IsSymlinkOrReparsePoint(info, targetAbs)
		if linkErr != nil || isLink || info.Mode()&os.ModeIrregular != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("bootstrap target %s is redirected or not a regular file: %w", targetAbs, corerrors.ErrValidation)
		}
		return nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("inspect bootstrap target %s: %w", targetAbs, statErr)
	}

	f, err := dstHandle.OpenFile(relName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create bootstrap target %s: %w", targetAbs, err)
	}
	fClosed := false
	defer func() {
		if !fClosed {
			_ = f.Close()
		}
	}()
	openedStat, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat bootstrap target %s: %w", targetAbs, err)
	}
	lstatStat, err := dstHandle.Lstat(relName)
	if err != nil || !os.SameFile(openedStat, lstatStat) {
		return fmt.Errorf("bootstrap target %s identity changed after open: %w", targetAbs, corerrors.ErrValidation)
	}
	written := 0
	for written < len(data) {
		n, wErr := f.Write(data[written:])
		if wErr != nil {
			return fmt.Errorf("write bootstrap target %s: %w", targetAbs, wErr)
		}
		if n == 0 {
			return fmt.Errorf("short write to bootstrap target %s: %w", targetAbs, io.ErrShortWrite)
		}
		written += n
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync bootstrap target %s: %w", targetAbs, err)
	}
	fClosed = true
	if err := f.Close(); err != nil {
		return fmt.Errorf("close bootstrap target %s: %w", targetAbs, err)
	}
	return nil
}

func writeWorkspaceBootstrapDefaults(wsRoot, storageHandle *os.Root, cleanRoot, storageDirName string) error {
	if initBeforeWriteDefaultsHook != nil {
		initBeforeWriteDefaultsHook()
	}

	wsStat, err := wsRoot.Stat(".")
	if err != nil {
		return fmt.Errorf("stat workspace root before defaults: %w", err)
	}
	storageStat, err := storageHandle.Stat(".")
	if err != nil {
		return fmt.Errorf("stat storage root before defaults: %w", err)
	}

	_, recheckWS, recheckErr := openOrCreateValidatedWorkspaceRoot(cleanRoot, false)
	if recheckErr != nil {
		if errors.Is(recheckErr, corerrors.ErrValidation) {
			return recheckErr
		}
		return fmt.Errorf("re-verify workspace root before defaults: %w", errors.Join(corerrors.ErrValidation, recheckErr))
	}
	defer func() { _ = recheckWS.Close() }()

	recheckWSStat, err := recheckWS.Stat(".")
	if err != nil || !os.SameFile(wsStat, recheckWSStat) {
		return fmt.Errorf("workspace root %s identity changed before writing defaults: %w", cleanRoot, corerrors.ErrValidation)
	}

	storageAbs := filepath.Join(cleanRoot, storageDirName)
	recheckStorageInfo, err := recheckWS.Lstat(storageDirName)
	if err != nil {
		return fmt.Errorf("re-inspect storage dir %s before defaults: %w", storageAbs, errors.Join(corerrors.ErrValidation, err))
	}
	storageLink, linkErr := core.IsSymlinkOrReparsePoint(recheckStorageInfo, storageAbs)
	if linkErr != nil || storageLink || recheckStorageInfo.Mode()&os.ModeIrregular != 0 || !recheckStorageInfo.IsDir() {
		return fmt.Errorf("storage dir %s is redirected or not a directory: %w", storageAbs, corerrors.ErrValidation)
	}
	recheckStorage, err := recheckWS.OpenRoot(storageDirName)
	if err != nil {
		return fmt.Errorf("re-open storage dir %s before defaults: %w", storageAbs, errors.Join(corerrors.ErrValidation, err))
	}
	defer func() { _ = recheckStorage.Close() }()
	recheckStorageStat, err := recheckStorage.Stat(".")
	if err != nil || !os.SameFile(storageStat, recheckStorageStat) {
		return fmt.Errorf("storage dir %s identity changed before writing defaults: %w", storageAbs, corerrors.ErrValidation)
	}

	var templatesHandle *os.Root
	defer func() {
		if templatesHandle != nil {
			_ = templatesHandle.Close()
		}
	}()
	for _, sub := range []string{"logs", "queue", "templates"} {
		subAbs := filepath.Join(storageAbs, sub)
		subInfo, subErr := storageHandle.Lstat(sub)
		if errors.Is(subErr, os.ErrNotExist) {
			if mkErr := storageHandle.Mkdir(sub, 0o755); mkErr != nil {
				return fmt.Errorf("create storage subdir %s: %w", subAbs, mkErr)
			}
			subInfo, subErr = storageHandle.Lstat(sub)
		}
		if subErr != nil {
			return fmt.Errorf("inspect storage subdir %s: %w", subAbs, subErr)
		}
		subLink, linkErr := core.IsSymlinkOrReparsePoint(subInfo, subAbs)
		if linkErr != nil || subLink || subInfo.Mode()&os.ModeIrregular != 0 || !subInfo.IsDir() {
			return fmt.Errorf("storage subdir %s is redirected or not a directory: %w", subAbs, corerrors.ErrValidation)
		}
		subHandle, openSubErr := storageHandle.OpenRoot(sub)
		if openSubErr != nil {
			return fmt.Errorf("open storage subdir %s: %w", subAbs, openSubErr)
		}
		openedSub, statSubErr := subHandle.Stat(".")
		if statSubErr != nil || !os.SameFile(subInfo, openedSub) {
			_ = subHandle.Close()
			return fmt.Errorf("storage subdir %s identity changed after open: %w", subAbs, corerrors.ErrValidation)
		}
		if sub == "templates" {
			templatesHandle = subHandle
		} else {
			if closeErr := subHandle.Close(); closeErr != nil {
				return fmt.Errorf("close storage subdir %s: %w", subAbs, closeErr)
			}
		}
	}

	tmpDefaultsDir, err := os.MkdirTemp("", "backlogit-init-defaults-*")
	if err != nil {
		return fmt.Errorf("create defaults staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDefaultsDir) }()

	if err := config.WriteDefaults(tmpDefaultsDir); err != nil {
		return fmt.Errorf("render defaults: %w", err)
	}
	if err := config.WriteMigrationDefaults(tmpDefaultsDir); err != nil {
		return fmt.Errorf("render migration defaults: %w", err)
	}

	for _, name := range []string{"config.yaml", "registry.yaml", "header-def.yaml", "stash.jsonl", "hooks.yaml", "migration.yaml"} {
		data, readErr := os.ReadFile(filepath.Join(tmpDefaultsDir, name))
		if readErr != nil {
			return fmt.Errorf("read staged default %s: %w", name, readErr)
		}
		if err := writeBootstrapFileIfNotExistsInRoot(storageHandle, storageAbs, name, data); err != nil {
			return err
		}
	}

	templatesAbs := filepath.Join(storageAbs, "templates")
	tplEntries, err := os.ReadDir(filepath.Join(tmpDefaultsDir, "templates"))
	if err != nil {
		return fmt.Errorf("list staged templates: %w", err)
	}
	for _, entry := range tplEntries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(tmpDefaultsDir, "templates", entry.Name()))
		if readErr != nil {
			return fmt.Errorf("read staged template %s: %w", entry.Name(), readErr)
		}
		if err := writeBootstrapFileIfNotExistsInRoot(templatesHandle, templatesAbs, entry.Name(), data); err != nil {
			return err
		}
	}
	return nil
}

func resolveInitTargetToolsDir(cmd *cobra.Command, cleanRoot, toolsDirFlag string) (string, error) {
	defaultDir, err := defaultTargetToolsDir(cleanRoot)
	if err != nil {
		return "", err
	}

	validateCandidate := func(raw string) (string, error) {
		if raw != strings.TrimSpace(raw) || strings.TrimSpace(raw) == "" {
			return "", fmt.Errorf("target tools directory %q must not be blank or whitespace-padded: %w", raw, corerrors.ErrValidation)
		}
		candidate := strings.TrimPrefix(strings.TrimPrefix(raw, "./"), `.\`)
		cleanRel, valErr := validateTargetToolsDirInput(candidate)
		if valErr != nil {
			return "", valErr
		}
		if _, wsRoot, openErr := openOrCreateValidatedWorkspaceRoot(cleanRoot, false); openErr == nil {
			defer func() { _ = wsRoot.Close() }()
			if dirErr := validateDaemonRelExistingDirs(wsRoot, cleanRoot, cleanRel); dirErr != nil {
				return "", dirErr
			}
		} else if !errors.Is(openErr, os.ErrNotExist) {
			return "", openErr
		}
		return filepath.ToSlash(cleanRel), nil
	}

	if cmd != nil && cmd.Flags().Changed("tools-dir") {
		return validateCandidate(toolsDirFlag)
	}

	if cmd != nil {
		if jsonrpcEnabled, flagErr := cmd.Flags().GetBool("jsonrpc"); flagErr == nil && jsonrpcEnabled {
			return defaultDir, nil
		}
	}

	var in io.Reader
	var out io.Writer
	if cmd != nil {
		in = cmd.InOrStdin()
		out = cmd.OutOrStdout()
	}
	shouldPrompt := false
	if in != nil {
		if inFile, ok := in.(*os.File); ok {
			isInTerm := term.IsTerminal(int(inFile.Fd()))
			isOutTerm := false
			if outFile, outOK := out.(*os.File); outOK {
				isOutTerm = term.IsTerminal(int(outFile.Fd()))
			}
			if isInTerm && isOutTerm {
				shouldPrompt = true
			} else if !isInTerm {
				if fi, statErr := inFile.Stat(); statErr == nil && (fi.Mode()&os.ModeNamedPipe != 0 || (fi.Mode().IsRegular() && fi.Size() > 0)) {
					shouldPrompt = true
				}
			}
		} else {
			shouldPrompt = true
		}
	}
	if !shouldPrompt {
		return defaultDir, nil
	}

	if _, writeErr := fmt.Fprintf(out, "Target tool install directory [.bin, bin, or custom workspace-relative path] (default: %s): ", defaultDir); writeErr != nil {
		return "", fmt.Errorf("write target tools directory prompt: %w", writeErr)
	}
	reader := bufio.NewReader(in)
	line, readErr := reader.ReadString('\n')
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return "", fmt.Errorf("read target tools directory prompt: %w", readErr)
	}
	trimmed := strings.TrimRight(line, "\r\n")
	var selected string
	switch trimmed {
	case "":
		selected = defaultDir
	case "1", ".bin":
		selected = ".bin"
	case "2", "bin":
		selected = "bin"
	case "3":
		if _, writeErr := fmt.Fprintf(out, "Enter custom workspace-relative tool directory: "); writeErr != nil {
			return "", fmt.Errorf("write custom tools directory prompt: %w", writeErr)
		}
		line2, readErr2 := reader.ReadString('\n')
		if readErr2 != nil && !errors.Is(readErr2, io.EOF) {
			return "", fmt.Errorf("read custom tools directory prompt: %w", readErr2)
		}
		trimmed2 := strings.TrimRight(line2, "\r\n")
		if strings.TrimSpace(trimmed2) == "" {
			return "", fmt.Errorf("custom target tools directory cannot be empty: %w", corerrors.ErrValidation)
		}
		selected = trimmed2
	default:
		selected = trimmed
	}
	return validateCandidate(selected)
}

func newInitCommand(cwd *string) *cobra.Command {
	var toolsDirFlag string
	cmd := &cobra.Command{
		Use:     "init [path]",
		Aliases: []string{"install"},
		Short:   "Initialize a new backlogit workspace",
		Long: `Initialize a backlogit workspace in the target directory.

This creates the .backlog storage root, logs directory, canonical stash JSONL
file, default YAML configuration files, default artifact templates, installs the
backlogit binary into the chosen target tool directory, and configures .mcp.json.`,
		Example: `  backlogit init
  backlogit init --tools-dir .bin
  backlogit init D:\Source\MyProject`,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := *cwd
			if len(args) > 0 {
				root = args[0]
			}
			absRoot, err := filepath.Abs(root)
			if err != nil {
				return fmt.Errorf("resolve workspace root: %w", err)
			}
			cleanRoot := filepath.Clean(absRoot)

			toolsDirRel, err := resolveInitTargetToolsDir(cmd, cleanRoot, toolsDirFlag)
			if err != nil {
				return err
			}

			validatedRoot, wsRoot, err := openOrCreateValidatedWorkspaceRoot(cleanRoot, true)
			if err != nil {
				return err
			}
			wsClosed := false
			defer func() {
				if !wsClosed {
					_ = wsRoot.Close()
				}
			}()
			cleanRoot = validatedRoot

			storageDirName := core.WorkspaceRootCandidates()[0]
			if cmd.CalledAs() == "install" {
				resolvedDir, err := resolveInstallStorageDir(cleanRoot)
				if err != nil {
					return err
				}
				storageDirName = resolvedDir
			} else {
				legacyName := core.WorkspaceRootCandidates()[1]
				if info, err := wsRoot.Lstat(legacyName); err == nil {
					legacyLink, linkErr := core.IsSymlinkOrReparsePoint(info, filepath.Join(cleanRoot, legacyName))
					if linkErr == nil && !legacyLink && info.IsDir() {
						return fmt.Errorf("workspace already exists at .backlogit; use 'backlogit migrate --workspace-dir' to rename it to .backlog")
					}
					return fmt.Errorf("legacy workspace dir %s is redirected or not a directory: %w", legacyName, corerrors.ErrValidation)
				} else if !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("stat legacy workspace dir: %w", err)
				}
			}

			dir := filepath.Join(cleanRoot, storageDirName)
			storageInfo, statErr := wsRoot.Lstat(storageDirName)
			if errors.Is(statErr, os.ErrNotExist) {
				if mkErr := wsRoot.Mkdir(storageDirName, 0o755); mkErr != nil {
					return fmt.Errorf("create workspace dir: %w", mkErr)
				}
				storageInfo, statErr = wsRoot.Lstat(storageDirName)
			}
			if statErr != nil {
				return fmt.Errorf("inspect workspace dir: %w", statErr)
			}
			isReparse, inspectErr := core.IsSymlinkOrReparsePoint(storageInfo, dir)
			if inspectErr != nil {
				return fmt.Errorf("inspect workspace dir: %w", inspectErr)
			}
			if isReparse || storageInfo.Mode()&os.ModeIrregular != 0 || !storageInfo.IsDir() {
				return fmt.Errorf("workspace dir %s is redirected or not a directory: %w", dir, corerrors.ErrValidation)
			}
			storageHandle, err := wsRoot.OpenRoot(storageDirName)
			if err != nil {
				return fmt.Errorf("open workspace dir: %w", err)
			}
			storageClosed := false
			defer func() {
				if !storageClosed {
					_ = storageHandle.Close()
				}
			}()
			openedStorage, err := storageHandle.Stat(".")
			if err != nil || !os.SameFile(storageInfo, openedStorage) {
				return fmt.Errorf("workspace dir %s identity changed after open: %w", dir, corerrors.ErrValidation)
			}

			if err := writeWorkspaceBootstrapDefaults(wsRoot, storageHandle, cleanRoot, storageDirName); err != nil {
				return err
			}

			storageClosed = true
			if err := storageHandle.Close(); err != nil {
				return fmt.Errorf("close workspace dir: %w", err)
			}
			wsClosed = true
			if err := wsRoot.Close(); err != nil {
				return fmt.Errorf("close workspace root: %w", err)
			}

			binName, err := installWorkspaceRuntimeExecutable(cleanRoot, storageDirName)
			if err != nil {
				return fmt.Errorf("install workspace runtime executable: %w", err)
			}
			if _, err := installWorkspaceRuntimeExecutable(cleanRoot, toolsDirRel); err != nil {
				return fmt.Errorf("install workspace tool executable: %w", err)
			}
			if err := ensureWorkspaceRuntimeIgnored(cleanRoot, toolsDirRel, binName); err != nil {
				return fmt.Errorf("configure ignore rules for workspace runtime: %w", err)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Initialized backlogit workspace at %s\n", dir); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&toolsDirFlag, "tools-dir", "", "workspace-relative target tool directory for backlogit binary and .mcp.json (default .bin or bin)")
	return cmd
}

func newSyncCommand(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Rehydrate the SQLite index from Markdown source files",
		Long: `Rebuild the backlogit SQLite cache from the Markdown and JSONL files in
the workspace.

Use this after making manual changes to .backlog or .backlogit files or when you want to
force the disposable cache to match the file-backed source of truth.`,
		Example: `  backlogit sync
  backlogit --cwd D:\Source\MyProject sync`,
		RunE: func(cmd *cobra.Command, _ []string) (retErr error) {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer func() {
				if cerr := ws.Close(); cerr != nil {
					retErr = errors.Join(retErr, fmt.Errorf("close workspace: %w", cerr))
				}
			}()

			if _, migrateErr := core.MigrateDBOnlyLinksBeforeRehydrate(ctx, ws); migrateErr != nil {
				return migrateErr
			}

			// 070.002-T: thread the CLI's configured logger through Rehydrate's
			// dependency-injection seam. applyLogLevel has already installed the
			// level/format-configured handler as the slog default, so this passes
			// the same logger explicitly rather than relying on the implicit
			// global fallback.
			count, err := db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB, db.WithLogger(slog.Default()))
			if err != nil {
				return fmt.Errorf("sync: %w", err)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Indexed %d artifacts\n", count); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}
}

// serveMCPStdio is a test seam for the stdio transport.
var serveMCPStdio func(*mcpinternal.Server) error

func newMCPCommand(cwd *string) *cobra.Command {
	var replace bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the backlogit MCP stdio server",
		Long: `Start the backlogit Model Context Protocol server over stdio.

Use this command from MCP-capable clients such as GitHub Copilot CLI, Claude
Code, or Cursor to expose backlogit workspace tools to agents.`,
		Example: `  backlogit mcp
  backlogit mcp --replace
  backlogit mcp status --format json
  backlogit mcp stop
  backlogit --cwd D:\Source\MyProject mcp`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			noUpdateCheck, err := noUpdateCheckFlagValue(cmd)
			if err != nil {
				return err
			}
			if serveMCPStdio == nil {
				return RunMCPSessionShim(cmd.Context(), *cwd, replace, cmd.InOrStdin(), cmd.OutOrStdout())
			}
			release, err := AcquireMCPDaemonLock(cmd.Context(), *cwd, replace)
			if err != nil {
				return err
			}
			defer func() { _ = release() }()
			s, err := openMCPServer(context.Background(), *cwd)
			if err != nil {
				return err
			}
			s.NoUpdateCheck = noUpdateCheck
			return serveMCPStdio(s)
		},
	}
	cmd.Flags().BoolVar(&replace, "replace", false, "stop any running workspace MCP daemon before starting")
	cmd.AddCommand(newMCPStatusCommand(cwd), newMCPStopCommand(cwd),
		newMCPDaemonCommand(cwd), newMCPPrewarmCommand(cwd))
	return cmd
}

func newMCPDaemonCommand(cwd *string) *cobra.Command {
	var replace bool
	var idleTimeout time.Duration
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Host the workspace background MCP daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunMCPDaemon(cmd.Context(), *cwd, replace, idleTimeout)
		},
	}
	cmd.Flags().BoolVar(&replace, "replace", false, "stop any running workspace MCP daemon before starting")
	cmd.Flags().DurationVar(&idleTimeout, "idle-timeout", 15*time.Minute, "stop after this duration with no active sessions")
	return cmd
}

func newMCPPrewarmCommand(cwd *string) *cobra.Command {
	var replace bool
	var idleTimeout time.Duration
	var format string
	cmd := &cobra.Command{
		Use:   "prewarm",
		Short: "Start or attach to the workspace background MCP daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			status, err := PrewarmMCPDaemon(cmd.Context(), *cwd, replace, idleTimeout)
			if err != nil {
				return err
			}
			if format == "json" {
				return writeMCPDaemonJSON(cmd, status)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "MCP daemon ready: pid %d, active_sessions %d, endpoint %s\n",
				status.PID, status.ActiveSessions, status.Endpoint)
			return wrapOutputErr(err)
		},
	}
	cmd.Flags().BoolVar(&replace, "replace", false, "stop any running workspace MCP daemon before starting")
	cmd.Flags().DurationVar(&idleTimeout, "idle-timeout", 15*time.Minute, "stop after this duration with no active sessions")
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

type mcpStatusToolchainOutput struct {
	MCPDaemonStatus
	Toolchain *toolchain.ToolchainVerificationReport `json:"toolchain,omitempty"`
}

func newMCPStatusCommand(cwd *string) *cobra.Command {
	var format string
	var includeToolchain, strict bool
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Report whether a backlogit MCP daemon is running for the workspace",
		Example: "  backlogit mcp status --format json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			status, err := InspectMCPDaemonStatus(*cwd)
			if err != nil {
				return err
			}
			if includeToolchain || strict {
				report, err := toolchain.AuditRepositoryToolchain(cmd.Context(), *cwd)
				if err != nil {
					return fmt.Errorf("inspect MCP status toolchain: %w", err)
				}
				output := mcpStatusToolchainOutput{MCPDaemonStatus: status, Toolchain: report}
				if err := writeMCPStatusToolchain(cmd, format, output); err != nil {
					return err
				}
				if strict && report.Violations > 0 {
					return fmt.Errorf("MCP toolchain audit found %d violations: %w", report.Violations, corerrors.ErrValidation)
				}
				return nil
			}
			if format == "json" {
				return writeMCPDaemonJSON(cmd, status)
			}
			if !status.Running {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "MCP daemon not running (lock: %s)\n", status.LockPath)
				return wrapOutputErr(err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "MCP daemon running: pid %d, started %s, version %s, active_sessions %d, endpoint %s (lock: %s)\n",
				status.PID, status.StartedAt.Format(time.RFC3339), status.Version, status.ActiveSessions, status.Endpoint, status.LockPath)
			return wrapOutputErr(err)
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	cmd.Flags().BoolVar(&includeToolchain, "toolchain", false, "include workspace all-tools Level 1 pinning and containment audit")
	cmd.Flags().BoolVar(&strict, "strict", false, "return non-zero exit status when toolchain violations or non-conformant local tools are present")
	return cmd
}

func writeMCPStatusToolchain(cmd *cobra.Command, format string, output mcpStatusToolchainOutput) error {
	if format == "json" {
		return writeMCPDaemonJSON(cmd, output)
	}
	var text strings.Builder
	if output.Running {
		fmt.Fprintf(&text, "MCP daemon running: pid %d, started %s, version %s, active_sessions %d, endpoint %s (lock: %s)\n",
			output.PID, output.StartedAt.Format(time.RFC3339), output.Version, output.ActiveSessions, output.Endpoint, output.LockPath)
	} else {
		fmt.Fprintf(&text, "MCP daemon not running (lock: %s)\n", output.LockPath)
	}
	report := output.Toolchain
	fmt.Fprintf(&text, "Toolchain: valid %t, violations %d, containment %s (OS/container sandbox deferred: %t)\n",
		report.Valid, report.Violations, report.ContainmentLevel, report.OSContainerSandboxDeferred)
	for _, findings := range [][]toolchain.ToolVerificationFinding{report.Tools, report.HostPrerequisites} {
		for _, f := range findings {
			fmt.Fprintf(&text, "  %s: %s [%s, %s] %s\n", f.ToolID, f.ContainmentClass, f.Status, f.Code, f.Message)
			if f.Status == "violation" {
				fmt.Fprintf(&text, "    Remediation: %s\n", f.Remediation)
			}
		}
	}
	_, err := io.WriteString(cmd.OutOrStdout(), text.String())
	return wrapOutputErr(err)
}

func newMCPStopCommand(cwd *string) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "stop",
		Short:   "Stop the backlogit MCP daemon running for the workspace",
		Example: "  backlogit mcp stop --format json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := StopMCPDaemon(cmd.Context(), *cwd)
			if err != nil {
				return err
			}
			if format == "json" {
				return writeMCPDaemonJSON(cmd, res)
			}
			if !res.Stopped {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), "MCP daemon not running")
				return wrapOutputErr(err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Stopped MCP daemon pid %d\n", res.PID)
			return wrapOutputErr(err)
		},
	}
	cmd.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return cmd
}

func wrapOutputErr(err error) error {
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func writeMCPDaemonJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}

func openMCPServer(ctx context.Context, rootPath string) (*mcpinternal.Server, error) {
	ws, err := core.NewWorkspace(ctx, rootPath)
	if err == nil {
		s := mcpinternal.NewServer(ws)
		wireMCPMetadataProvider(s, rootPath)
		return s, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		s := mcpinternal.NewServerForRoot(rootPath)
		wireMCPMetadataProvider(s, rootPath)
		return s, nil
	}
	diagnostic, diagnosticErr := core.NewDiagnosticWorkspace(ctx, rootPath)
	if diagnosticErr == nil {
		if closeErr := diagnostic.Close(); closeErr != nil {
			return nil, fmt.Errorf("close diagnostic workspace: %w", closeErr)
		}
		s := mcpinternal.NewServerForRoot(rootPath)
		wireMCPMetadataProvider(s, rootPath)
		return s, nil
	}
	return nil, fmt.Errorf("open workspace: %w", err)
}

// wireMCPMetadataProvider injects the CLI command catalog into the MCP server so
// its metadata catalog reaches parity with the CLI metadata path. The cli
// package owns the cobra command tree, so it supplies the described commands to
// the mcp package (which cannot import cli without an import cycle).
func wireMCPMetadataProvider(s *mcpinternal.Server, cwd string) {
	s.CLICommandProvider = func() []core.CommandInfo {
		return core.DescribeCLICommands(buildMetadataRootCommand(cwd))
	}
	s.AttestationPins = envPinProvider{}
}
