package db

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// rawTelemetryRecord is the union type for decoding mixed telemetry JSONL.
type rawTelemetryRecord struct {
	RecordType        string         `json:"record_type"`
	HarvestedAt       time.Time      `json:"harvested_at"`
	SessionID         string         `json:"session_id"`
	Branch            string         `json:"branch"`
	Repository        string         `json:"repository"`
	TotalTokens       int            `json:"total_tokens"`
	PromptTokens      int            `json:"prompt_tokens"`
	CompletionTokens  int            `json:"completion_tokens"`
	CachedTokens      int            `json:"cached_tokens"`
	ModelCalls        int            `json:"model_calls"`
	ToolCalls         int            `json:"tool_calls"`
	TokensPerTask     *float64       `json:"tokens_per_task"`
	CompactionCount   int            `json:"compaction_count"`
	CompletedTasks    []string       `json:"completed_tasks"`
	TokensByModel     map[string]int `json:"tokens_by_model"`
	TokensByServer    map[string]int `json:"tokens_by_server"`
	ToolCallsByServer map[string]int `json:"tool_calls_by_server"`
	// Context window fields (031-F / 031.003-T)
	PeakUtilization   *float64 `json:"peak_utilization,omitempty"`
	RemainingCapacity *int     `json:"remaining_capacity,omitempty"`
	DepletionRate     *float64 `json:"depletion_rate,omitempty"`
	MaxContextTokens  *int     `json:"max_context_tokens,omitempty"`
	// tool_usage fields
	ServerName string `json:"server_name"`
	ToolName   string `json:"tool_name"`
	CallCount  int    `json:"call_count"`
	TotalDurMs int    `json:"total_duration_ms"`
}

// EnsureTelemetrySchema creates the telemetry_sessions and telemetry_tool_usage
// tables idempotently. Called lazily by the telemetry harvest handler on first use
// rather than during workspace initialization; tables are created on demand.
//
// telemetry_tool_usage uses a composite primary key (session_id, server_name,
// tool_name) — no AUTOINCREMENT (Plan Review F7). This enforces uniqueness per
// harvest run and prevents double-counting on re-harvest.
func EnsureTelemetrySchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS telemetry_sessions (
			session_id        TEXT    PRIMARY KEY,
			branch            TEXT    NOT NULL DEFAULT '',
			repository        TEXT    NOT NULL DEFAULT '',
			total_tokens      INTEGER NOT NULL DEFAULT 0,
			prompt_tokens     INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			cached_tokens     INTEGER NOT NULL DEFAULT 0,
			model_calls       INTEGER NOT NULL DEFAULT 0,
			tool_calls        INTEGER NOT NULL DEFAULT 0,
			tokens_per_task   REAL,
			compaction_count  INTEGER NOT NULL DEFAULT 0,
			harvested_at      TEXT    NOT NULL DEFAULT '',
			peak_utilization  REAL,
			remaining_capacity INTEGER,
			depletion_rate    REAL,
			max_context_tokens INTEGER,
			tokens_by_server  TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS telemetry_tool_usage (
			session_id      TEXT    NOT NULL,
			server_name     TEXT    NOT NULL,
			tool_name       TEXT    NOT NULL,
			call_count      INTEGER NOT NULL DEFAULT 0,
			total_dur_ms    INTEGER NOT NULL DEFAULT 0,
			harvested_at    TEXT    NOT NULL DEFAULT '',
			PRIMARY KEY (session_id, server_name, tool_name)
		)`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("ensure telemetry schema: %w", err)
		}
	}

	// Migrate existing workspaces: add context-window columns when the table
	// was created by an older version. SQLite supports ADD COLUMN but not
	// IF NOT EXISTS; we catch the "duplicate column name" error gracefully.
	alterStmts := []string{
		`ALTER TABLE telemetry_sessions ADD COLUMN peak_utilization REAL`,
		`ALTER TABLE telemetry_sessions ADD COLUMN remaining_capacity INTEGER`,
		`ALTER TABLE telemetry_sessions ADD COLUMN depletion_rate REAL`,
		`ALTER TABLE telemetry_sessions ADD COLUMN max_context_tokens INTEGER`,
		`ALTER TABLE telemetry_sessions ADD COLUMN tokens_by_server TEXT`,
	}
	for _, stmt := range alterStmts {
		if _, err := db.Exec(stmt); err != nil {
			if !strings.Contains(err.Error(), "duplicate column name") {
				return fmt.Errorf("migrate telemetry schema: %w", err)
			}
			// Column already exists in an existing workspace — skip silently.
		}
	}
	return nil
}

// RehydrateTelemetry clears and rebuilds the telemetry_sessions and
// telemetry_tool_usage tables from .backlogit/telemetry-sessions.jsonl.
//
// Single write path: JSONL → rehydrate → SQLite (Plan Review F5).
// No direct upserts during harvest — RehydrateTelemetry is the only writer.
// Idempotent: calling twice with the same JSONL produces the same table state.
func RehydrateTelemetry(ctx context.Context, workspacePath string, sqlDB *sql.DB) error {
	jsonlPath := filepath.Join(workspaceStorageRoot(workspacePath), "telemetry-sessions.jsonl")
	f, err := os.Open(jsonlPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Debug("telemetry-sessions.jsonl not found, skipping rehydration", "path", jsonlPath)
			return nil
		}
		return fmt.Errorf("open telemetry-sessions.jsonl: %w", err)
	}
	defer f.Close()

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin telemetry rehydration tx: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Clear existing data for a clean idempotent rebuild.
	if _, err := tx.ExecContext(ctx, `DELETE FROM telemetry_sessions`); err != nil {
		return fmt.Errorf("clear telemetry_sessions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM telemetry_tool_usage`); err != nil {
		return fmt.Errorf("clear telemetry_tool_usage: %w", err)
	}

	reader := bufio.NewReader(f)
	for {
		rawLine, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return fmt.Errorf("scan telemetry-sessions.jsonl: %w", readErr)
		}
		isEOF := readErr == io.EOF
		line := strings.TrimRight(rawLine, "\r\n")
		if line == "" {
			if isEOF {
				break
			}
			continue
		}
		var rec rawTelemetryRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			slog.Warn("skipping malformed telemetry JSONL line", "err", err)
			if isEOF {
				break
			}
			continue
		}
		harvestedAt := rec.HarvestedAt.Format(time.RFC3339)

		switch rec.RecordType {
		case "session_summary":
			var tokensByServerJSON interface{}
			if len(rec.TokensByServer) > 0 {
				b, merr := json.Marshal(rec.TokensByServer)
				if merr != nil {
					return fmt.Errorf("marshal tokens_by_server for session %q: %w", rec.SessionID, merr)
				}
				tokensByServerJSON = string(b)
			}
			_, err = tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO telemetry_sessions
					(session_id, branch, repository, total_tokens, prompt_tokens, completion_tokens,
					 cached_tokens, model_calls, tool_calls, tokens_per_task, compaction_count, harvested_at,
					 peak_utilization, remaining_capacity, depletion_rate, max_context_tokens, tokens_by_server)
				 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				rec.SessionID, rec.Branch, rec.Repository,
				rec.TotalTokens, rec.PromptTokens, rec.CompletionTokens,
				rec.CachedTokens, rec.ModelCalls, rec.ToolCalls,
				rec.TokensPerTask, rec.CompactionCount, harvestedAt,
				rec.PeakUtilization, rec.RemainingCapacity, rec.DepletionRate, rec.MaxContextTokens,
				tokensByServerJSON,
			)
			if err != nil {
				return fmt.Errorf("insert session_summary %q: %w", rec.SessionID, err)
			}

		case "tool_usage":
			_, err = tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO telemetry_tool_usage
					(session_id, server_name, tool_name, call_count, total_dur_ms, harvested_at)
				 VALUES (?,?,?,?,?,?)`,
				rec.SessionID, rec.ServerName, rec.ToolName,
				rec.CallCount, rec.TotalDurMs, harvestedAt,
			)
			if err != nil {
				return fmt.Errorf("insert tool_usage %q/%q: %w", rec.SessionID, rec.ToolName, err)
			}

		default:
			slog.Debug("ignoring unknown telemetry record type", "record_type", rec.RecordType)
		}
		if isEOF {
			break
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit telemetry rehydration: %w", err)
	}
	tx = nil
	return nil
}
