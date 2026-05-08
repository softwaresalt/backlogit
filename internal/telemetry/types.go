// Package telemetry harvests Copilot CLI telemetry from the workspace-scoped
// .copilot/ directory, correlates events with backlogit task completions,
// attributes tool calls to their originating MCP servers, and exposes metrics
// through backlogit's existing SQL query surface.
//
// Constitutional note: read-only access to .copilot/ is a documented exception
// to Principle IV (Workspace Containment). All writes target .backlogit/.
package telemetry

import (
	"io"
	"time"
)

// EventKind identifies the type of a telemetry event.
type EventKind string

const (
	// EventKindModelCall represents a cli.model_call event from Copilot CLI logs.
	EventKindModelCall EventKind = "cli.model_call"
	// EventKindToolCall represents a cli.tool_call event from Copilot CLI logs.
	EventKindToolCall EventKind = "cli.tool_call"
)

// ModelCall holds data extracted from a cli.model_call telemetry event.
type ModelCall struct {
	SessionID        string `json:"session_id"`
	RequestID        string `json:"request_id"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens_count"`
	CompletionTokens int    `json:"completion_tokens_count"`
	TotalTokens      int    `json:"total_tokens_count"`
	CachedTokens     int    `json:"cached_tokens_count"`
	DurationMs       int    `json:"duration_ms"`
}

// ToolCall holds data extracted from a cli.tool_call telemetry event.
type ToolCall struct {
	SessionID   string `json:"session_id"`
	ModelCallID string `json:"model_call_id"`
	ToolName    string `json:"tool_name"`
	ResultType  string `json:"result_type"`
	DurationMs  int    `json:"duration_ms"`
}

// TelemetryEvent is a union of model and tool telemetry events parsed from a
// Copilot CLI process log. Exactly one of ModelCall or ToolCall is set.
// Timestamp is populated from the log-line prefix (e.g. "2026-04-09T00:00:02.000Z")
// and used for --since filtering. It is zero when the prefix is absent or
// unparseable.
type TelemetryEvent struct {
	Kind      EventKind  `json:"kind"`
	ModelCall *ModelCall `json:"model_call,omitempty"`
	ToolCall  *ToolCall  `json:"tool_call,omitempty"`
	// Timestamp is the wall-clock time extracted from the log-line prefix.
	// Zero when unavailable or malformed.
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// CompactionEvent holds data from a session.compaction_complete event in a
// session-state events.jsonl file.
type CompactionEvent struct {
	Timestamp           string `json:"timestamp"`
	PreCompactionTokens int    `json:"preCompactionTokens"`
	InputTokens         int    `json:"input"`
	OutputTokens        int    `json:"output"`
	CachedInputTokens   int    `json:"cachedInput"`
}

// SessionMeta aggregates session metadata merged from session-state events.jsonl
// and the session-store.db SQLite database.
type SessionMeta struct {
	SessionID        string            `json:"session_id"`
	Branch           string            `json:"branch"`
	Repository       string            `json:"repository"`
	WorkDir          string            `json:"work_dir"`
	StartedAt        string            `json:"started_at"`
	CompactionEvents []CompactionEvent `json:"compaction_events"`
}

// SessionSummary holds fully correlated telemetry for a single session.
type SessionSummary struct {
	SessionID        string         `json:"session_id"`
	Branch           string         `json:"branch"`
	Repository       string         `json:"repository"`
	TotalTokens      int            `json:"total_tokens"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	CachedTokens     int            `json:"cached_tokens"`
	ModelCalls       int            `json:"model_calls"`
	ToolCalls        int            `json:"tool_calls"`
	TokensByModel    map[string]int `json:"tokens_by_model"`
	// TokensByServer maps server name to its proportional token allocation for
	// this session. Allocation is computed by distributing TotalTokens across
	// servers in proportion to their share of tool calls.
	TokensByServer   map[string]int    `json:"tokens_by_server"`
	CompletedTasks   []string          `json:"completed_tasks"`
	TokensPerTask    *float64          `json:"tokens_per_task"`
	CompactionEvents []CompactionEvent `json:"compaction_events"`
	// ContextWindow holds derived context utilisation for this session.
	// Nil when no model calls are recorded or model is unknown.
	ContextWindow *ContextWindowMetrics `json:"context_window,omitempty"`
}

// LogParser is the streaming interface for parsing telemetry log sources.
// Implementations call emit for each parsed TelemetryEvent. Malformed input
// lines are skipped with a slog debug log; the parse continues on error-free lines.
type LogParser interface {
	Parse(r io.Reader, emit func(TelemetryEvent) error) error
}
