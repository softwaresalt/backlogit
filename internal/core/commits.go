package core

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
	dbpkg "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
)

// CommitLinkInfo contains the metadata linking an artifact to a git commit.
type CommitLinkInfo struct {
	ItemID    string `json:"item_id"`
	CommitSHA string `json:"commit_sha"`
	Message   string `json:"message"`
	Author    string `json:"author"`
}

// AssociateCommit performs commit association for an artifact through an ordered
// list of discrete steps so that F5's compensating envelope can wrap each step
// independently without rewriting this function:
//
//  1. Frontmatter scalar update (idempotent, reversible — reloads from markdown,
//     never the DB fast path; see compound rule 2026-07-28).
//  2. commit_links upsert (idempotent, reversible via DELETE).
//  3. JSONL append (append-only, sequenced LAST, explicitly never compensated).
//
// The JSONL append is NOT idempotent: events.EventWriter.AppendEvent appends with
// no dedup key and documents ErrWriteIndeterminate on partial/fsync failure as
// unsafe to blindly retry. It is sequenced last so no subsequent step requires
// compensating it. Its Compensate in F5 will be a documented no-op (an audit
// trail is never rewritten). ErrWriteNotApplied before any bytes are written
// leaves the call safely retryable. ErrWriteIndeterminate is surfaced as an error
// without retrying and without compensating the prior two steps, matching F5's
// existing indeterminate-outcome rule — no new dedup/locking mechanism is
// introduced here.
//
// The ew parameter must not be nil. Callers are responsible for lifecycle
// management: the MCP server passes its shared s.Events instance so concurrent
// calls serialize through that writer's mutex; the CLI constructs a
// per-invocation writer via NewWorkspaceEventWriter, mirroring the established
// comment/checkpoint disposition plan (see compound rule 2026-07-04). The core
// function never mints an EventWriter itself.
//
// message and author are preserved when provided (MCP track_commit). When not
// available (CLI fallback), empty strings are stored and documented in the
// governed-operation parity contract (U6).
func AssociateCommit(ctx context.Context, ws *Workspace, ew *events.EventWriter, itemID, sha, message, author string) error {
	if ew == nil {
		return fmt.Errorf("associate commit: EventWriter must not be nil")
	}

	// Step 1: commit_links upsert — idempotent (INSERT OR REPLACE), reversible via DELETE.
	// Sequenced first so a DB failure is fail-fast with no file mutation, avoiding
	// partial state where the frontmatter scalar is updated but commit_links is absent.
	if _, err := ws.DB.ExecContext(ctx,
		"INSERT OR REPLACE INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)",
		itemID, sha, message, author,
	); err != nil {
		return fmt.Errorf("associate commit step 1 (commit_links upsert): %w", err)
	}

	// Step 2: frontmatter scalar update — reloads from markdown via UpdateArtifact
	// (which delegates to findArtifact, never the DB fast path).
	if _, err := UpdateArtifact(ctx, ws, itemID, map[string]any{"commit": sha}); err != nil {
		return fmt.Errorf("associate commit step 2 (frontmatter scalar): %w", err)
	}

	// Step 3: JSONL append — append-only, LAST step, never compensated.
	// Timestamp is intentionally zero: AppendEvent stamps time.Now() on its own copy
	// so the JSONL timestamp is authoritative (matching the AppendComment convention).
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	event := events.Event{
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: "commit_tracked",
		Delta: map[string]any{
			"commit_sha": sha,
			"message":    message,
			"author":     author,
		},
	}
	if err := ew.AppendEvent(ctx, event); err != nil {
		// Surface the error without retrying; do not compensate prior steps.
		// ErrWriteIndeterminate: caller treats as a non-retryable partial write.
		// ErrWriteNotApplied: caller may safely retry the whole call.
		return fmt.Errorf("associate commit step 3 (JSONL append): %w", err)
	}
	if indexErr := dbpkg.IndexEvent(ctx, ws.DB, logsDir, event); indexErr != nil {
		// Index failure is non-fatal: the JSONL file is the source of truth and
		// can be rebuilt on next sync. Log and continue.
		slog.Warn("associate commit: failed to index commit_tracked event",
			"item_id", itemID, "sha", sha, "error", indexErr)
	}

	return nil
}

// LinkCommit associates a git commit SHA and author with an artifact in the SQLite index
// and appends a commit_tracked event to the item's JSONL log for rehydration durability.
//
// Deprecated: Use AssociateCommit instead. LinkCommit is a best-effort implementation
// that silently swallows JSONL append failures and does not update the frontmatter scalar.
// It is retained for backward compatibility but must not be used for new code.
func LinkCommit(ctx context.Context, db *sql.DB, ws *Workspace, itemID, commitSHA, message, author string) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)`,
		itemID, commitSHA, message, author,
	)
	if err != nil {
		return fmt.Errorf("link commit: %w", err)
	}

	// Append to the item's JSONL log so rehydration and search can rebuild state from log files.
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	ew := NewWorkspaceEventWriter(ws, logsDir)
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: "commit_tracked",
		Delta: map[string]any{
			"commit_sha": commitSHA,
			"message":    message,
			"author":     author,
		},
	}
	if evErr := ew.AppendEvent(ctx, event); evErr != nil {
		slog.Warn("link commit: failed to append to item log", "item_id", itemID, "sha", commitSHA, "error", evErr)
	} else if indexErr := dbpkg.IndexEvent(ctx, db, logsDir, event); indexErr != nil {
		slog.Warn("link commit: failed to index item log", "item_id", itemID, "sha", commitSHA, "error", indexErr)
	}

	return nil
}

// AppendComment appends a "comment" event to an item's JSONL log and indexes it
// into the SQLite item-log table. It is the shared path used by both the CLI
// `comment add` command and the MCP append_comment handler so the persisted and
// indexed comment event is byte-identical across surfaces.
//
// The ew parameter lets a long-lived caller (the MCP server) pass its shared
// *events.EventWriter so concurrent append_comment invocations serialize through
// that writer's mutex exactly as they did before this path was extracted. A nil
// ew is valid for one-shot callers (the CLI process) and causes a fresh writer to
// be created. This mirrors the established handler pattern where the shared
// s.Events writer performs the append while the index path is derived from the
// workspace root.
//
// Timestamp handling is deliberate and behavior-preserving: the event is built
// with a zero Timestamp. EventWriter.AppendEvent receives the Event by value and
// stamps time.Now() on its own copy for the JSONL line, while the same
// zero-Timestamp event value is handed to db.IndexEvent — exactly matching the
// pre-extraction inline MCP handler. Do NOT set event.Timestamp here; doing so
// would change the indexed row's timestamp and break parity with the prior
// behavior. Errors are wrapped with %w so callers can use errors.Is/As.
func AppendComment(ctx context.Context, ws *Workspace, ew *events.EventWriter, itemID, actor, comment, commitSHA string) error {
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	if ew == nil {
		ew = NewWorkspaceEventWriter(ws, logsDir)
	}
	event := events.Event{
		Actor:     actor,
		ItemID:    itemID,
		EventType: "comment",
		Delta:     map[string]any{"comment": comment},
		CommitSHA: commitSHA,
	}
	if err := ew.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("append comment: %w", err)
	}
	if err := dbpkg.IndexEvent(ctx, ws.DB, logsDir, event); err != nil {
		return fmt.Errorf("index comment log: %w", err)
	}
	return nil
}

// GetCommitLinks retrieves all commit associations for a given artifact.
func GetCommitLinks(ctx context.Context, db *sql.DB, itemID string) ([]CommitLinkInfo, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT item_id, commit_sha, message, author FROM commit_links WHERE item_id = ?`,
		itemID,
	)
	if err != nil {
		return nil, fmt.Errorf("get commit links: %w", err)
	}
	defer rows.Close()

	var links []CommitLinkInfo
	for rows.Next() {
		var l CommitLinkInfo
		if err := rows.Scan(&l.ItemID, &l.CommitSHA, &l.Message, &l.Author); err != nil {
			return nil, fmt.Errorf("scan commit link: %w", err)
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// artifactIDPattern matches typed hierarchical artifact IDs like F001 or F001.T001.ST001.
var artifactIDPattern = regexp.MustCompile(`\b([A-Z]{1,6}\d{3,}(?:\.[A-Z]{1,6}\d{3,})*)\b`)

// AutoLinkCommits scans recent git log messages for artifact ID references and
// creates links automatically. depth=0 returns immediately with no links.
func AutoLinkCommits(ctx context.Context, db *sql.DB, ws *Workspace, depth int) ([]CommitLinkInfo, error) {
	if depth <= 0 {
		return nil, nil
	}

	cmd := exec.CommandContext(ctx, "git", "-C", ws.RootPath,
		"log", "--pretty=format:%H\t%ae\t%s", fmt.Sprintf("-n%d", depth))
	// Explicitly scrub the formal-gate-evidence key from this child process's
	// environment (106-F F1/U2) rather than leaving Env nil, which would
	// otherwise inherit the full ambient environment unfiltered.
	cmd.Env = config.ChildProcessEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var linked []CommitLinkInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 {
			continue
		}
		sha, author, msg := parts[0], parts[1], parts[2]
		for _, match := range artifactIDPattern.FindAllString(msg, -1) {
			if err := LinkCommit(ctx, db, ws, match, sha, msg, author); err != nil {
				continue
			}
			linked = append(linked, CommitLinkInfo{
				ItemID: match, CommitSHA: sha, Message: msg, Author: author,
			})
		}
	}
	return linked, nil
}
