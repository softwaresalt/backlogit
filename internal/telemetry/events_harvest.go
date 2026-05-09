package telemetry

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// rawNewEvent is the wrapper for new-format events.jsonl entries.
// New-format events use "type" (not "event_type") with a nested "data" block.
type rawNewEvent struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
	Timestamp string          `json:"timestamp"`
	ID        string          `json:"id"`
}

// rawToolStartData maps the data block of a tool.execution_start event.
type rawToolStartData struct {
	ToolCallID    string `json:"toolCallId"`
	ToolName      string `json:"toolName"`      // qualified: "backlogit-backlogit_move_item" or "powershell"
	MCPServerName string `json:"mcpServerName"` // empty for built-in tools
	MCPToolName   string `json:"mcpToolName"`   // short name for MCP tools; absent for built-ins
	TurnID        string `json:"turnId"`
}

// rawToolCompleteData maps the data block of a tool.execution_complete event.
type rawToolCompleteData struct {
	ToolCallID string `json:"toolCallId"`
	Model      string `json:"model"`
	TurnID     string `json:"turnId"`
	Success    bool   `json:"success"`
}

// rawSessionStartData maps the data block of a session.start event.
type rawSessionStartData struct {
	SessionID string `json:"sessionId"`
	StartTime string `json:"startTime"`
	Context   struct {
		Branch     string `json:"branch"`
		Repository string `json:"repository"`
	} `json:"context"`
}

// rawShutdownData maps the data block of a session.shutdown event.
type rawShutdownData struct {
	TotalApiDurationMs   int64 `json:"totalApiDurationMs"`
	TotalPremiumRequests int   `json:"totalPremiumRequests"`
	ModelMetrics         map[string]struct {
		Requests struct {
			Count int `json:"count"`
			Cost  int `json:"cost"`
		} `json:"requests"`
		Usage struct {
			InputTokens      int `json:"inputTokens"`
			OutputTokens     int `json:"outputTokens"`
			CacheReadTokens  int `json:"cacheReadTokens"`
			CacheWriteTokens int `json:"cacheWriteTokens"`
			ReasoningTokens  int `json:"reasoningTokens"`
		} `json:"usage"`
	} `json:"modelMetrics"`
	CurrentTokens         int `json:"currentTokens"`
	SystemTokens          int `json:"systemTokens"`
	ConversationTokens    int `json:"conversationTokens"`
	ToolDefinitionsTokens int `json:"toolDefinitionsTokens"`
}

// pendingToolStart holds the start-event data for an in-flight tool call
// awaiting its matching tool.execution_complete event.
type pendingToolStart struct {
	toolName   string
	serverName string
	turnID     string
	timestamp  time.Time
}

// ParseEventFacts reads an events.jsonl stream and extracts ToolCallFact records
// (from matched tool.execution_start + tool.execution_complete pairs) and a
// SessionFact (from the session.shutdown event, when present).
//
// Unmatched tool starts (e.g., active sessions mid-flight) are silently dropped.
// The returned sessionFact is nil when no shutdown event was found.
func ParseEventFacts(r io.Reader, sessionID string) ([]ToolCallFact, *SessionFact, error) {
	pending := make(map[string]pendingToolStart)
	var facts []ToolCallFact
	var sessionFact *SessionFact
	var branch, repository string
	var sessionStartedAt time.Time

	reader := bufio.NewReader(r)
	for {
		rawLine, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, nil, fmt.Errorf("scan events: %w", readErr)
		}
		isEOF := errors.Is(readErr, io.EOF)
		line := strings.TrimRight(rawLine, "\r\n")
		if line != "" {
			processEventFactLine(line, sessionID, pending, &facts, &sessionFact, &branch, &repository, &sessionStartedAt)
		}
		if isEOF {
			break
		}
	}

	if sessionFact != nil {
		sessionFact.ToolCallCount = len(facts)
	}
	return facts, sessionFact, nil
}

// processEventFactLine handles one JSON line from events.jsonl.
// It updates the mutable state passed in by pointer.
func processEventFactLine(
	line, sessionID string,
	pending map[string]pendingToolStart,
	facts *[]ToolCallFact,
	sessionFact **SessionFact,
	branch, repository *string,
	sessionStartedAt *time.Time,
) {
	var evt rawNewEvent
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		slog.Debug("skip malformed events.jsonl line", "err", err)
		return
	}

	switch evt.Type {
	case "session.start":
		var d rawSessionStartData
		if err := json.Unmarshal(evt.Data, &d); err != nil {
			return
		}
		*branch = d.Context.Branch
		*repository = d.Context.Repository
		if t, err := time.Parse(time.RFC3339Nano, d.StartTime); err == nil {
			*sessionStartedAt = t
		} else if t, err := time.Parse(time.RFC3339, d.StartTime); err == nil {
			*sessionStartedAt = t
		}

	case "tool.execution_start":
		var d rawToolStartData
		if err := json.Unmarshal(evt.Data, &d); err != nil {
			return
		}
		ts := parseEventTimestamp(evt.Timestamp)
		pending[d.ToolCallID] = pendingToolStart{
			toolName:   d.ToolName,
			serverName: d.MCPServerName,
			turnID:     d.TurnID,
			timestamp:  ts,
		}

	case "tool.execution_complete":
		var d rawToolCompleteData
		if err := json.Unmarshal(evt.Data, &d); err != nil {
			return
		}
		start, ok := pending[d.ToolCallID]
		if !ok {
			return // unmatched complete — skip
		}
		completedAt := parseEventTimestamp(evt.Timestamp)
		durMs := completedAt.Sub(start.timestamp).Milliseconds()
		// Use the short MCP tool name when available; fall back to the full name.
		toolName := start.toolName
		if start.serverName != "" && strings.HasPrefix(toolName, start.serverName+"-") {
			toolName = strings.TrimPrefix(toolName, start.serverName+"-")
		}
		*facts = append(*facts, ToolCallFact{
			RecordType:  "tool_call_fact",
			SessionID:   sessionID,
			Branch:      *branch,
			Repository:  *repository,
			ToolName:    toolName,
			ServerName:  start.serverName,
			IsBuiltin:   start.serverName == "",
			TurnID:      start.turnID,
			Model:       d.Model,
			StartedAt:   start.timestamp,
			CompletedAt: completedAt,
			DurationMs:  durMs,
			Success:     d.Success,
		})
		delete(pending, d.ToolCallID)

	case "session.shutdown":
		var d rawShutdownData
		if err := json.Unmarshal(evt.Data, &d); err != nil {
			return
		}
		shutdownAt := parseEventTimestamp(evt.Timestamp)
		sf := &SessionFact{
			RecordType:            "session_fact",
			SessionID:             sessionID,
			Branch:                *branch,
			Repository:            *repository,
			StartedAt:             *sessionStartedAt,
			ShutdownAt:            shutdownAt,
			TotalApiDurationMs:    d.TotalApiDurationMs,
			TotalPremiumRequests:  d.TotalPremiumRequests,
			CurrentTokens:         d.CurrentTokens,
			SystemTokens:          d.SystemTokens,
			ConversationTokens:    d.ConversationTokens,
			ToolDefinitionsTokens: d.ToolDefinitionsTokens,
		}
		if len(d.ModelMetrics) > 0 {
			sf.ModelMetrics = make(map[string]ModelUsageMetrics, len(d.ModelMetrics))
			for model, mm := range d.ModelMetrics {
				sf.ModelMetrics[model] = ModelUsageMetrics{
					RequestCount:     mm.Requests.Count,
					RequestCost:      mm.Requests.Cost,
					InputTokens:      mm.Usage.InputTokens,
					OutputTokens:     mm.Usage.OutputTokens,
					CacheReadTokens:  mm.Usage.CacheReadTokens,
					CacheWriteTokens: mm.Usage.CacheWriteTokens,
					ReasoningTokens:  mm.Usage.ReasoningTokens,
				}
			}
		}
		*sessionFact = sf
	}
}

// parseEventTimestamp tries RFC3339Nano then RFC3339 to handle both precisions.
func parseEventTimestamp(s string) time.Time {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// HarvestEventsFacts scans .copilot/session-state/ for events.jsonl files and
// appends ToolCallFact and SessionFact records to .backlogit/telemetry/.
//
// Fully-processed sessions (those with a session.shutdown event) are tracked in
// cp.ProcessedEventSessions and skipped on subsequent harvests.
// When force is true, all sessions are re-processed and existing fact files cleared.
//
// Returns the number of tool call facts and session facts written.
func HarvestEventsFacts(copilotPath, workspacePath string, cp *HarvestCheckpoint, harvestedAt time.Time, force bool) (int, int, error) {
	sessionStateDir := filepath.Join(copilotPath, "session-state")
	entries, err := os.ReadDir(sessionStateDir)
	if os.IsNotExist(err) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, fmt.Errorf("read session-state dir: %w", err)
	}

	telemetryDir := filepath.Join(workspacePath, ".backlogit", "telemetry")
	if err := os.MkdirAll(telemetryDir, 0o755); err != nil {
		return 0, 0, fmt.Errorf("create telemetry dir: %w", err)
	}

	if force {
		cp.ProcessedEventSessions = make(map[string]bool)
		for _, name := range []string{"tool-calls.jsonl", "session-facts.jsonl"} {
			p := filepath.Join(telemetryDir, name)
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				slog.Warn("failed to clear fact file", "path", p, "err", err)
			}
		}
	}
	if cp.ProcessedEventSessions == nil {
		cp.ProcessedEventSessions = make(map[string]bool)
	}

	toolCallsPath := filepath.Join(telemetryDir, "tool-calls.jsonl")
	sessionFactsPath := filepath.Join(telemetryDir, "session-facts.jsonl")

	tcFile, err := os.OpenFile(toolCallsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, 0, fmt.Errorf("open tool-calls.jsonl: %w", err)
	}
	defer tcFile.Close()

	sfFile, err := os.OpenFile(sessionFactsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, 0, fmt.Errorf("open session-facts.jsonl: %w", err)
	}
	defer sfFile.Close()

	tcEnc := json.NewEncoder(tcFile)
	tcEnc.SetEscapeHTML(false)
	sfEnc := json.NewEncoder(sfFile)
	sfEnc.SetEscapeHTML(false)

	totalToolCalls := 0
	totalSessionFacts := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionID := entry.Name()
		if cp.ProcessedEventSessions[sessionID] {
			continue
		}

		eventsPath := filepath.Join(sessionStateDir, sessionID, "events.jsonl")
		f, err := os.Open(eventsPath)
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("failed to open events.jsonl", "session", sessionID, "err", err)
			}
			continue
		}

		facts, sessionFact, err := ParseEventFacts(f, sessionID)
		f.Close()
		if err != nil {
			slog.Warn("failed to parse event facts", "session", sessionID, "err", err)
			continue
		}

		for i := range facts {
			facts[i].HarvestedAt = harvestedAt
			if encErr := tcEnc.Encode(facts[i]); encErr != nil {
				return totalToolCalls, totalSessionFacts, fmt.Errorf("encode tool call fact: %w", encErr)
			}
		}
		totalToolCalls += len(facts)

		if sessionFact != nil {
			sessionFact.HarvestedAt = harvestedAt
			if encErr := sfEnc.Encode(sessionFact); encErr != nil {
				return totalToolCalls, totalSessionFacts, fmt.Errorf("encode session fact: %w", encErr)
			}
			totalSessionFacts++
			cp.ProcessedEventSessions[sessionID] = true
		}
	}

	return totalToolCalls, totalSessionFacts, nil
}
