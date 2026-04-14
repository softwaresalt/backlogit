package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/events"
	"github.com/backlogit/backlogit/internal/hooks"
)

// Workspace coordinates cross-store operations across Markdown, SQLite, and JSONL.
type Workspace struct {
	RootPath  string
	Config    *config.WorkspaceConfig
	DB        *sql.DB
	HeaderDef *config.HeaderDefConfig
	Templates []*config.TemplateConfig
	// HookRunner dispatches lifecycle hooks. May be nil in tests that
	// construct Workspace{} directly; all call sites must nil-guard.
	HookRunner *hooks.HookRunner
	// Hooks holds the parsed hooks.yaml configuration.
	Hooks *config.HooksConfig
	// webhookNotifier is stored for shutdown draining. Unexported.
	webhookNotifier interface{ Shutdown(context.Context) error }
}

// WorkspaceStorageRoot returns the .backlogit directory for a workspace root.
func WorkspaceStorageRoot(rootPath string) string {
	return filepath.Join(rootPath, ".backlogit")
}

// WorkspaceLogsRoot returns the .backlogit\logs directory for a workspace root.
func WorkspaceLogsRoot(rootPath string) string {
	return filepath.Join(WorkspaceStorageRoot(rootPath), "logs")
}

// NewWorkspace creates a workspace, loads config, opens DB, and ensures schema.
func NewWorkspace(ctx context.Context, rootPath string) (*Workspace, error) {
	resolvedRoot, err := resolveWorkspaceRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}

	backlogitDir := WorkspaceStorageRoot(resolvedRoot)
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
		RootPath:   resolvedRoot,
		Config:     cfg,
		DB:         database,
		HeaderDef:  headerDef,
		Templates:  templates,
		HookRunner: hookRunner,
		Hooks:      hooksCfg,
	}
	// Assign only when non-nil to avoid storing a typed-nil *WebhookNotifier
	// inside the interface, which would bypass the != nil guard in Close().
	if webhookNotifier != nil {
		workspace.webhookNotifier = webhookNotifier
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
	cleanRoot := filepath.Clean(rootPath)

	if hasWorkspaceConfig(WorkspaceStorageRoot(cleanRoot)) {
		return cleanRoot, nil
	}
	if filepath.Base(cleanRoot) == ".backlogit" && hasWorkspaceConfig(cleanRoot) {
		return filepath.Dir(cleanRoot), nil
	}

	entries, err := os.ReadDir(cleanRoot)
	if err != nil {
		return "", fmt.Errorf("read workspace root %s: %w", cleanRoot, err)
	}

	var matches []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		candidateRoot := filepath.Join(cleanRoot, entry.Name())
		if hasWorkspaceConfig(WorkspaceStorageRoot(candidateRoot)) {
			matches = append(matches, candidateRoot)
		}
	}

	if len(matches) == 1 {
		return matches[0], nil
	}

	return cleanRoot, nil
}

func hasWorkspaceConfig(storageRoot string) bool {
	info, err := os.Stat(filepath.Join(storageRoot, "config.yaml"))
	if err != nil {
		return false
	}
	return !info.IsDir()
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
