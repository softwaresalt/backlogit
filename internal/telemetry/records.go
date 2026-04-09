package telemetry

import "time"

// SessionSummaryRecord is the typed JSONL record written to
// .backlogit/telemetry-sessions.jsonl. One record per session per harvest run.
//
// Using typed struct fields instead of map[string]any enforces the contract
// that token counts are always integers and server/model maps are always
// map[string]int (Plan Review F4).
type SessionSummaryRecord struct {
	RecordType       string         `json:"record_type"` // "session_summary"
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
	TokensByModel    map[string]int `json:"tokens_by_model"`
	TokensByServer   map[string]int `json:"tokens_by_server"`
	CompletedTasks   []string       `json:"completed_tasks"`
	TokensPerTask    *float64       `json:"tokens_per_task"`
	CompactionCount  int            `json:"compaction_count"`
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
