package telemetry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	idb "github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/errors"
)

// HarvestResult summarises the outcome of a telemetry harvest run.
type HarvestResult struct {
	SessionsHarvested int
	ToolCallsIndexed  int
	TotalTokens       int
}

// HarvestTelemetry is the top-level harvest orchestrator. It:
//  1. Parses process logs from copilotPath (.copilot/logs/)
//  2. Loads session metadata from copilotPath (.copilot/session-state/, session-store.db)
//  3. Correlates events with backlogit task completions via workspacePath/.backlogit/events.jsonl
//  4. Attributes tool calls to MCP servers via the attribution registry
//  5. Writes typed records to workspacePath/.backlogit/telemetry-sessions.jsonl
//  6. Triggers RehydrateTelemetry to rebuild SQLite telemetry tables
//
// Full re-harvest in v1; no incremental checkpointing (Plan Review F2).
// Returns ErrTelemetrySourceMissing when copilotPath does not exist.
func HarvestTelemetry(ctx context.Context, workspacePath, copilotPath string, sqlDB *sql.DB) (HarvestResult, error) {
	if _, err := os.Stat(copilotPath); os.IsNotExist(err) {
		return HarvestResult{}, fmt.Errorf("copilot directory not found: %w", errors.ErrTelemetrySourceMissing)
	}

	// Parse all *.log files from copilotPath/logs/.
	events, err := parseLogFiles(filepath.Join(copilotPath, "logs"))
	if err != nil {
		return HarvestResult{}, fmt.Errorf("parse log files: %w", err)
	}

	// Load session metadata from session-state dir and session-store.db.
	metas, err := loadSessionMetas(copilotPath)
	if err != nil {
		slog.Warn("session metadata load partial failure", "err", err)
	}

	// Correlate events into per-session summaries.
	summaries, err := Correlate(ctx, events, metas, workspacePath)
	if err != nil {
		return HarvestResult{}, fmt.Errorf("correlate telemetry: %w", err)
	}

	// Compute per-(session, server, tool) call counts and durations.
	toolStats := make(map[toolKey]toolStat)
	for _, e := range events {
		if e.Kind != EventKindToolCall || e.ToolCall == nil {
			continue
		}
		tc := e.ToolCall
		server := AttributeTool(tc.ToolName)
		key := toolKey{tc.SessionID, server, tc.ToolName}
		s := toolStats[key]
		s.count++
		s.durMs += tc.DurationMs
		toolStats[key] = s
	}

	// Write typed records to telemetry-sessions.jsonl.
	harvestedAt := time.Now().UTC()
	jsonlPath := filepath.Join(workspacePath, ".backlogit", "telemetry-sessions.jsonl")
	if err := writeTelemetryJSONL(jsonlPath, summaries, toolStats, events, harvestedAt); err != nil {
		return HarvestResult{}, fmt.Errorf("write telemetry-sessions.jsonl: %w", err)
	}

	// Ensure telemetry schema and rehydrate SQLite tables.
	if err := idb.EnsureTelemetrySchema(sqlDB); err != nil {
		return HarvestResult{}, fmt.Errorf("ensure telemetry schema: %w", err)
	}
	if err := idb.RehydrateTelemetry(ctx, workspacePath, sqlDB); err != nil {
		return HarvestResult{}, fmt.Errorf("rehydrate telemetry: %w", err)
	}

	totalTokens := 0
	for _, s := range summaries {
		totalTokens += s.TotalTokens
	}

	return HarvestResult{
		SessionsHarvested: len(summaries),
		ToolCallsIndexed:  len(toolStats),
		TotalTokens:       totalTokens,
	}, nil
}

// parseLogFiles globs *.log files from logsDir and parses each with CopilotCLIParser.
func parseLogFiles(logsDir string) ([]TelemetryEvent, error) {
	pattern := filepath.Join(logsDir, "*.log")
	logFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob log files: %w", err)
	}
	parser := &CopilotCLIParser{}
	var events []TelemetryEvent
	for _, logPath := range logFiles {
		f, err := os.Open(logPath)
		if err != nil {
			slog.Warn("failed to open log file", "path", logPath, "err", err)
			continue
		}
		parseErr := parser.Parse(f, func(e TelemetryEvent) error {
			events = append(events, e)
			return nil
		})
		f.Close()
		if parseErr != nil {
			slog.Warn("log parse error", "path", logPath, "err", parseErr)
		}
	}
	return events, nil
}

// loadSessionMetas merges compaction events and session-store.db into a unified metas map.
func loadSessionMetas(copilotPath string) (map[string]SessionMeta, error) {
	metas := make(map[string]SessionMeta)

	sessionStateDir := filepath.Join(copilotPath, "session-state")
	compactionMap, err := LoadSessionEvents(sessionStateDir)
	if err != nil {
		return metas, fmt.Errorf("load session events: %w", err)
	}
	for sid, ces := range compactionMap {
		meta := metas[sid]
		meta.SessionID = sid
		meta.CompactionEvents = ces
		metas[sid] = meta
	}

	storeMetas, err := ReadSessionStore(filepath.Join(copilotPath, "session-store.db"))
	if err != nil {
		return metas, fmt.Errorf("read session store: %w", err)
	}
	for sid, sm := range storeMetas {
		existing := metas[sid]
		existing.SessionID = sid
		if sm.Branch != "" {
			existing.Branch = sm.Branch
		}
		if sm.Repository != "" {
			existing.Repository = sm.Repository
		}
		if sm.WorkDir != "" {
			existing.WorkDir = sm.WorkDir
		}
		if sm.StartedAt != "" {
			existing.StartedAt = sm.StartedAt
		}
		metas[sid] = existing
	}
	return metas, nil
}

type toolKey struct {
	sessionID, serverName, toolName string
}
type toolStat struct{ count, durMs int }

// writeTelemetryJSONL writes session summary and tool usage records to the JSONL file.
func writeTelemetryJSONL(
	jsonlPath string,
	summaries []SessionSummary,
	toolStats map[toolKey]toolStat,
	events []TelemetryEvent,
	harvestedAt time.Time,
) (retErr error) {
	f, err := os.Create(jsonlPath)
	if err != nil {
		return fmt.Errorf("create %s: %w", jsonlPath, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && retErr == nil {
			retErr = fmt.Errorf("close %s: %w", jsonlPath, closeErr)
		}
	}()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	for _, s := range summaries {
		// Compute server → call count for the session summary record.
		serverCalls := make(map[string]int)
		for _, e := range events {
			if e.Kind == EventKindToolCall && e.ToolCall != nil &&
				e.ToolCall.SessionID == s.SessionID {
				server := AttributeTool(e.ToolCall.ToolName)
				serverCalls[server]++
			}
		}
		rec := SessionSummaryRecord{
			RecordType:       "session_summary",
			HarvestedAt:      harvestedAt,
			SessionID:        s.SessionID,
			Branch:           s.Branch,
			Repository:       s.Repository,
			TotalTokens:      s.TotalTokens,
			PromptTokens:     s.PromptTokens,
			CompletionTokens: s.CompletionTokens,
			CachedTokens:     s.CachedTokens,
			ModelCalls:       s.ModelCalls,
			ToolCalls:        s.ToolCalls,
			TokensByModel:    s.TokensByModel,
			TokensByServer:   serverCalls,
			CompletedTasks:   s.CompletedTasks,
			TokensPerTask:    s.TokensPerTask,
			CompactionCount:  len(s.CompactionEvents),
		}
		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("encode session summary %q: %w", s.SessionID, err)
		}
	}

	for key, stat := range toolStats {
		rec := ToolUsageRecord{
			RecordType:  "tool_usage",
			HarvestedAt: harvestedAt,
			SessionID:   key.sessionID,
			ServerName:  key.serverName,
			ToolName:    key.toolName,
			CallCount:   stat.count,
			TotalDurMs:  stat.durMs,
		}
		if err := enc.Encode(rec); err != nil {
			return fmt.Errorf("encode tool usage %q/%q: %w", key.sessionID, key.toolName, err)
		}
	}
	return nil
}
