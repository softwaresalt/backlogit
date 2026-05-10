package telemetry

// FactFieldDescriptor describes a single field in a telemetry JSONL fact table.
type FactFieldDescriptor struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	JSONKey  string `json:"json_key"`
	Optional bool   `json:"optional,omitempty"`
}

// FactTableSchema describes the schema of a single telemetry JSONL fact table.
type FactTableSchema struct {
	Name       string                `json:"name"`
	File       string                `json:"file"`
	RecordType string                `json:"record_type"`
	Fields     []FactFieldDescriptor `json:"fields"`
}

// DescribeFactTables returns the schema reference for all telemetry JSONL
// fact tables. This manual registry must stay in sync with the struct
// definitions in records.go.
func DescribeFactTables() []FactTableSchema {
	return []FactTableSchema{
		{
			Name:       "session_summary",
			File:       "telemetry-sessions.jsonl",
			RecordType: "session_summary",
			Fields:     sessionSummaryFields(),
		},
		{
			Name:       "tool_usage",
			File:       "telemetry-sessions.jsonl",
			RecordType: "tool_usage",
			Fields:     toolUsageFields(),
		},
		{
			Name:       "tool_call_fact",
			File:       "telemetry/tool-calls.jsonl",
			RecordType: "tool_call_fact",
			Fields:     toolCallFactFields(),
		},
		{
			Name:       "session_fact",
			File:       "telemetry/session-facts.jsonl",
			RecordType: "session_fact",
			Fields:     sessionFactFields(),
		},
	}
}

// DescribeTelemetrySQLTables returns the schema reference for telemetry SQL
// tables. This describes the SQLite tables managed by EnsureTelemetrySchema.
func DescribeTelemetrySQLTables() []FactTableSchema {
	return []FactTableSchema{
		{
			Name:       "telemetry_sessions",
			File:       "backlogit.db",
			RecordType: "sql_table",
			Fields:     telemetrySessionsSQLFields(),
		},
		{
			Name:       "telemetry_tool_usage",
			File:       "backlogit.db",
			RecordType: "sql_table",
			Fields:     telemetryToolUsageSQLFields(),
		},
	}
}

func sessionSummaryFields() []FactFieldDescriptor {
	return []FactFieldDescriptor{
		{Name: "RecordType", Type: "string", JSONKey: "record_type"},
		{Name: "HarvestedAt", Type: "time.Time", JSONKey: "harvested_at"},
		{Name: "SessionID", Type: "string", JSONKey: "session_id"},
		{Name: "Branch", Type: "string", JSONKey: "branch"},
		{Name: "Repository", Type: "string", JSONKey: "repository"},
		{Name: "TotalTokens", Type: "int", JSONKey: "total_tokens"},
		{Name: "PromptTokens", Type: "int", JSONKey: "prompt_tokens"},
		{Name: "CompletionTokens", Type: "int", JSONKey: "completion_tokens"},
		{Name: "CachedTokens", Type: "int", JSONKey: "cached_tokens"},
		{Name: "ModelCalls", Type: "int", JSONKey: "model_calls"},
		{Name: "ToolCalls", Type: "int", JSONKey: "tool_calls"},
		{Name: "TokensByModel", Type: "map[string]int", JSONKey: "tokens_by_model"},
		{Name: "TokensByServer", Type: "map[string]int", JSONKey: "tokens_by_server", Optional: true},
		{Name: "ToolCallsByServer", Type: "map[string]int", JSONKey: "tool_calls_by_server"},
		{Name: "CompletedTasks", Type: "[]string", JSONKey: "completed_tasks"},
		{Name: "TokensPerTask", Type: "*float64", JSONKey: "tokens_per_task"},
		{Name: "CompactionCount", Type: "int", JSONKey: "compaction_count"},
		{Name: "PeakUtilization", Type: "*float64", JSONKey: "peak_utilization", Optional: true},
		{Name: "RemainingCapacity", Type: "*int", JSONKey: "remaining_capacity", Optional: true},
		{Name: "DepletionRate", Type: "*float64", JSONKey: "depletion_rate", Optional: true},
		{Name: "MaxContextTokens", Type: "*int", JSONKey: "max_context_tokens", Optional: true},
		{Name: "ModelClass", Type: "string", JSONKey: "model_class", Optional: true},
		{Name: "ReasoningLevel", Type: "string", JSONKey: "reasoning_level", Optional: true},
	}
}

func toolUsageFields() []FactFieldDescriptor {
	return []FactFieldDescriptor{
		{Name: "RecordType", Type: "string", JSONKey: "record_type"},
		{Name: "HarvestedAt", Type: "time.Time", JSONKey: "harvested_at"},
		{Name: "SessionID", Type: "string", JSONKey: "session_id"},
		{Name: "ServerName", Type: "string", JSONKey: "server_name"},
		{Name: "ToolName", Type: "string", JSONKey: "tool_name"},
		{Name: "CallCount", Type: "int", JSONKey: "call_count"},
		{Name: "TotalDurMs", Type: "int", JSONKey: "total_duration_ms"},
	}
}

func toolCallFactFields() []FactFieldDescriptor {
	return []FactFieldDescriptor{
		{Name: "RecordType", Type: "string", JSONKey: "record_type"},
		{Name: "HarvestedAt", Type: "time.Time", JSONKey: "harvested_at"},
		{Name: "SessionID", Type: "string", JSONKey: "session_id"},
		{Name: "Branch", Type: "string", JSONKey: "branch", Optional: true},
		{Name: "Repository", Type: "string", JSONKey: "repository", Optional: true},
		{Name: "ToolName", Type: "string", JSONKey: "tool_name"},
		{Name: "ServerName", Type: "string", JSONKey: "server_name", Optional: true},
		{Name: "IsBuiltin", Type: "bool", JSONKey: "is_builtin"},
		{Name: "TurnID", Type: "string", JSONKey: "turn_id", Optional: true},
		{Name: "Model", Type: "string", JSONKey: "model", Optional: true},
		{Name: "StartedAt", Type: "time.Time", JSONKey: "started_at"},
		{Name: "CompletedAt", Type: "time.Time", JSONKey: "completed_at"},
		{Name: "DurationMs", Type: "int64", JSONKey: "duration_ms"},
		{Name: "Success", Type: "bool", JSONKey: "success"},
	}
}

func sessionFactFields() []FactFieldDescriptor {
	return []FactFieldDescriptor{
		{Name: "RecordType", Type: "string", JSONKey: "record_type"},
		{Name: "HarvestedAt", Type: "time.Time", JSONKey: "harvested_at"},
		{Name: "SessionID", Type: "string", JSONKey: "session_id"},
		{Name: "Branch", Type: "string", JSONKey: "branch", Optional: true},
		{Name: "Repository", Type: "string", JSONKey: "repository", Optional: true},
		{Name: "StartedAt", Type: "time.Time", JSONKey: "started_at", Optional: true},
		{Name: "ShutdownAt", Type: "time.Time", JSONKey: "shutdown_at", Optional: true},
		{Name: "TotalApiDurationMs", Type: "int64", JSONKey: "total_api_duration_ms", Optional: true},
		{Name: "TotalPremiumRequests", Type: "int", JSONKey: "total_premium_requests", Optional: true},
		{Name: "ModelMetrics", Type: "map[string]ModelUsageMetrics", JSONKey: "model_metrics", Optional: true},
		{Name: "CurrentTokens", Type: "int", JSONKey: "current_tokens", Optional: true},
		{Name: "SystemTokens", Type: "int", JSONKey: "system_tokens", Optional: true},
		{Name: "ConversationTokens", Type: "int", JSONKey: "conversation_tokens", Optional: true},
		{Name: "ToolDefinitionsTokens", Type: "int", JSONKey: "tool_definitions_tokens", Optional: true},
		{Name: "ToolCallCount", Type: "int", JSONKey: "tool_call_count", Optional: true},
	}
}

func telemetrySessionsSQLFields() []FactFieldDescriptor {
	return []FactFieldDescriptor{
		{Name: "session_id", Type: "TEXT", JSONKey: "session_id"},
		{Name: "harvested_at", Type: "DATETIME", JSONKey: "harvested_at"},
		{Name: "branch", Type: "TEXT", JSONKey: "branch"},
		{Name: "repository", Type: "TEXT", JSONKey: "repository"},
		{Name: "total_tokens", Type: "INTEGER", JSONKey: "total_tokens"},
		{Name: "prompt_tokens", Type: "INTEGER", JSONKey: "prompt_tokens"},
		{Name: "completion_tokens", Type: "INTEGER", JSONKey: "completion_tokens"},
		{Name: "cached_tokens", Type: "INTEGER", JSONKey: "cached_tokens"},
		{Name: "model_calls", Type: "INTEGER", JSONKey: "model_calls"},
		{Name: "tool_calls", Type: "INTEGER", JSONKey: "tool_calls"},
		{Name: "tokens_by_model", Type: "TEXT", JSONKey: "tokens_by_model"},
		{Name: "tokens_by_server", Type: "TEXT", JSONKey: "tokens_by_server"},
		{Name: "tool_calls_by_server", Type: "TEXT", JSONKey: "tool_calls_by_server"},
		{Name: "tokens_per_task", Type: "REAL", JSONKey: "tokens_per_task"},
		{Name: "compaction_count", Type: "INTEGER", JSONKey: "compaction_count"},
	}
}

func telemetryToolUsageSQLFields() []FactFieldDescriptor {
	return []FactFieldDescriptor{
		{Name: "session_id", Type: "TEXT", JSONKey: "session_id"},
		{Name: "server_name", Type: "TEXT", JSONKey: "server_name"},
		{Name: "tool_name", Type: "TEXT", JSONKey: "tool_name"},
		{Name: "call_count", Type: "INTEGER", JSONKey: "call_count"},
		{Name: "total_duration_ms", Type: "INTEGER", JSONKey: "total_duration_ms"},
		{Name: "harvested_at", Type: "DATETIME", JSONKey: "harvested_at"},
	}
}
