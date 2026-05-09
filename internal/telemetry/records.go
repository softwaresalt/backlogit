package telemetry

import (
	"strings"
	"time"
)

// SessionSummaryRecord is the typed JSONL record written to
// .backlogit/telemetry-sessions.jsonl. One record per session per harvest run.
//
// Using typed struct fields instead of map[string]any enforces the contract
// that token counts are always integers and server/model maps are always
// map[string]int (Plan Review F4).
type SessionSummaryRecord struct {
	RecordType        string         `json:"record_type"` // "session_summary"
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
	TokensByModel     map[string]int `json:"tokens_by_model"`
	TokensByServer    map[string]int `json:"tokens_by_server,omitempty"`
	ToolCallsByServer map[string]int `json:"tool_calls_by_server"`
	CompletedTasks    []string       `json:"completed_tasks"`
	TokensPerTask     *float64       `json:"tokens_per_task"`
	CompactionCount   int            `json:"compaction_count"`
	// Context window metrics — nil when model calls are unavailable.
	PeakUtilization   *float64 `json:"peak_utilization,omitempty"`
	RemainingCapacity *int     `json:"remaining_capacity,omitempty"`
	DepletionRate     *float64 `json:"depletion_rate,omitempty"`
	MaxContextTokens  *int     `json:"max_context_tokens,omitempty"`
	// Model-awareness fields derived from TokensByModel at harvest time.
	// Both are omitempty: older records without these fields remain valid.
	ModelClass     string `json:"model_class,omitempty"`
	ReasoningLevel string `json:"reasoning_level,omitempty"`
}

// ToolUsageRecord is the typed JSONL record for per-server tool call counts
// within a single session. The composite (session_id, server_name, tool_name)
// is unique per harvest (Plan Review F7).
type ToolUsageRecord struct {
	RecordType  string    `json:"record_type"` // "tool_usage"
	HarvestedAt time.Time `json:"harvested_at"`
	SessionID   string    `json:"session_id"`
	ServerName  string    `json:"server_name"`
	ToolName    string    `json:"tool_name"`
	CallCount   int       `json:"call_count"`
	TotalDurMs  int       `json:"total_duration_ms"`
}

// DeriveModelClass returns a coarse model-class label from a model name string.
//
// Rules (applied in order):
//   - empty string → ""
//   - contains "sonnet" → "sonnet"
//   - contains "haiku"  → "haiku"
//   - contains "opus"   → "opus"
//   - starts with "gpt" → "gpt"
//   - starts with "o1", "o3", or "o4" → "o-series"
//   - fallback → "other"
func DeriveModelClass(model string) string {
	if model == "" {
		return ""
	}
	lower := strings.ToLower(model)
	switch {
	case strings.Contains(lower, "sonnet"):
		return "sonnet"
	case strings.Contains(lower, "haiku"):
		return "haiku"
	case strings.Contains(lower, "opus"):
		return "opus"
	case strings.HasPrefix(lower, "gpt"):
		return "gpt"
	case strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") || strings.HasPrefix(lower, "o4"):
		return "o-series"
	default:
		return "other"
	}
}

// DeriveReasoningLevel returns a reasoning-level label from a model name.
// Returns "high" for full o1/o3/o4 models, "low" for mini variants,
// and empty string for all other models (including empty input).
func DeriveReasoningLevel(model string) string {
	lower := strings.ToLower(model)
	if strings.HasSuffix(lower, "-mini") {
		// o*-mini variants: only matches o1-mini, o3-mini, o4-mini.
		if strings.HasPrefix(lower, "o1-") || strings.HasPrefix(lower, "o3-") || strings.HasPrefix(lower, "o4-") {
			return "low"
		}
		return ""
	}
	// Full o1, o3, o4 (exact names only; variants like o1-preview return empty).
	if lower == "o1" || lower == "o3" || lower == "o4" {
		return "high"
	}
	return ""
}

// ModelUsageMetrics holds per-model token and request metrics extracted from
// a session.shutdown event in events.jsonl.
type ModelUsageMetrics struct {
	RequestCount     int `json:"request_count"`
	RequestCost      int `json:"request_cost"` // in "premium requests"
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	CacheReadTokens  int `json:"cache_read_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
	ReasoningTokens  int `json:"reasoning_tokens"`
}

// ToolCallFact is the typed JSONL record for a single completed tool call
// harvested from session-state events.jsonl. One record per matched
// tool.execution_start + tool.execution_complete pair.
// Stored in .backlogit/telemetry/tool-calls.jsonl.
type ToolCallFact struct {
	RecordType  string    `json:"record_type"` // "tool_call_fact"
	HarvestedAt time.Time `json:"harvested_at"`
	SessionID   string    `json:"session_id"`
	Branch      string    `json:"branch,omitempty"`
	Repository  string    `json:"repository,omitempty"`
	ToolName    string    `json:"tool_name"`             // short name (mcpToolName for MCP, toolName for built-ins)
	ServerName  string    `json:"server_name,omitempty"` // empty for built-in tools
	IsBuiltin   bool      `json:"is_builtin"`
	TurnID      string    `json:"turn_id,omitempty"`
	Model       string    `json:"model,omitempty"` // model that triggered the call
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	DurationMs  int64     `json:"duration_ms"`
	Success     bool      `json:"success"`
}

// SessionFact is the typed JSONL record for aggregate session metrics harvested
// from a session.shutdown event in events.jsonl. Includes per-model token
// breakdowns, context window composition, and API duration.
// Stored in .backlogit/telemetry/session-facts.jsonl.
type SessionFact struct {
	RecordType            string                       `json:"record_type"` // "session_fact"
	HarvestedAt           time.Time                    `json:"harvested_at"`
	SessionID             string                       `json:"session_id"`
	Branch                string                       `json:"branch,omitempty"`
	Repository            string                       `json:"repository,omitempty"`
	StartedAt             time.Time                    `json:"started_at,omitempty"`
	ShutdownAt            time.Time                    `json:"shutdown_at,omitempty"`
	TotalApiDurationMs    int64                        `json:"total_api_duration_ms,omitempty"`
	TotalPremiumRequests  int                          `json:"total_premium_requests,omitempty"`
	ModelMetrics          map[string]ModelUsageMetrics `json:"model_metrics,omitempty"`
	CurrentTokens         int                          `json:"current_tokens,omitempty"`
	SystemTokens          int                          `json:"system_tokens,omitempty"`
	ConversationTokens    int                          `json:"conversation_tokens,omitempty"`
	ToolDefinitionsTokens int                          `json:"tool_definitions_tokens,omitempty"`
	ToolCallCount         int                          `json:"tool_call_count,omitempty"`
}

// PrimaryModel returns the name of the model with the highest token count
// from tokensByModel. When multiple models share the maximum, the
// alphabetically first name is returned for deterministic output.
// Returns empty string when the map is nil or empty.
func PrimaryModel(tokensByModel map[string]int) string {
	var best string
	var bestCount int
	for model, count := range tokensByModel {
		if count > bestCount || (count == bestCount && (best == "" || model < best)) {
			best = model
			bestCount = count
		}
	}
	return best
}
