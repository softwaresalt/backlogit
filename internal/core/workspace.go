package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
)

// workspaceRootCandidates lists the supported storage-root directory names in
// precedence order. Private and never derived from config.
var workspaceRootCandidates = [...]string{".backlog", ".backlogit"}

const workspaceDirOverrideEnvVar = "BACKLOGIT_WORKSPACE_DIR"

// Workspace coordinates cross-store operations across Markdown, SQLite, and JSONL.
type Workspace struct {
	RootPath    string
	StorageRoot string
	Config      *config.WorkspaceConfig
	DB          *sql.DB
	HeaderDef   *config.HeaderDefConfig
	Templates   []*config.TemplateConfig
	// writeStashEntriesAtomically allows stash persistence to be overridden per
	// workspace in tests without introducing a package-level mutable hook.
	writeStashEntriesAtomically func(path, content string) error
	// HookRunner dispatches lifecycle hooks. May be nil in tests that
	// construct Workspace{} directly; all call sites must nil-guard.
	HookRunner *hooks.HookRunner
	// Hooks holds the parsed hooks.yaml configuration.
	Hooks *config.HooksConfig
	// GateBroker is the pre-task-completion gate broker (082-F). It is nil when
	// the gate is disabled (enabled:false) or when a Workspace is constructed
	// directly in tests without wiring — a nil broker means the gate is skipped,
	// preserving pre-gate completion behavior. Gate-specific tests inject a broker
	// with fake Runner/Git/Version seams.
	GateBroker *gate.Broker
	// gateConfig retains the normalized gate config used to build GateBroker so the
	// completion path can consult terminal_statuses, evidence_required, and
	// force_cli_only without re-parsing.
	gateConfig config.PreTaskCompletionGateConfig
	// gateEvidenceAppend allows the gate evidence appender to be overridden per
	// workspace in tests (mirroring writeStashEntriesAtomically) so a forced
	// append failure can exercise the evidence_required rollback contract without
	// OS-level filesystem tricks. Nil means use the real appendItemEventErr.
	gateEvidenceAppend func(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) error
	// webhookNotifier is stored for shutdown draining. Unexported.
	webhookNotifier interface{ Shutdown(context.Context) error }
}

// WorkspaceRootCandidates returns a fresh copy of the supported storage-root
// directory names in precedence order.
func WorkspaceRootCandidates() []string {
	out := make([]string, len(workspaceRootCandidates))
	copy(out, workspaceRootCandidates[:])
	return out
}

// WorkspaceStorageRoot returns the resolved storage root when it can be
// determined from the filesystem and otherwise falls back to the legacy
// .backlogit location.
func WorkspaceStorageRoot(rootPath string) string {
	if storageRoot, err := resolveStorageRoot(rootPath); err == nil {
		return storageRoot
	}
	return filepath.Join(rootPath, workspaceRootCandidates[len(workspaceRootCandidates)-1])
}

// WorkspaceLogsRoot returns the logs directory for a workspace root.
func WorkspaceLogsRoot(rootPath string) string {
	return filepath.Join(WorkspaceStorageRoot(rootPath), "logs")
}

func workspaceStorageRoot(ws *Workspace) string {
	if ws != nil && ws.StorageRoot != "" {
		return ws.StorageRoot
	}
	if ws == nil {
		return ""
	}
	return WorkspaceStorageRoot(ws.RootPath)
}

// NewWorkspace creates a workspace, loads config, opens DB, and ensures schema.
func NewWorkspace(ctx context.Context, rootPath string) (*Workspace, error) {
	backlogitDir, err := resolveStorageRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	resolvedRoot := filepath.Dir(backlogitDir)
	cfg, err := config.Load(ctx, backlogitDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	dbPath := filepath.Join(backlogitDir, "backlogit.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Load header-def (optional — nil if file is absent).
	headerDef, hdErr := config.LoadHeaderDef(backlogitDir)
	if hdErr != nil {
		if !errors.Is(hdErr, os.ErrNotExist) {
			database.Close()
			return nil, fmt.Errorf("load header-def: %w", hdErr)
		}
		headerDef = nil
	}

	// Load templates (optional — nil if templates dir is absent).
	templates, templatesErr := config.LoadTemplates(filepath.Join(backlogitDir, "templates"))
	if templatesErr != nil {
		database.Close()
		return nil, fmt.Errorf("load templates: %w", templatesErr)
	}

	// Load hooks configuration and initialize hook runner.
	hooksCfg, hooksErr := config.LoadHooks(backlogitDir)
	if hooksErr != nil {
		database.Close()
		return nil, fmt.Errorf("load hooks config: %w", hooksErr)
	}

	var hookRunner *hooks.HookRunner
	if hooksCfg.Enabled {
		hookRunner = hooks.NewHookRunner()

		// Register built-in pre-hooks.
		if hooksCfg.Lifecycle.ValidateTransition {
			transitions := hooksCfg.Lifecycle.Transitions
			hookRunner.Register(hooks.HookUpdateArtifact, hooks.PhasePre, hooks.HookRegistration{
				Name:     "validate_status_transition",
				Priority: 20,
				Fn:       hooks.ValidateStatusTransition(transitions),
			})
		}

		// Register built-in post-hooks.
		if hooksCfg.Lifecycle.EmitEvents {
			writer := events.NewHookEventWriter(backlogitDir)
			appender := &hookEventAppenderAdapter{writer: writer}
			hooks.RegisterEmitHookEvent(hookRunner, appender)
		}

		// Always register informational index stale hook.
		hooks.RegisterLogIndexStale(hookRunner)
	}

	// Wire webhook notifier if endpoints are configured and hooks are enabled.
	var webhookNotifier *hooks.WebhookNotifier
	if hooksCfg.Enabled && len(hooksCfg.Notifications.Endpoints) > 0 && hookRunner != nil {
		endpoints := make([]hooks.WebhookEndpointConfig, 0, len(hooksCfg.Notifications.Endpoints))
		for _, ep := range hooksCfg.Notifications.Endpoints {
			filter := make(map[string]struct{}, len(ep.EventFilter))
			for _, f := range ep.EventFilter {
				filter[f] = struct{}{}
			}
			timeout := time.Duration(ep.TimeoutSecs) * time.Second
			if timeout <= 0 {
				timeout = 10 * time.Second
			}
			endpoints = append(endpoints, hooks.WebhookEndpointConfig{
				URL:         ep.URL,
				EventFilter: filter,
				Headers:     ep.Headers,
				Timeout:     timeout,
			})
		}
		rateLimit := hooksCfg.Notifications.RateLimit
		if rateLimit <= 0 {
			rateLimit = 10
		}
		webhookNotifier = hooks.NewWebhookNotifier(endpoints, rateLimit, slog.Default())
		hooks.RegisterWebhookNotifier(hookRunner, webhookNotifier)
	}

	if headerDef != nil {
		if err := db.EnsureSchemaWithExtensions(database, headerDef); err != nil {
			database.Close()
			return nil, fmt.Errorf("ensure schema: %w", err)
		}
	} else {
		if err := db.EnsureSchema(database); err != nil {
			database.Close()
			return nil, fmt.Errorf("ensure schema: %w", err)
		}
	}

	workspace := &Workspace{
		StorageRoot:                 backlogitDir,
		RootPath:                    resolvedRoot,
		Config:                      cfg,
		DB:                          database,
		HeaderDef:                   headerDef,
		Templates:                   templates,
		writeStashEntriesAtomically: writeStringAtomically,
		HookRunner:                  hookRunner,
		Hooks:                       hooksCfg,
	}
	// Assign only when non-nil to avoid storing a typed-nil *WebhookNotifier
	// inside the interface, which would bypass the != nil guard in Close().
	if webhookNotifier != nil {
		workspace.webhookNotifier = webhookNotifier
	}

	// Wire the pre-task-completion gate broker (082-F) unless explicitly disabled.
	// The broker is skipped entirely when enabled:false (kill switch); under auto
	// it fails open at run time when autoharness is unresolvable, and under true it
	// fails closed. A nil broker (bare test workspace) preserves pre-gate behavior.
	if hooksCfg != nil {
		gateCfg := hooksCfg.Lifecycle.PreTaskCompletionGate
		gateCfg.Normalize()
		workspace.gateConfig = gateCfg
		if gateCfg.Enabled != "false" {
			workspace.GateBroker = buildGateBroker(resolvedRoot, gateCfg)
		}
	}

	// F-7 migration guard: write any DB-only links to Markdown frontmatter
	// BEFORE any rehydration that would clear item_links. This is idempotent
	// and best-effort — failures are logged but do not abort initialization.
	if _, migrateErr := MigrateDBOnlyLinks(ctx, workspace); migrateErr != nil {
		slog.WarnContext(ctx, "migrate db-only links failed during workspace init", "error", migrateErr)
	}

	if err := recoverPendingShipmentOperations(ctx, workspace); err != nil {
		database.Close()
		return nil, fmt.Errorf("recover shipment operations: %w", err)
	}
	return workspace, nil
}

func resolveWorkspaceRoot(rootPath string) (string, error) {
	storageRoot, err := resolveStorageRoot(rootPath)
	if err != nil {
		return "", err
	}
	return filepath.Dir(storageRoot), nil
}

func resolveStorageRoot(rootPath string) (string, error) {
	cleanRoot, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return "", fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}

	if storageRoot, resolved, err := resolveDirectStorageRoot(cleanRoot); resolved {
		return storageRoot, err
	}

	override, hasOverride, err := workspaceDirOverride()
	if err != nil {
		return "", err
	}

	if hasOverride {
		storageRoot, present, probeErr := probeWorkspaceCandidate(cleanRoot, override, nil)
		if probeErr != nil {
			return "", probeErr
		}
		if !present {
			return "", fmt.Errorf("workspace storage root %q not found under %s: %w", override, cleanRoot, os.ErrNotExist)
		}
		return storageRoot, nil
	}

	entries, err := os.ReadDir(cleanRoot)
	if err != nil {
		return "", fmt.Errorf("read workspace root %s: %w", cleanRoot, err)
	}

	matches := make([]string, 0, len(workspaceRootCandidates))
	matchNames := make([]string, 0, len(workspaceRootCandidates))
	for _, candidate := range workspaceRootCandidates {
		storageRoot, present, probeErr := probeWorkspaceCandidate(cleanRoot, candidate, entries)
		if probeErr != nil {
			return "", probeErr
		}
		if present {
			matches = append(matches, storageRoot)
			matchNames = append(matchNames, candidate)
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("workspace storage root not found under %s: %w", cleanRoot, os.ErrNotExist)
	case 1:
		return matches[0], nil
	default:
		return "", &blerrors.AmbiguousWorkspaceRootError{Roots: matchNames}
	}
}

func resolveDirectStorageRoot(rootPath string) (string, bool, error) {
	base := filepath.Base(rootPath)
	parent := filepath.Dir(rootPath)

	for _, candidate := range workspaceRootCandidates {
		if base == candidate {
			storageRoot, present, err := probeWorkspaceCandidate(parent, candidate, nil)
			if err != nil {
				return "", true, err
			}
			if !present {
				return "", true, fmt.Errorf("workspace storage root %s: %w", rootPath, os.ErrNotExist)
			}
			return storageRoot, true, nil
		}
		if strings.EqualFold(base, candidate) {
			return "", true, fmt.Errorf("workspace storage root %q must use the exact supported case", base)
		}
	}

	return "", false, nil
}

func workspaceDirOverride() (string, bool, error) {
	override, ok := os.LookupEnv(workspaceDirOverrideEnvVar)
	if !ok {
		return "", false, nil
	}

	validated, err := validateWorkspaceDirOverride(override)
	if err != nil {
		return "", true, err
	}
	return validated, true, nil
}

func validateWorkspaceDirOverride(override string) (string, error) {
	if override == "" {
		return "", fmt.Errorf("%s is set but empty", workspaceDirOverrideEnvVar)
	}
	if strings.ContainsRune(override, '\x00') {
		return "", fmt.Errorf("%s contains a NUL byte", workspaceDirOverrideEnvVar)
	}
	if strings.ContainsAny(override, `/\`) {
		return "", fmt.Errorf("%s must be one of %s", workspaceDirOverrideEnvVar, strings.Join(WorkspaceRootCandidates(), ", "))
	}
	if override == "." || override == ".." {
		return "", fmt.Errorf("%s must be one of %s", workspaceDirOverrideEnvVar, strings.Join(WorkspaceRootCandidates(), ", "))
	}
	if filepath.IsAbs(override) {
		return "", fmt.Errorf("%s must be one of %s", workspaceDirOverrideEnvVar, strings.Join(WorkspaceRootCandidates(), ", "))
	}
	if filepath.VolumeName(override) != "" || strings.HasPrefix(override, `\\`) {
		return "", fmt.Errorf("%s must be one of %s", workspaceDirOverrideEnvVar, strings.Join(WorkspaceRootCandidates(), ", "))
	}
	if slices.Contains(workspaceRootCandidates[:], override) {
		return override, nil
	}
	for _, candidate := range workspaceRootCandidates {
		if strings.EqualFold(candidate, override) {
			return "", fmt.Errorf("%s must use the exact supported case", workspaceDirOverrideEnvVar)
		}
	}
	return "", fmt.Errorf("%s must be one of %s", workspaceDirOverrideEnvVar, strings.Join(WorkspaceRootCandidates(), ", "))
}

func probeWorkspaceCandidate(rootPath, candidate string, entries []os.DirEntry) (string, bool, error) {
	if entries == nil {
		var err error
		entries, err = os.ReadDir(rootPath)
		if err != nil {
			return "", false, fmt.Errorf("read workspace root %s: %w", rootPath, err)
		}
	}

	actualName, found, exactCase := findWorkspaceCandidateEntry(entries, candidate)
	if found && !exactCase {
		aliasPath := filepath.Join(rootPath, actualName)
		aliasInfo, err := os.Lstat(aliasPath)
		if err != nil {
			return "", false, fmt.Errorf("lstat workspace candidate alias %s: %w", aliasPath, err)
		}
		candidatePath := filepath.Join(rootPath, candidate)
		if candidateInfo, err := os.Lstat(candidatePath); err == nil && os.SameFile(aliasInfo, candidateInfo) {
			return "", false, fmt.Errorf("workspace candidate alias %q must use the exact supported case %q", actualName, candidate)
		}
		return "", false, fmt.Errorf("workspace candidate alias %q must use the exact supported case %q", actualName, candidate)
	}
	if !found {
		return "", false, nil
	}

	candidatePath := filepath.Join(rootPath, candidate)
	info, err := os.Lstat(candidatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("lstat workspace candidate %s: %w", candidatePath, err)
	}
	if !info.IsDir() {
		return "", false, fmt.Errorf("workspace candidate %s is not a directory", candidatePath)
	}

	reparsePoint, err := isSymlinkOrReparse(info, candidatePath)
	if err != nil {
		return "", false, fmt.Errorf("inspect workspace candidate %s: %w", candidatePath, err)
	}
	if reparsePoint {
		return "", false, fmt.Errorf("workspace candidate %s is a symlink or reparse point", candidatePath)
	}

	if !pathContained(rootPath, candidatePath) {
		return "", false, fmt.Errorf("workspace candidate %s escapes workspace root %s", candidatePath, rootPath)
	}

	realRoot, err := filepath.EvalSymlinks(rootPath)
	if err != nil {
		return "", false, fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}
	realCandidate, err := filepath.EvalSymlinks(candidatePath)
	if err != nil {
		return "", false, fmt.Errorf("resolve workspace candidate %s: %w", candidatePath, err)
	}
	if !pathContained(realRoot, realCandidate) {
		return "", false, fmt.Errorf("workspace candidate %s escapes workspace root %s after resolution", candidatePath, rootPath)
	}

	configPath := filepath.Join(candidatePath, "config.yaml")
	configInfo, err := os.Lstat(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("lstat workspace config %s: %w", configPath, err)
	}

	configReparsePoint, err := isSymlinkOrReparse(configInfo, configPath)
	if err != nil {
		return "", false, fmt.Errorf("inspect workspace config %s: %w", configPath, err)
	}
	if configReparsePoint || !configInfo.Mode().IsRegular() {
		return "", false, fmt.Errorf("workspace config %s must be a regular file", configPath)
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		return "", false, fmt.Errorf("open workspace config %s: %w", configPath, err)
	}
	if err := configFile.Close(); err != nil {
		return "", false, fmt.Errorf("close workspace config %s: %w", configPath, err)
	}

	return candidatePath, true, nil
}

func findWorkspaceCandidateEntry(entries []os.DirEntry, candidate string) (string, bool, bool) {
	var aliasName string
	for _, entry := range entries {
		if entry.Name() == candidate {
			return entry.Name(), true, true
		}
		if strings.EqualFold(entry.Name(), candidate) {
			aliasName = entry.Name()
		}
	}
	if aliasName != "" {
		return aliasName, true, false
	}
	return "", false, false
}

// Close drains any in-flight webhook dispatches and closes the database connection.
func (ws *Workspace) Close() error {
	if ws.webhookNotifier != nil {
		if err := ws.webhookNotifier.Shutdown(context.Background()); err != nil {
			slog.Warn("webhook notifier shutdown error", "error", err)
		}
	}
	if ws.DB != nil {
		return ws.DB.Close()
	}
	return nil
}

// SafeResolve returns an absolute path within the workspace root or an error
// if the target escapes the workspace boundary.
func SafeResolve(workspaceRoot, target string) (string, error) {
	// Convert workspaceRoot to absolute first so both sides of the comparison are absolute.
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	cleanRoot := filepath.Clean(absRoot)

	normalizedTarget := strings.ReplaceAll(target, "\\", string(filepath.Separator))
	abs, err := filepath.Abs(filepath.Join(absRoot, normalizedTarget))
	if err != nil {
		return "", fmt.Errorf("resolve path: %w", err)
	}

	if !strings.HasPrefix(abs, cleanRoot+string(filepath.Separator)) && abs != cleanRoot {
		return "", fmt.Errorf("path escapes workspace boundary: %s", target)
	}
	return abs, nil
}

// hookEventAppenderAdapter adapts HookEventWriter to the hooks.HookEventAppender
// interface, decoupling the events package from the hooks package.
type hookEventAppenderAdapter struct {
	writer *events.HookEventWriter
}

func (a *hookEventAppenderAdapter) AppendEvent(ctx context.Context, payload hooks.HookEventPayload) error {
	event := events.HookEvent{
		EventType: payload.EventType,
		Payload: map[string]any{
			"schema_version": payload.SchemaVersion,
			"item_id":        payload.ItemID,
			"artifact_type":  payload.ArtifactType,
			"actor":          payload.Actor,
		},
	}
	if len(payload.ChangedFields) > 0 {
		event.Payload["changed_fields"] = payload.ChangedFields
	}
	if payload.StatusDelta != nil {
		event.Payload["status_delta"] = map[string]any{
			"from": payload.StatusDelta.From,
			"to":   payload.StatusDelta.To,
		}
	}
	if payload.TitleDelta != nil {
		event.Payload["title_delta"] = map[string]any{
			"from": payload.TitleDelta.From,
			"to":   payload.TitleDelta.To,
		}
	}
	_, err := a.writer.AppendHookEvent(ctx, event)
	return err
}
