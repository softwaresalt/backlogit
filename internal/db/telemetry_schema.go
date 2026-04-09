package db

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// rawTelemetryRecord is the union type for decoding mixed telemetry JSONL.
type rawTelemetryRecord struct {
	RecordType       string         `json:"record_type"`
	HarvestedAt      time.Time      `json:"harvested_at"`
	SessionID        string         `json:"session_id"`
	Branch           string         `json:"branch"`
	Repository       string         `json:"repository"`
	TotalTokens      int            `json:"total_tokens"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	CachedTokens     int            `json:"cached_tokens"`
	ModelCalls       int            `json:"model_calls"`
	ToolCalls        int            `json:"tool_calls"`
	TokensPerTask    *float64       `json:"tokens_per_task"`
	CompactionCount  int            `json:"compaction_count"`
	CompletedTasks   []string       `json:"completed_tasks"`
	TokensByModel    map[string]int `json:"tokens_by_model"`
	TokensByServer   map[string]int `json:"tokens_by_server"`
	// tool_usage fields
	ServerName string `json:"server_name"`
	ToolName   string `json:"tool_name"`
	CallCount  int    `json:"call_count"`
	TotalDurMs int    `json:"total_duration_ms"`
}

// EnsureTelemetrySchema creates the telemetry_sessions and telemetry_tool_usage
// tables idempotently. Called from EnsureSchema during workspace initialization.
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
			harvested_at      TEXT    NOT NULL DEFAULT ''
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
	return nil
}

// RehydrateTelemetry clears and rebuilds the telemetry_sessions and
// telemetry_tool_usage tables from .backlogit/telemetry-sessions.jsonl.
//
// Single write path: JSONL → rehydrate → SQLite (Plan Review F5).
// No direct upserts during harvest — RehydrateTelemetry is the only writer.
// Idempotent: calling twice with the same JSONL produces the same table state.
func RehydrateTelemetry(ctx context.Context, workspacePath string, sqlDB *sql.DB) error {
	jsonlPath := filepath.Join(workspacePath, ".backlogit", "telemetry-sessions.jsonl")
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

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec rawTelemetryRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			slog.Warn("skipping malformed telemetry JSONL line", "err", err)
			continue
		}
		harvestedAt := rec.HarvestedAt.Format(time.RFC3339)

		switch rec.RecordType {
		case "session_summary":
			_, err = tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO telemetry_sessions
					(session_id, branch, repository, total_tokens, prompt_tokens, completion_tokens,
					 cached_tokens, model_calls, tool_calls, tokens_per_task, compaction_count, harvested_at)
				 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
				rec.SessionID, rec.Branch, rec.Repository,
				rec.TotalTokens, rec.PromptTokens, rec.CompletionTokens,
				rec.CachedTokens, rec.ModelCalls, rec.ToolCalls,
				rec.TokensPerTask, rec.CompactionCount, harvestedAt,
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
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan telemetry-sessions.jsonl: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit telemetry rehydration: %w", err)
	}
	tx = nil
	return nil
}
