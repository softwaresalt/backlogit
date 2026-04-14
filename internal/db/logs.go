package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/softwaresalt/backlogit/internal/events"
)

type execContexter interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ItemLogEntry represents an indexed event log entry for a work item.
type ItemLogEntry struct {
	ID        int64
	ItemID    string
	LogPath   string
	Timestamp time.Time
	Actor     string
	EventType string
	Content   string
	Delta     map[string]any
}

// IndexEvent records the relationship between a work item and its log file and
// stores the event in the database for search and efficient lookup.
func IndexEvent(ctx context.Context, database *sql.DB, logsDir string, event events.Event) error {
	if event.ItemID == "" {
		return fmt.Errorf("index event: item_id is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	logPath, err := filepath.Rel(filepath.Dir(logsDir), events.LogPathForItem(logsDir, event.ItemID))
	if err != nil {
		return fmt.Errorf("index event: resolve log path: %w", err)
	}
	if err := UpsertItemLog(ctx, database, event.ItemID, filepath.ToSlash(logPath), event.Timestamp); err != nil {
		return err
	}
	return InsertItemLogEntry(ctx, database, filepath.ToSlash(logPath), event)
}

// indexEventTx is the transaction-scoped variant of IndexEvent used during
// bulk rehydration.  Batching all log index writes into a single transaction
// avoids per-event disk syncs that make rehydration extremely slow.
func indexEventTx(ctx context.Context, tx *sql.Tx, logsDir string, event events.Event) error {
	if event.ItemID == "" {
		return fmt.Errorf("index event: item_id is required")
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	logPath, err := filepath.Rel(filepath.Dir(logsDir), events.LogPathForItem(logsDir, event.ItemID))
	if err != nil {
		return fmt.Errorf("index event: resolve log path: %w", err)
	}
	slashPath := filepath.ToSlash(logPath)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO item_logs (item_id, log_path, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(item_id) DO UPDATE SET
		   log_path = excluded.log_path,
		   updated_at = excluded.updated_at`,
		event.ItemID, slashPath, event.Timestamp.Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("upsert item log %s: %w", event.ItemID, err)
	}
	deltaJSON, jsonErr := json.Marshal(event.Delta)
	if jsonErr != nil {
		return fmt.Errorf("marshal event delta: %w", jsonErr)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_log_entries
			(item_id, log_path, timestamp, actor, event_type, content, delta_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ItemID, slashPath,
		event.Timestamp.Format(time.RFC3339Nano),
		event.Actor, event.EventType, eventContent(event), string(deltaJSON),
	); err != nil {
		return fmt.Errorf("insert item log entry for %s: %w", event.ItemID, err)
	}
	return nil
}

// UpsertItemLog stores the item-to-log-file relationship.
func UpsertItemLog(ctx context.Context, database *sql.DB, itemID, logPath string, updatedAt time.Time) error {
	_, err := database.ExecContext(ctx,
		`INSERT INTO item_logs (item_id, log_path, updated_at)
		 VALUES (?, ?, ?)
		 ON CONFLICT(item_id) DO UPDATE SET
		   log_path = excluded.log_path,
		   updated_at = excluded.updated_at`,
		itemID,
		logPath,
		updatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert item log %s: %w", itemID, err)
	}
	return nil
}

// InsertItemLogEntry stores a single event log entry.
func InsertItemLogEntry(ctx context.Context, database *sql.DB, logPath string, event events.Event) error {
	deltaJSON, err := json.Marshal(event.Delta)
	if err != nil {
		return fmt.Errorf("marshal event delta: %w", err)
	}
	_, err = database.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_log_entries
			(item_id, log_path, timestamp, actor, event_type, content, delta_json)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		event.ItemID,
		logPath,
		event.Timestamp.Format(time.RFC3339Nano),
		event.Actor,
		event.EventType,
		eventContent(event),
		string(deltaJSON),
	)
	if err != nil {
		return fmt.Errorf("insert item log entry for %s: %w", event.ItemID, err)
	}
	return nil
}

// DeleteAllItemLogs clears indexed item log relationships and entries before rehydration.
func DeleteAllItemLogs(ctx context.Context, database *sql.DB) error {
	return deleteAllItemLogs(ctx, database)
}

func deleteAllItemLogs(ctx context.Context, execer execContexter) error {
	if _, err := execer.ExecContext(ctx, `DELETE FROM item_logs`); err != nil {
		return fmt.Errorf("clear item logs: %w", err)
	}
	if _, err := execer.ExecContext(ctx, `DELETE FROM item_log_entries`); err != nil {
		return fmt.Errorf("clear item log entries: %w", err)
	}
	return nil
}

// ListItemLogEntries returns indexed log entries for a single work item.
func ListItemLogEntries(ctx context.Context, database *sql.DB, itemID string, limit int) ([]ItemLogEntry, error) {
	query := `SELECT id, item_id, log_path, timestamp, actor, event_type, content, delta_json
		FROM item_log_entries
		WHERE item_id = ?
		ORDER BY timestamp ASC`
	args := []any{itemID}
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list item log entries: %w", err)
	}
	defer rows.Close()

	return scanItemLogEntries(rows)
}

// SearchItemLogEntries performs FTS5 search across indexed log entry content.
func SearchItemLogEntries(ctx context.Context, database *sql.DB, query string, limit int) ([]ItemLogEntry, error) {
	rows, err := database.QueryContext(ctx,
		`SELECT e.id, e.item_id, e.log_path, e.timestamp, e.actor, e.event_type, e.content, e.delta_json
		   FROM item_log_entries e
		   JOIN item_log_entries_fts fts ON e.id = fts.rowid
		  WHERE item_log_entries_fts MATCH ?
		  ORDER BY e.timestamp DESC
		  LIMIT ?`,
		escapeFTS5Query(query),
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search item log entries: %w", err)
	}
	defer rows.Close()

	return scanItemLogEntries(rows)
}

func scanItemLogEntries(rows *sql.Rows) ([]ItemLogEntry, error) {
	var entries []ItemLogEntry
	for rows.Next() {
		var (
			entry     ItemLogEntry
			timestamp string
			deltaJSON string
		)
		if err := rows.Scan(
			&entry.ID,
			&entry.ItemID,
			&entry.LogPath,
			&timestamp,
			&entry.Actor,
			&entry.EventType,
			&entry.Content,
			&deltaJSON,
		); err != nil {
			return nil, fmt.Errorf("scan item log entry: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			parsed, err = time.Parse(time.RFC3339, timestamp)
		}
		if err != nil {
			return nil, fmt.Errorf("parse item log timestamp %q: %w", timestamp, err)
		}
		entry.Timestamp = parsed
		if deltaJSON != "" && deltaJSON != "null" {
			if err := json.Unmarshal([]byte(deltaJSON), &entry.Delta); err != nil {
				return nil, fmt.Errorf("unmarshal item log delta: %w", err)
			}
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func eventContent(event events.Event) string {
	parts := []string{event.Actor, event.EventType}
	parts = append(parts, flattenDelta(event.Delta)...)
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, " ")
}

func flattenDelta(delta map[string]any) []string {
	if len(delta) == 0 {
		return nil
	}
	keys := make([]string, 0, len(delta))
	for key := range delta {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		parts = append(parts, flattenValue(delta[key])...)
	}
	return parts
}

func flattenValue(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []string:
		return append([]string(nil), v...)
	case []any:
		var parts []string
		for _, item := range v {
			parts = append(parts, flattenValue(item)...)
		}
		return parts
	case map[string]any:
		return flattenDelta(v)
	default:
		return []string{fmt.Sprintf("%v", v)}
	}
}
