package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ArchiveRecord tracks the metadata for a single archived artifact,
// including any cascaded children and failures.
type ArchiveRecord struct {
	ID            string           `json:"id"`
	ArchivedAt    time.Time        `json:"archived_at"`
	ArchivedBy    string           `json:"archived_by"`
	OriginalPath  string           `json:"original_path"`
	ArchivePath   string           `json:"archive_path"`
	CascadedItems []string         `json:"cascaded_items,omitempty"`
	FailedItems   []ArchiveFailure `json:"failed_items,omitempty"`
}

// ArchiveFailure records a child item that failed to archive during cascade.
type ArchiveFailure struct {
	ItemID string `json:"item_id"`
	Error  string `json:"error"`
}

// ArchivePolicy defines rules for automatic archiving of completed artifacts.
type ArchivePolicy struct {
	TerminalStatuses []string `json:"terminal_statuses" yaml:"terminal_statuses"`
	RetentionDays    int      `json:"retention_days" yaml:"retention_days"`
	ArchiveDir       string   `json:"archive_dir" yaml:"archive_dir"`
}

// ArchiveOpt configures optional behavior for ArchiveItem.
type ArchiveOpt func(*archiveConfig)

type archiveConfig struct {
	commitSHA string
	topLevel  *bool // nil means default true
	cascade   bool  // when true, archive children recursively before the parent
}

type artifactMoveKind int

const (
	artifactMoveFilesystem artifactMoveKind = iota
	artifactMoveGit
)

var artifactGitCommandTimeout = 5 * time.Second

type artifactMovePlan struct {
	kind         artifactMoveKind
	workTreeRoot string
	sourceRel    string
	destRel      string
}

// WithCommitSHA attaches a git commit SHA to the archive event for traceability.
func WithCommitSHA(sha string) ArchiveOpt {
	return func(c *archiveConfig) { c.commitSHA = sha }
}

// WithTopLevel sets whether this archive call is a top-level operation.
// When false, post-hooks that emit external events (event emission, webhooks)
// skip to prevent duplicate notifications from nested operations.
// Default is true when not specified.
func WithTopLevel(topLevel bool) ArchiveOpt {
	return func(c *archiveConfig) { c.topLevel = &topLevel }
}

// WithCascade enables recursive archiving of child items before archiving
// the target item. Children are archived deepest-first (subtasks before tasks).
// Default is false.
func WithCascade(cascade bool) ArchiveOpt {
	return func(c *archiveConfig) { c.cascade = cascade }
}

// ArchiveItem moves an artifact from its active directory to the archive directory,
// updating the SQLite index and storing the original path in frontmatter for restoration.
// When WithCascade(true) is set, child items are archived bottom-up before the parent.
func ArchiveItem(ctx context.Context, database *sql.DB, ws *Workspace, itemID string, opts ...ArchiveOpt) (*ArchiveRecord, error) {
	var cfg archiveConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	// Cascade: archive all descendants bottom-up before archiving the target.
	var cascadedItems []string
	var failedItems []ArchiveFailure
	if cfg.cascade {
		cascadedItems, failedItems = archiveDescendants(ctx, database, ws, itemID, cfg)
	}

	backlogDir := workspaceStorageRoot(ws)
	currentPath, err := FindArtifactPath(ctx, ws, itemID)
	if err != nil {
		return nil, fmt.Errorf("find artifact: %w", err)
	}

	archiveDir := filepath.Join(backlogDir, "archive")
	if err := mkdirAllDurable(archiveDir, WorkspaceDurableWrites(ws)); err != nil {
		return nil, fmt.Errorf("create archive dir: %w", err)
	}

	// 060.002-T: When FindArtifactPath returns an archive-dir path but a queue
	// copy also exists for the same ID (e.g., from a half-completed archive),
	// prefer the queue copy as the canonical source so the queue is fully drained.
	if filepath.Clean(filepath.Dir(currentPath)) == filepath.Clean(archiveDir) {
		queueDir := filepath.Join(backlogDir, queueRootDir(ws))
		if queuePath, queueErr := findArtifactInDir(queueDir, itemID); queueErr == nil && queuePath != "" {
			currentPath = queuePath
		}
	}

	// Stamp the original path into frontmatter for later restoration.
	raw, err := os.ReadFile(currentPath)
	if err != nil {
		return nil, fmt.Errorf("read artifact: %w", err)
	}
	fm, body, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return nil, fmt.Errorf("parse artifact: %w", err)
	}
	if fm == nil {
		fm = map[string]any{}
	}
	oldStatus, _ := fm["status"].(string)
	isTopLevel := cfg.topLevel == nil || *cfg.topLevel // default true

	// 066.003-T: Refuse to overwrite a DISTINCT item already occupying the
	// path-keyed archive destination. Computed and checked here -- before the
	// pre-archive hooks fire and before any file is written -- so a refused
	// archive has no side effects. When a foreign item (same root ID/filename
	// but different identity) already sits at the destination, archiving the
	// live copy would silently destroy it (the 066-F data-loss scenario). The
	// legitimate 060.002-T half-archive recovery (the SAME logical item already
	// half-copied into the archive) must still succeed, so the guard only fires
	// when the occupant is a different item (distinct id or title). Same-path
	// in-place archival (currentPath == archivePath) is never a collision.
	archivePath := filepath.Join(archiveDir, filepath.Base(currentPath))
	archiveDestinationOccupied := false
	if filepath.Clean(archivePath) != filepath.Clean(currentPath) {
		if _, statErr := os.Stat(archivePath); statErr == nil {
			occupant, _, occErr := parseFile(archivePath)
			sourceID, _ := fm["id"].(string)
			sourceTitle, _ := fm["title"].(string)
			if occErr != nil {
				// The destination is occupied by a file we cannot parse. Refuse
				// rather than overwrite it (it may be a distinct item with corrupt
				// frontmatter), but report the unparseable-occupant case explicitly
				// instead of mislabeling it a "distinct item" -- the parse error is
				// the actionable detail during incident/debugging.
				return nil, fmt.Errorf("archive %q: destination %q is occupied by an unparseable file (%v): %w",
					itemID, workspaceRelativePath(ws.RootPath, archivePath), occErr, blerrors.ErrArchiveDestinationOccupied)
			}
			sameItem := occupant.ID == sourceID && occupant.Title == sourceTitle
			if !sameItem {
				return nil, fmt.Errorf("archive %q: destination %q is occupied by a distinct item: %w",
					itemID, workspaceRelativePath(ws.RootPath, archivePath), blerrors.ErrArchiveDestinationOccupied)
			}
			archiveDestinationOccupied = true
		} else if !os.IsNotExist(statErr) {
			// A non-not-exist stat error (permission/IO) means we cannot safely
			// determine whether the destination is occupied. Fail loud with context
			// rather than proceeding into a write that would fail in less clear ways,
			// matching CreateArtifact's destination stat handling.
			return nil, fmt.Errorf("archive %q: stat destination %q: %w",
				itemID, workspaceRelativePath(ws.RootPath, archivePath), statErr)
		}
	}

	// Fire pre-archive hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       itemID,
			ArtifactType: fmArtifactType(fm),
			OldValues:    map[string]any{"status": oldStatus},
			NewValues:    map[string]any{"status": string(models.StatusArchived), "archived": true},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     isTopLevel,
		}
		if err := ws.HookRunner.FirePre(ctx, hooks.HookArchiveItem, hookCtx); err != nil {
			return nil, fmt.Errorf("pre-archive hook: %w", err)
		}
	}

	// 067.002-T (U2): For a pre-archived item (already at its archive path because a
	// terminal status routed it to .backlogit/archive/ before ArchiveItem ran),
	// currentPath == archivePath, so workspaceRelativePath would self-reference the
	// archive path and strand the item. Stamp the canonical queue restore path
	// instead. The normal queued->archive branch is unchanged (byte-for-byte).
	if filepath.Clean(currentPath) == filepath.Clean(archivePath) {
		fm["archived_from"] = canonicalRestorePath(ws, filepath.Base(currentPath))
	} else {
		fm["archived_from"] = workspaceRelativePath(ws.RootPath, currentPath)
	}
	// 060.003-T: Preserve the pre-archive status so UnarchiveItem can restore it.
	fm["archived_status"] = oldStatus
	fm["status"] = string(models.StatusArchived)
	newContent := models.SerializeFrontmatter(fm, body)

	samePath := filepath.Clean(currentPath) == filepath.Clean(archivePath)
	movePlan, err := planArtifactMove(ctx, ws.RootPath, currentPath, archivePath)
	if err != nil {
		return nil, fmt.Errorf("plan archive move: %w", err)
	}
	useGitMove := !samePath && !archiveDestinationOccupied && movePlan.kind == artifactMoveGit
	if useGitMove {
		// git mv intentionally stages the delete/add rename pair. The frontmatter
		// rewrite below is left as a normal worktree modification so surrounding
		// backlog commits can stage it with the rest of the mutation.
		if err := performArtifactMove(ctx, movePlan, "archive"); err != nil {
			return nil, err
		}
		if err := replaceFileWithOptions(ws, archivePath, []byte(newContent)); err != nil {
			rollbackGitArtifactMove(reverseArtifactMovePlan(movePlan), archivePath, raw, "archive content write")
			return nil, fmt.Errorf("write archive file: %w", err)
		}
		durableSyncMovedFromDir(ws, currentPath, archivePath, "archive git move")
	} else {
		if err := replaceFileWithOptions(ws, archivePath, []byte(newContent)); err != nil {
			return nil, fmt.Errorf("write archive file: %w", err)
		}
		// Only remove the source file when it differs from the archive destination.
		// When the registry routes a terminal status (e.g. "done") to the archive
		// directory, the item may already reside there before ArchiveItem is called.
		// Removing currentPath in that case would delete the file we just wrote.
		if !samePath {
			if err := os.Remove(currentPath); err != nil {
				_ = os.Remove(archivePath)
				return nil, fmt.Errorf("remove original: %w", err)
			}
			durableSyncMovedFromDir(ws, currentPath, archivePath, "archive")
		}
	}

	// Update DB status to archived. On failure, restore the file to its
	// original path so filesystem and DB index remain consistent.
	if _, dbErr := database.ExecContext(ctx, "UPDATE items SET status = ? WHERE id = ?", string(models.StatusArchived), itemID); dbErr != nil {
		if useGitMove {
			rollbackGitArtifactMove(reverseArtifactMovePlan(movePlan), archivePath, raw, "archive DB rollback")
		} else {
			// Always restore original content to currentPath regardless of whether
			// paths differ: the file may have been overwritten with archive frontmatter
			// even when currentPath == archivePath (already-archived items).
			if restoreErr := os.WriteFile(currentPath, raw, 0o644); restoreErr != nil {
				slog.Error("archive: failed to restore file after DB error",
					"archive_path", archivePath, "original_path", currentPath, "error", restoreErr)
			} else if !samePath {
				// Remove the stale archive copy only when it is a distinct file.
				_ = os.Remove(archivePath)
			}
		}
		return nil, fmt.Errorf("sync archive state: %w", dbErr)
	}

	// Best-effort: log archive event to the item's JSONL log (non-fatal on failure).
	// Errors are logged for diagnosability, matching the pattern in commits.go.
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	ew := NewWorkspaceEventWriter(ws, logsDir)
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: "archived",
		Delta:     map[string]any{"archive_path": workspaceRelativePath(ws.RootPath, archivePath)},
		CommitSHA: cfg.commitSHA,
	}
	if evErr := ew.AppendEvent(ctx, event); evErr != nil {
		slog.Warn("archive item: failed to append event to item log", "item_id", itemID, "error", evErr)
	} else if indexErr := db.IndexEvent(ctx, database, logsDir, event); indexErr != nil {
		slog.Warn("archive item: failed to index event", "item_id", itemID, "error", indexErr)
	}

	// Fire post-archive hooks.
	if ws.HookRunner != nil {
		hookCtx := hooks.HookContext{
			ItemID:       itemID,
			ArtifactType: fmArtifactType(fm),
			OldValues:    map[string]any{"status": oldStatus},
			NewValues:    map[string]any{"status": string(models.StatusArchived), "archived": true},
			Actor:        "backlogit",
			Workspace:    ws.RootPath,
			TopLevel:     isTopLevel,
		}
		ws.HookRunner.FirePost(ctx, hooks.HookArchiveItem, hookCtx)
	}

	// Archive any stash entries linked to this item (best-effort).
	if n, stashErr := ArchiveLinkedStashEntries(ctx, ws, itemID); stashErr != nil {
		slog.Warn("archive item: failed to archive linked stash entries",
			"item_id", itemID, "error", stashErr)
	} else if n > 0 {
		slog.Info("archive item: archived linked stash entries",
			"item_id", itemID, "count", n)
	}

	return &ArchiveRecord{
		ID:            itemID,
		ArchivedAt:    time.Now(),
		OriginalPath:  currentPath,
		ArchivePath:   archivePath,
		CascadedItems: cascadedItems,
		FailedItems:   failedItems,
	}, nil
}

// archiveDescendants collects all descendants of parentID bottom-up and archives
// each one individually. It returns the IDs of successfully archived items and
// any failures. Already-archived children are silently skipped.
func archiveDescendants(ctx context.Context, database *sql.DB, ws *Workspace, parentID string, cfg archiveConfig) ([]string, []ArchiveFailure) {
	descendants := collectDescendants(ctx, database, parentID, make(map[string]bool), 0, maxCascadeDepth(ws))
	// descendants are collected parent-first; reverse for bottom-up archive order.
	for i, j := 0, len(descendants)-1; i < j; i, j = i+1, j-1 {
		descendants[i], descendants[j] = descendants[j], descendants[i]
	}

	var cascaded []string
	var failures []ArchiveFailure
	for _, childID := range descendants {
		_, err := ArchiveItem(ctx, database, ws, childID, WithTopLevel(false), WithCommitSHA(cfg.commitSHA))
		if err != nil {
			// Skip already-archived items silently.
			if isAlreadyArchived(ctx, database, childID) {
				continue
			}
			failures = append(failures, ArchiveFailure{ItemID: childID, Error: err.Error()})
			slog.Warn("cascade archive: child failed", "parent_id", parentID, "child_id", childID, "error", err)
			continue
		}
		cascaded = append(cascaded, childID)
	}
	return cascaded, failures
}

// collectDescendants recursively finds all child item IDs for parentID using
// the DB index. Items are returned parent-first (caller reverses for bottom-up).
func collectDescendants(ctx context.Context, database *sql.DB, parentID string, visited map[string]bool, depth, maxDepth int) []string {
	if depth >= maxDepth || visited[parentID] {
		return nil
	}
	visited[parentID] = true

	children, err := db.QueryItems(ctx, database, db.QueryFilters{ParentID: parentID})
	if err != nil {
		slog.Warn("cascade archive: failed to query children", "parent_id", parentID, "error", err)
		return nil
	}

	var result []string
	for _, child := range children {
		result = append(result, child.ID)
		result = append(result, collectDescendants(ctx, database, child.ID, visited, depth+1, maxDepth)...)
	}
	return result
}

// maxCascadeDepth returns a safe maximum recursion depth for cascade operations.
func maxCascadeDepth(ws *Workspace) int {
	if ws != nil && ws.Config != nil && ws.Config.QueueLayout != nil && len(ws.Config.QueueLayout.Levels) > 0 {
		return len(ws.Config.QueueLayout.Levels) + 1
	}
	return 10 // fallback
}

// isAlreadyArchived checks whether an item has status=archived in the DB.
func isAlreadyArchived(ctx context.Context, database *sql.DB, itemID string) bool {
	item, err := db.GetItem(ctx, database, itemID)
	if err != nil {
		return false
	}
	return item.Status == models.StatusArchived
}

func planArtifactMove(ctx context.Context, workspaceRoot, sourcePath, destPath string) (artifactMovePlan, error) {
	plan := artifactMovePlan{kind: artifactMoveFilesystem}
	if filepath.Clean(sourcePath) == filepath.Clean(destPath) {
		return plan, nil
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return plan, nil
	}
	topLevel, err := runGitCommand(ctx, gitPath, workspaceRoot, "rev-parse", "--show-toplevel")
	if err != nil {
		if isGitNoWorkTreeError(err, workspaceRoot) {
			return plan, nil
		}
		return plan, fmt.Errorf("detect git worktree: %w", err)
	}
	workTreeRoot := filepath.Clean(filepath.FromSlash(strings.TrimSpace(string(topLevel))))
	if workTreeRoot == "" {
		return plan, nil
	}
	sourceRel, ok := gitRelativePath(workTreeRoot, sourcePath)
	if !ok {
		return plan, nil
	}
	destRel, ok := gitRelativePath(workTreeRoot, destPath)
	if !ok {
		return plan, nil
	}
	if _, err := runGitCommand(ctx, gitPath, workTreeRoot, "ls-files", "--error-unmatch", "--", sourceRel); err != nil {
		if isGitUntrackedPathError(err) {
			return plan, nil
		}
		return plan, fmt.Errorf("detect tracked artifact: %w", err)
	}
	plan.kind = artifactMoveGit
	plan.workTreeRoot = workTreeRoot
	plan.sourceRel = sourceRel
	plan.destRel = destRel
	return plan, nil
}

func performArtifactMove(ctx context.Context, plan artifactMovePlan, operation string) error {
	if plan.kind != artifactMoveGit {
		return nil
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("%s git mv: %w", operation, err)
	}
	if _, err := runGitCommand(ctx, gitPath, plan.workTreeRoot, "mv", plan.sourceRel, plan.destRel); err != nil {
		return fmt.Errorf("%s git mv %q to %q: %w", operation, plan.sourceRel, plan.destRel, err)
	}
	return nil
}

func reverseArtifactMovePlan(plan artifactMovePlan) artifactMovePlan {
	return artifactMovePlan{
		kind:         plan.kind,
		workTreeRoot: plan.workTreeRoot,
		sourceRel:    plan.destRel,
		destRel:      plan.sourceRel,
	}
}

func rollbackGitArtifactMove(rollbackPlan artifactMovePlan, currentPath string, originalContent []byte, operation string) {
	rollbackGitArtifactMoveWithReplace(rollbackPlan, currentPath, originalContent, operation, replaceFile)
}

func rollbackGitArtifactMoveWithReplace(rollbackPlan artifactMovePlan, currentPath string, originalContent []byte, operation string, replace func(string, []byte) error) {
	if restoreErr := replace(currentPath, originalContent); restoreErr != nil {
		slog.Error("archive git rollback: failed to restore file content",
			"operation", operation, "path", currentPath, "error", restoreErr)
	}
	rollbackCtx, cancel := context.WithTimeout(context.Background(), artifactGitCommandTimeout)
	defer cancel()
	if moveErr := performArtifactMove(rollbackCtx, rollbackPlan, operation); moveErr != nil {
		slog.Error("archive git rollback: failed to reverse git move",
			"operation", operation, "path", currentPath, "error", moveErr)
	}
}

func restoreArchiveAfterUnarchiveFailure(ws *Workspace, archivePath, originalPath string, raw []byte, samePath bool) error {
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}
	if err := replaceFileWithOptions(ws, archivePath, raw); err != nil {
		return fmt.Errorf("restore archive file: %w", err)
	}
	if !samePath {
		if err := os.Remove(originalPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove restored file: %w", err)
		}
	}
	return nil
}

// replaceFileWriteFn is the atomic-write seam used by replaceFileWithOptions. It
// is a package-global overridden by tests to observe the durable flag routed to
// the low-level primitive.
//
// Must not run with t.Parallel: tests that swap this package-global seam read on
// the production write path.
var replaceFileWriteFn = atomicfile.WriteFileAtomicWithOptions

// replaceFileWithOptions atomically writes content to targetPath, routing the
// workspace's durable_writes preference into the low-level primitive so archive
// and restore content writes are power-loss durable when the flag is enabled.
func replaceFileWithOptions(ws *Workspace, targetPath string, content []byte) error {
	if err := replaceFileWriteFn(targetPath, content, atomicfile.Options{DurableWrites: WorkspaceDurableWrites(ws)}); err != nil {
		return fmt.Errorf("replace file %q: %w", targetPath, err)
	}
	return nil
}

// replaceFile is the durable-off wrapper retained for callers that hold no
// workspace (best-effort git-rollback restores and focused tests).
func replaceFile(targetPath string, content []byte) error {
	return replaceFileWithOptions(nil, targetPath, content)
}

func runGitCommand(ctx context.Context, gitPath, workTreeRoot string, args ...string) ([]byte, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, artifactGitCommandTimeout)
	defer cancel()
	cmdArgs := append([]string{"-C", workTreeRoot}, args...)
	cmd := exec.CommandContext(cmdCtx, gitPath, cmdArgs...)
	cmd.Env = gitCommandEnv()
	output, err := cmd.CombinedOutput()
	if cmdCtx.Err() != nil {
		return output, cmdCtx.Err()
	}
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail != "" {
			return output, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, detail)
		}
		return output, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return output, nil
}

func isGitNoWorkTreeError(err error, workspaceRoot string) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	if gitEntryPresentAtOrAbove(workspaceRoot) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not a git repository") ||
		strings.Contains(msg, "not a gitdir") ||
		strings.Contains(msg, "outside repository")
}

func gitEntryPresentAtOrAbove(start string) bool {
	dir, err := filepath.Abs(start)
	if err != nil {
		return true
	}
	for {
		if _, statErr := os.Lstat(filepath.Join(dir, ".git")); statErr == nil || !os.IsNotExist(statErr) {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

func isGitUntrackedPathError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "did not match any file") ||
		strings.Contains(msg, "pathspec") && strings.Contains(msg, "known to git")
}

func gitCommandEnv() []string {
	// Start from the ambient environment already scrubbed of the formal-gate
	// evidence key (106-F F1/U2), then apply this call site's own git-specific
	// exclusions on top.
	base := config.ChildProcessEnv()
	env := make([]string, 0, len(base)+1)
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if isGitOverrideEnv(key) || isGitLocaleEnv(key) || isGitPromptEnv(key) {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0")
	env = append(env, "LC_ALL=C", "LANG=C")
	return env
}

func isGitLocaleEnv(key string) bool {
	switch key {
	case "LC_ALL", "LANG":
		return true
	default:
		return false
	}
}

func isGitPromptEnv(key string) bool {
	return key == "GIT_TERMINAL_PROMPT"
}

func isGitOverrideEnv(key string) bool {
	upperKey := strings.ToUpper(key)
	if strings.HasPrefix(upperKey, "GIT_CONFIG_") {
		return true
	}
	switch upperKey {
	case "GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_COMMON_DIR",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_NAMESPACE",
		"GIT_CONFIG",
		"GIT_CONFIG_GLOBAL",
		"GIT_CONFIG_SYSTEM",
		"GIT_CEILING_DIRECTORIES",
		"GIT_PREFIX",
		"GIT_SUPER_PREFIX":
		return true
	default:
		return false
	}
}

func gitRelativePath(workTreeRoot, targetPath string) (string, bool) {
	absRoot, ok := canonicalExistingPath(workTreeRoot)
	if !ok {
		return "", false
	}
	absTarget := canonicalTargetPath(targetPath)
	rel, err := filepath.Rel(absRoot, absTarget)
	if err != nil {
		return "", false
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return rel, true
}

func canonicalExistingPath(targetPath string) (string, bool) {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", false
	}
	if evalPath, err := filepath.EvalSymlinks(absPath); err == nil {
		return filepath.Clean(evalPath), true
	}
	if _, statErr := os.Stat(absPath); statErr != nil {
		return "", false
	}
	return filepath.Clean(absPath), true
}

func canonicalTargetPath(targetPath string) string {
	if existing, ok := canonicalExistingPath(targetPath); ok {
		return existing
	}
	parent, ok := canonicalExistingPath(filepath.Dir(targetPath))
	if !ok {
		absPath, err := filepath.Abs(targetPath)
		if err != nil {
			return filepath.Clean(targetPath)
		}
		return filepath.Clean(absPath)
	}
	return filepath.Join(parent, filepath.Base(targetPath))
}

// UnarchiveItem restores an artifact from the archive back to its original path.
func UnarchiveItem(ctx context.Context, database *sql.DB, ws *Workspace, itemID string) error {
	backlogDir := workspaceStorageRoot(ws)
	archivePath, err := FindArtifactPath(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("find archived artifact: %w", err)
	}

	raw, err := os.ReadFile(archivePath)
	if err != nil {
		return fmt.Errorf("read archive file: %w", err)
	}
	fm, body, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return fmt.Errorf("parse archive file: %w", err)
	}

	originalPath, _ := fm["archived_from"].(string)
	if originalPath == "" {
		status, _ := fm["status"].(string)
		if status != string(models.StatusArchived) {
			return fmt.Errorf("item %s is not archived", itemID)
		}
		return fmt.Errorf("archived item %s is missing archived_from metadata", itemID)
	}
	originalPath = resolveWorkspacePath(ws.RootPath, originalPath)

	// 067.003-T (U3): Read-time self-heal. A legacy self-referential record stored
	// its own archive path in archived_from (the pre-067 ArchiveItem bug). Trusting
	// it would strand the item in the archive — the same-path branch below skips the
	// removal when originalPath == archivePath. Recompute the canonical queue restore
	// target so unarchive invertibility does not depend on the doctor
	// --fix-archived-from migration having run first. The same-path branch is retained
	// below as a defensive net for any residual unrecomputed case.
	if filepath.Clean(originalPath) == filepath.Clean(archivePath) {
		originalPath = resolveWorkspacePath(ws.RootPath, canonicalRestorePath(ws, filepath.Base(archivePath)))
	}

	// F-006: Validate the restore path is contained within .backlogit to prevent
	// path traversal when restoring artifacts from archive.
	rel, relErr := filepath.Rel(backlogDir, originalPath)
	if relErr != nil || len(rel) >= 2 && rel[:2] == ".." {
		return fmt.Errorf("archived_from path %q escapes workspace storage: cannot restore", originalPath)
	}

	// Restore frontmatter without the archived_from field.
	// 060.003-T: Also restore the pre-archive status from archived_status.
	delete(fm, "archived_from")
	archivedStatus, _ := fm["archived_status"].(string)
	delete(fm, "archived_status")
	if archivedStatus == "" {
		// Backward compat: items archived before the fix have no archived_status;
		// default to "queued" so they become accessible in normal list flows.
		archivedStatus = string(models.StatusQueued)
	}
	fm["status"] = archivedStatus
	restored := models.SerializeFrontmatter(fm, body)

	if err := mkdirAllDurable(filepath.Dir(originalPath), WorkspaceDurableWrites(ws)); err != nil {
		return fmt.Errorf("create restore dir: %w", err)
	}

	// After read-time self-heal the restore target is the canonical queue path,
	// which should not already exist. Guard against clobbering a live record: a
	// distinct pre-existing destination would be silently overwritten by os.Rename
	// on POSIX (data loss) and would fail cryptically on Windows. The in-place case
	// (originalPath == archivePath) legitimately overwrites and is handled below.
	samePath := filepath.Clean(archivePath) == filepath.Clean(originalPath)
	if !samePath {
		if _, statErr := os.Lstat(originalPath); statErr == nil {
			return fmt.Errorf("restore target %q already exists: refusing to overwrite", originalPath)
		} else if !os.IsNotExist(statErr) {
			return fmt.Errorf("stat restore target %q: %w", originalPath, statErr)
		}
	}
	movePlan, err := planArtifactMove(ctx, ws.RootPath, archivePath, originalPath)
	if err != nil {
		return fmt.Errorf("plan restore move: %w", err)
	}
	useGitMove := !samePath && movePlan.kind == artifactMoveGit
	writeWasIndeterminate := false
	if useGitMove {
		// git mv stages the archive->queue rename; the restored frontmatter update
		// remains unstaged for the caller's normal commit flow.
		if err := performArtifactMove(ctx, movePlan, "restore"); err != nil {
			return err
		}
		if err := replaceFileWithOptions(ws, originalPath, []byte(restored)); err != nil {
			if blerrors.IsWriteIndeterminate(err) {
				// The rename committed — do NOT roll back a possibly-applied write.
				writeWasIndeterminate = true
			} else {
				rollbackGitArtifactMove(reverseArtifactMovePlan(movePlan), originalPath, raw, "restore content write")
				return fmt.Errorf("write restored file: %w", err)
			}
		}
		if !writeWasIndeterminate {
			durableSyncMovedFromDir(ws, archivePath, originalPath, "unarchive git move")
		}
	} else {
		writeErr := replaceFileWithOptions(ws, originalPath, []byte(restored))
		writeWasIndeterminate = blerrors.IsWriteIndeterminate(writeErr)
		if writeErr != nil && !writeWasIndeterminate {
			return fmt.Errorf("write restored file: %w", writeErr)
		}
		// Only remove the archive file when it differs from the restored path.
		// When archived_from stored an archive-dir path (because the file was already
		// there before ArchiveItem ran), originalPath == archivePath and the rename
		// above updated the file in place — removing it here would undo that write.
		if !samePath {
			if err := os.Remove(archivePath); err != nil {
				return fmt.Errorf("remove archive file: %w", err)
			}
			durableSyncMovedFromDir(ws, archivePath, originalPath, "unarchive")
		}
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err == nil {
		if upsertErr := db.UpsertItem(ctx, database, artifact); upsertErr != nil {
			if writeWasIndeterminate {
				// Either branch: the restore write was indeterminate (rename committed,
				// outcome uncertain). Do NOT rollback — would violate the
				// never-roll-back-indeterminate invariant. Surface both errors.
				return fmt.Errorf("sync unarchive state: %w", errors.Join(blerrors.ErrWriteIndeterminate, upsertErr))
			} else if useGitMove {
				rollbackGitArtifactMove(reverseArtifactMovePlan(movePlan), originalPath, raw, "restore DB rollback")
			} else if rollbackErr := restoreArchiveAfterUnarchiveFailure(ws, archivePath, originalPath, raw, samePath); rollbackErr != nil {
				slog.Error("unarchive: failed to restore archive file after DB error",
					"archive_path", archivePath, "restore_path", originalPath, "error", rollbackErr)
			}
			return fmt.Errorf("sync unarchive state: %w", upsertErr)
		}
	}
	// Restore write was indeterminate: the file is likely at originalPath but
	// parent-fsync durability was not confirmed. Surface ErrWriteIndeterminate so
	// the caller can reconcile. The archive copy has been removed and the DB row is
	// present (commit-then-surface invariant).
	if writeWasIndeterminate {
		return fmt.Errorf("restore write outcome uncertain: %w", blerrors.ErrWriteIndeterminate)
	}
	return nil
}

// fmArtifactType extracts artifact_type from frontmatter for hook context.
func fmArtifactType(fm map[string]any) string {
	if t, ok := fm["artifact_type"].(string); ok {
		return t
	}
	if t, ok := fm["type"].(string); ok {
		return t
	}
	return ""
}

// findArtifactInDir searches a single directory (non-recursive) for a .md file
// whose frontmatter ID matches id. Returns ("", nil) when not found.
func findArtifactInDir(dirPath, id string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read dir %s: %w", dirPath, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		path := filepath.Join(dirPath, entry.Name())
		a, _, parseErr := parseFile(path)
		if parseErr != nil {
			continue
		}
		if a.ID == id {
			return path, nil
		}
	}
	return "", nil
}

func queueRootDir(ws *Workspace) string {
	if ws != nil && ws.Config != nil && ws.Config.QueueLayout != nil && ws.Config.QueueLayout.RootDir != "" {
		return ws.Config.QueueLayout.RootDir
	}
	return "queue"
}

// canonicalRestorePath returns the repo-root-relative, storage-root-prefixed POSIX
// restore path for a record basename: "<storageRoot>/<queueRootDir(ws)>/<basename>"
// (default "queue"). It is pure over ws.Config.QueueLayout (mirrors queueRootDir)
// and deliberately does NOT consult the status-keyed registry routing, which would
// re-introduce the archive self-reference for terminal-status items.
//
// The output format matches workspaceRelativePath(ws.RootPath, …) — the
// "<storageRoot>/queue/<id>.md" form asserted by archive_test.go and accepted by the
// UnarchiveItem F-006 traversal guard (archive.go:368-373). A QueueLayout.RootDir
// that is empty, absolute, volume-qualified, cleans to "." or "..", or otherwise
// escapes its parent is rejected: the resolver falls back to the default "queue"
// so the returned path is always workspace-contained.
func canonicalRestorePath(ws *Workspace, basename string) string {
	storageRoot := workspaceRootCandidates[len(workspaceRootCandidates)-1]
	if ws != nil {
		storageRoot = filepath.Base(workspaceStorageRoot(ws))
	}
	root := queueRootDir(ws)
	if isUnsafeRootDir(root) {
		root = "queue"
	}
	return path.Join(storageRoot, filepath.ToSlash(filepath.Clean(root)), filepath.Base(basename))
}

// isUnsafeRootDir reports whether a configured QueueLayout.RootDir would escape
// the workspace storage root if used directly. It rejects absolute
// paths (POSIX or OS-specific), volume-qualified paths, and any ".." parent
// traversal so canonicalRestorePath can fall back to a contained default.
func isUnsafeRootDir(root string) bool {
	if root == "" {
		return true
	}
	if filepath.IsAbs(root) || path.IsAbs(filepath.ToSlash(root)) || filepath.VolumeName(root) != "" {
		return true
	}
	cleaned := filepath.ToSlash(filepath.Clean(root))
	if cleaned == "." || cleaned == ".." ||
		strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return true
	}
	return false
}

func workspaceRelativePath(rootPath string, target string) string {
	rel, err := filepath.Rel(rootPath, target)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(target))
	}
	return filepath.ToSlash(rel)
}

func resolveWorkspacePath(rootPath string, trackedPath string) string {
	if trackedPath == "" {
		return ""
	}
	if filepath.IsAbs(trackedPath) {
		return filepath.Clean(trackedPath)
	}
	return filepath.Join(rootPath, filepath.FromSlash(trackedPath))
}

// AutoArchive scans for items in terminal statuses past the retention window and
// archives them according to the policy.
func AutoArchive(ctx context.Context, database *sql.DB, ws *Workspace, policy *ArchivePolicy) (int, error) {
	if len(policy.TerminalStatuses) == 0 {
		return 0, nil
	}

	threshold := time.Now().AddDate(0, 0, -policy.RetentionDays)
	items, err := db.QueryItems(ctx, database, db.QueryFilters{IncludeArchived: true})
	if err != nil {
		return 0, fmt.Errorf("query items: %w", err)
	}

	terminalSet := make(map[string]bool, len(policy.TerminalStatuses))
	for _, s := range policy.TerminalStatuses {
		terminalSet[s] = true
	}

	count := 0
	for _, item := range items {
		if !terminalSet[string(item.Status)] {
			continue
		}
		if item.UpdatedAt.After(threshold) {
			continue
		}
		if _, archiveErr := ArchiveItem(ctx, database, ws, item.ID); archiveErr != nil {
			continue
		}
		count++
	}
	return count, nil
}
