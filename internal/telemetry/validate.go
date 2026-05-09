package telemetry

import "log/slog"

// ValidateSessionSummary reports whether s represents a complete, harvestable
// session. A session is invalid (partial) when it has tool calls recorded but
// zero tokens — this occurs when oversized log entries are dropped by the parser,
// leaving tool calls without corresponding model-call token attribution.
//
// Callers in the harvest pipeline must reject invalid sessions to prevent partial
// zero-token records from being written to telemetry-sessions.jsonl.
func ValidateSessionSummary(s SessionSummary) bool {
	if s.TotalTokens == 0 && s.ToolCalls > 0 {
		slog.Warn("skipping partial session: zero tokens with recorded tool calls",
			"session_id", s.SessionID,
			"tool_calls", s.ToolCalls,
		)
		return false
	}
	return true
}

// IsGhostSession reports whether s is a fully inactive (ghost) session.
// A ghost session has zero total tokens, zero model calls, and zero tool calls.
// Ghost sessions are excluded from trend report aggregation and averages so
// that abandoned or zero-activity sessions do not distort per-session metrics.
func IsGhostSession(s SessionSummaryRecord) bool {
	return s.TotalTokens == 0 && s.ModelCalls == 0 && s.ToolCalls == 0
}
