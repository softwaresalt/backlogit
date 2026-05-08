package telemetry

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
	"runtime"
	"strings"
	"time"

	idb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/errors"
)

// HarvestResult summarises the outcome of a telemetry harvest run.
type HarvestResult struct {
	SessionsHarvested int
	ToolCallsIndexed  int
	TotalTokens       int
}

// HarvestTelemetry is the top-level harvest orchestrator. It:
//  1. Loads (or creates) the harvest checkpoint from workspacePath/.backlogit/
//  2. Parses process logs from copilotPath (.copilot/logs/) starting at saved offsets
//  3. Loads session metadata from copilotPath (.copilot/session-state/, session-store.db)
//  4. Correlates events with backlogit task completions via per-item logs in workspacePath/.backlogit/logs/
//  5. Computes context window utilisation per session
//  6. Attributes tool calls to MCP servers via the attribution registry
//  7. Merges new sessions with prior JSONL records (incremental) or replaces (Force)
//  8. Writes typed records to workspacePath/.backlogit/telemetry-sessions.jsonl
//  9. Triggers RehydrateTelemetry to rebuild SQLite telemetry tables
//
// 10. Saves the updated checkpoint for the next run
//
// When opts.Force is false and a checkpoint exists, only new log data (by byte
// offset) is parsed and merged with existing JSONL. When opts.Force is true,
// all logs are re-processed from offset 0 and the JSONL is overwritten.
// When opts.Since is set, events whose timestamp precedes that value are
// excluded; events with unparseable timestamps are always included.
//
// Returns ErrTelemetrySourceMissing when copilotPath does not exist.
func HarvestTelemetry(ctx context.Context, workspacePath, copilotPath string, sqlDB *sql.DB, opts HarvestOptions) (HarvestResult, error) {
	if _, err := os.Stat(copilotPath); os.IsNotExist(err) {
		return HarvestResult{}, fmt.Errorf("copilot directory not found: %w", errors.ErrTelemetrySourceMissing)
	}

	// Load checkpoint (zero on missing/corrupt, or when Force is set).
	cp := &HarvestCheckpoint{FileOffsets: make(map[string]int64)}
	if !opts.Force {
		var loadErr error
		cp, loadErr = LoadCheckpoint(workspacePath)
		if loadErr != nil {
			slog.Warn("failed to load harvest checkpoint; treating as fresh", "err", loadErr)
			cp = &HarvestCheckpoint{FileOffsets: make(map[string]int64)}
		}
	}

	// Parse new bytes from log files starting at checkpoint offsets.
	events, newOffsets, err := parseLogFiles(filepath.Join(copilotPath, "logs"), cp, opts)
	if err != nil {
		return HarvestResult{}, fmt.Errorf("parse log files: %w", err)
	}

	// Load session metadata from session-state dir and session-store.db.
	metas, err := loadSessionMetas(copilotPath)
	if err != nil {
		slog.Warn("session metadata load partial failure", "err", err)
	}

	// Correlate events into per-session summaries for new events only.
	newSummaries, err := Correlate(ctx, events, metas, workspacePath, opts.AttributionPrefixes)
	if err != nil {
		return HarvestResult{}, fmt.Errorf("correlate telemetry: %w", err)
	}

	// Filter out partial sessions before any further processing. A session is
	// partial when oversized log entries were dropped by the parser, leaving
	// tool calls with no corresponding token attribution.
	validSummaries := newSummaries[:0]
	for _, s := range newSummaries {
		if ValidateSessionSummary(s) {
			validSummaries = append(validSummaries, s)
		}
	}
	newSummaries = validSummaries

	// Align the event stream with the filtered session set so that tool_usage
	// records written to JSONL (and subsequently SQLite) do not contain orphan
	// entries for sessions intentionally rejected above.
	validIDs := make(map[string]struct{}, len(newSummaries))
	for _, s := range newSummaries {
		validIDs[s.SessionID] = struct{}{}
	}
	filteredEvents := events[:0]
	for _, e := range events {
		var sid string
		switch e.Kind {
		case EventKindModelCall:
			if e.ModelCall != nil {
				sid = e.ModelCall.SessionID
			}
		case EventKindToolCall:
			if e.ToolCall != nil {
				sid = e.ToolCall.SessionID
			}
		}
		if _, ok := validIDs[sid]; ok {
			filteredEvents = append(filteredEvents, e)
		}
	}
	events = filteredEvents

	// Compute context window metrics for each new session summary.
	modelCallsBySession := groupModelCallsBySession(events)
	for i, s := range newSummaries {
		calls := modelCallsBySession[s.SessionID]
		var compactions []CompactionEvent
		if meta, ok := metas[s.SessionID]; ok {
			compactions = meta.CompactionEvents
		}
		newSummaries[i].ContextWindow = ComputeContextMetrics(calls, compactions)
	}

	// Compute per-(session, server, tool) call counts and durations.
	attr := BuildAttributor(opts.AttributionPrefixes)
	toolStats := make(map[toolKey]toolStat)
	for _, e := range events {
		if e.Kind != EventKindToolCall || e.ToolCall == nil {
			continue
		}
		tc := e.ToolCall
		server := attr(tc.ToolName)
		key := toolKey{tc.SessionID, server, tc.ToolName}
		s := toolStats[key]
		s.count++
		s.durMs += tc.DurationMs
		toolStats[key] = s
	}

	// Derive per-session server call counts from toolStats.
	serverCallsPerSession := make(map[string]map[string]int)
	for key, stat := range toolStats {
		if serverCallsPerSession[key.sessionID] == nil {
			serverCallsPerSession[key.sessionID] = make(map[string]int)
		}
		serverCallsPerSession[key.sessionID][key.serverName] += stat.count
	}

	// Read prior JSONL for incremental merge, excluding sessions we just re-processed.
	jsonlPath := filepath.Join(workspacePath, ".backlogit", "telemetry-sessions.jsonl")
	var priorSessions []SessionSummaryRecord
	var priorTools []ToolUsageRecord
	if !opts.Force {
		excludeIDs := sessionIDSet(newSummaries)
		priorSessions, priorTools, _ = readSessionJSONL(jsonlPath, excludeIDs)
	}

	// Write JSONL: prior records preserved, new records appended.
	harvestedAt := time.Now().UTC()
	if err := writeTelemetryJSONL(jsonlPath, newSummaries, toolStats, serverCallsPerSession, harvestedAt, priorSessions, priorTools); err != nil {
		return HarvestResult{}, fmt.Errorf("write telemetry-sessions.jsonl: %w", err)
	}

	// Ensure telemetry schema and rehydrate SQLite tables.
	if err := idb.EnsureTelemetrySchema(sqlDB); err != nil {
		return HarvestResult{}, fmt.Errorf("ensure telemetry schema: %w", err)
	}
	if err := idb.RehydrateTelemetry(ctx, workspacePath, sqlDB); err != nil {
		return HarvestResult{}, fmt.Errorf("rehydrate telemetry: %w", err)
	}

	// Save checkpoint (even after Force — so the next run starts from current EOF).
	cp.FileOffsets = newOffsets
	cp.LastHarvest = harvestedAt
	if saveErr := SaveCheckpoint(workspacePath, cp); saveErr != nil {
		slog.Warn("failed to save harvest checkpoint", "err", saveErr)
	}

	// Total tokens = new sessions + preserved prior sessions (for idempotency).
	totalTokens := 0
	for _, s := range newSummaries {
		totalTokens += s.TotalTokens
	}
	for _, r := range priorSessions {
		totalTokens += r.TotalTokens
	}

	return HarvestResult{
		SessionsHarvested: len(newSummaries) + len(priorSessions),
		ToolCallsIndexed:  len(toolStats) + len(priorTools),
		TotalTokens:       totalTokens,
	}, nil
}

// parseLogFiles globs *.log files from logsDir and parses each with CopilotCLIParser.
// When opts.Force is false, each file is read starting from cp.FileOffsets[basename].
// Returns the parsed events and a map of new byte offsets (basename → EOF position).
func parseLogFiles(logsDir string, cp *HarvestCheckpoint, opts HarvestOptions) ([]TelemetryEvent, map[string]int64, error) {
	pattern := filepath.Join(logsDir, "*.log")
	logFiles, err := filepath.Glob(pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("glob log files: %w", err)
	}
	parser := &CopilotCLIParser{}
	var events []TelemetryEvent
	newOffsets := make(map[string]int64, len(logFiles))

	for _, logPath := range logFiles {
		base := filepath.Base(logPath)
		startOffset := int64(0)
		if !opts.Force {
			startOffset = cp.FileOffsets[base]
		}

		f, err := os.Open(logPath)
		if err != nil {
			slog.Warn("failed to open log file", "path", logPath, "err", err)
			continue
		}

		if startOffset > 0 {
			if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
				slog.Warn("failed to seek log file", "path", logPath, "err", err)
				f.Close()
				continue
			}
		}

		parseErr := parser.Parse(f, func(e TelemetryEvent) error {
			// Apply --since filter: skip events with a known timestamp that
			// precedes the cutoff. Zero-value timestamps are always included.
			if opts.Since != nil && !e.Timestamp.IsZero() && e.Timestamp.Before(*opts.Since) {
				return nil
			}
			events = append(events, e)
			return nil
		})

		// Record the current EOF as the new offset for this file.
		if eofOffset, seekErr := f.Seek(0, io.SeekEnd); seekErr == nil {
			newOffsets[base] = eofOffset
		}
		f.Close()
		if parseErr != nil {
			slog.Warn("log parse error", "path", logPath, "err", parseErr)
		}
	}
	return events, newOffsets, nil
}

// readSessionJSONL reads session_summary and tool_usage records from the JSONL,
// skipping records whose session_id is in excludeIDs (nil map includes all).
// Returns nil slices (not an error) when the file does not exist.
func readSessionJSONL(jsonlPath string, excludeIDs map[string]bool) ([]SessionSummaryRecord, []ToolUsageRecord, error) {
	f, err := os.Open(jsonlPath)
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("open telemetry JSONL: %w", err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	var sessions []SessionSummaryRecord
	var tools []ToolUsageRecord

	for {
		rawLine, readErr := reader.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return nil, nil, fmt.Errorf("read telemetry JSONL: %w", readErr)
		}
		isEOF := readErr == io.EOF
		raw := []byte(strings.TrimRight(rawLine, "\r\n"))
		if len(raw) > 0 {
			var hdr struct {
				RecordType string `json:"record_type"`
				SessionID  string `json:"session_id"`
			}
			if err := json.Unmarshal(raw, &hdr); err == nil {
				if !excludeIDs[hdr.SessionID] {
					switch hdr.RecordType {
					case "session_summary":
						var r SessionSummaryRecord
						if err := json.Unmarshal(raw, &r); err == nil {
							sessions = append(sessions, r)
						}
					case "tool_usage":
						var r ToolUsageRecord
						if err := json.Unmarshal(raw, &r); err == nil {
							tools = append(tools, r)
						}
					}
				}
			}
		}
		if isEOF {
			break
		}
	}
	return sessions, tools, nil
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
// Prior records (from incremental merge) are written first, followed by new records.
// Uses a temp-file-then-rename atomic write to prevent a corrupt file on crash.
func writeTelemetryJSONL(
	jsonlPath string,
	summaries []SessionSummary,
	toolStats map[toolKey]toolStat,
	serverCallsPerSession map[string]map[string]int,
	harvestedAt time.Time,
	priorSessions []SessionSummaryRecord,
	priorTools []ToolUsageRecord,
) error {
	tmpPath := jsonlPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp %s: %w", tmpPath, err)
	}

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	// Write prior session summaries unchanged.
	for _, rec := range priorSessions {
		if err := enc.Encode(rec); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode prior session summary %q: %w", rec.SessionID, err)
		}
	}

	// Write new session summaries with context window fields.
	for _, s := range summaries {
		rec := SessionSummaryRecord{
			RecordType:        "session_summary",
			HarvestedAt:       harvestedAt,
			SessionID:         s.SessionID,
			Branch:            s.Branch,
			Repository:        s.Repository,
			TotalTokens:       s.TotalTokens,
			PromptTokens:      s.PromptTokens,
			CompletionTokens:  s.CompletionTokens,
			CachedTokens:      s.CachedTokens,
			ModelCalls:        s.ModelCalls,
			ToolCalls:         s.ToolCalls,
			TokensByModel:     s.TokensByModel,
			TokensByServer:    s.TokensByServer,
			ToolCallsByServer: serverCallsPerSession[s.SessionID],
			CompletedTasks:    s.CompletedTasks,
			TokensPerTask:     s.TokensPerTask,
			CompactionCount:   len(s.CompactionEvents),
		}
		if cw := s.ContextWindow; cw != nil {
			rec.PeakUtilization = &cw.PeakUtilization
			rec.RemainingCapacity = &cw.RemainingCapacity
			rec.DepletionRate = &cw.DepletionRate
			rec.MaxContextTokens = &cw.MaxContextTokens
		}
		if err := enc.Encode(rec); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode session summary %q: %w", s.SessionID, err)
		}
	}

	// Write prior tool usage records unchanged.
	for _, rec := range priorTools {
		if err := enc.Encode(rec); err != nil {
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode prior tool usage %q/%q: %w", rec.SessionID, rec.ToolName, err)
		}
	}

	// Write new tool usage records.
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
			f.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encode tool usage %q/%q: %w", key.sessionID, key.toolName, err)
		}
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync %s: %w", tmpPath, err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close %s: %w", tmpPath, err)
	}
	// On POSIX, os.Rename atomically replaces the destination (no pre-remove needed).
	// On Windows, os.Rename fails when the destination already exists; remove first.
	// The removal window is narrow and acceptable for regenerable telemetry files.
	if runtime.GOOS == "windows" {
		_ = os.Remove(jsonlPath)
	}
	if err := os.Rename(tmpPath, jsonlPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("commit %s: %w", jsonlPath, err)
	}
	return nil
}

// groupModelCallsBySession groups model call events by their session ID.
func groupModelCallsBySession(events []TelemetryEvent) map[string][]ModelCall {
	result := make(map[string][]ModelCall)
	for _, e := range events {
		if e.Kind == EventKindModelCall && e.ModelCall != nil {
			sid := e.ModelCall.SessionID
			result[sid] = append(result[sid], *e.ModelCall)
		}
	}
	return result
}

// sessionIDSet returns a set of session IDs for fast membership testing.
func sessionIDSet(summaries []SessionSummary) map[string]bool {
	set := make(map[string]bool, len(summaries))
	for _, s := range summaries {
		set[s.SessionID] = true
	}
	return set
}
