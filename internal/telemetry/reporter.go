package telemetry

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ReportOptions configures the telemetry report output produced by
// GenerateReport.
type ReportOptions struct {
	// SessionID, when non-empty, restricts the report to a single session.
	SessionID string
	// GroupBy controls the aggregation dimension. Valid values: "server",
	// "model", "session", "tool".
	GroupBy string
	// Format controls the output encoding. Valid values: "table", "json".
	Format string
	// Limit, when > 0, restricts the number of rows returned. Used by the
	// "top" subcommand to implement top-N behaviour.
	Limit int
}

// GenerateReport reads telemetry-sessions.jsonl from
// <workspacePath>/.backlogit/ and produces a formatted report string
// according to opts. Returns an informative message (not an error) when no
// harvested data exists.
func GenerateReport(workspacePath string, opts ReportOptions) (string, error) {
	jsonlPath := filepath.Join(workspacePath, ".backlogit", "telemetry-sessions.jsonl")
	sessions, tools, err := readSessionJSONL(jsonlPath, nil)
	if err != nil {
		return "", fmt.Errorf("read telemetry data: %w", err)
	}

	// Filter by session ID when requested.
	if opts.SessionID != "" {
		var filteredSessions []SessionSummaryRecord
		var filteredTools []ToolUsageRecord
		for _, s := range sessions {
			if s.SessionID == opts.SessionID {
				filteredSessions = append(filteredSessions, s)
			}
		}
		for _, t := range tools {
			if t.SessionID == opts.SessionID {
				filteredTools = append(filteredTools, t)
			}
		}
		sessions = filteredSessions
		tools = filteredTools
	}

	if len(sessions) == 0 && len(tools) == 0 {
		return "No telemetry data found. Run `backlogit telemetry harvest` first.\n", nil
	}

	// Apply row limit to session results.
	if opts.Limit > 0 && len(sessions) > opts.Limit {
		sessions = sessions[:opts.Limit]
	}

	groupBy := opts.GroupBy
	if groupBy == "" {
		groupBy = "session"
	}
	format := opts.Format
	if format == "" {
		format = "table"
	}

	switch format {
	case "json":
		return formatReportJSON(sessions, tools, groupBy)
	default:
		return formatReportTable(sessions, tools, groupBy)
	}
}

func formatReportTable(sessions []SessionSummaryRecord, _ []ToolUsageRecord, groupBy string) (string, error) {
	switch groupBy {
	case "server":
		return formatServerTable(sessions), nil
	default:
		return formatSessionTable(sessions), nil
	}
}

func formatReportJSON(sessions []SessionSummaryRecord, _ []ToolUsageRecord, groupBy string) (string, error) {
	var payload any
	switch groupBy {
	case "server":
		aggregate := make(map[string]int)
		for _, s := range sessions {
			for server, count := range s.ToolCallsByServer {
				aggregate[server] += count
			}
		}
		payload = aggregate
	default:
		payload = sessions
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}
	return string(data) + "\n", nil
}

func formatSessionTable(sessions []SessionSummaryRecord) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-36s  %8s  %11s  %10s\n",
		"SESSION", "TOKENS", "MODEL_CALLS", "TOOL_CALLS"))
	sb.WriteString(fmt.Sprintf("%-36s  %8s  %11s  %10s\n",
		strings.Repeat("-", 36), strings.Repeat("-", 8),
		strings.Repeat("-", 11), strings.Repeat("-", 10)))
	for _, s := range sessions {
		sb.WriteString(fmt.Sprintf("%-36s  %8d  %11d  %10d\n",
			s.SessionID, s.TotalTokens, s.ModelCalls, s.ToolCalls))
	}
	return sb.String()
}

func formatServerTable(sessions []SessionSummaryRecord) string {
	aggregate := make(map[string]int)
	for _, s := range sessions {
		for server, count := range s.ToolCallsByServer {
			aggregate[server] += count
		}
	}
	servers := make([]string, 0, len(aggregate))
	for s := range aggregate {
		servers = append(servers, s)
	}
	sort.Strings(servers)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-24s  %10s\n", "SERVER", "TOOL_CALLS"))
	sb.WriteString(fmt.Sprintf("%-24s  %10s\n", strings.Repeat("-", 24), strings.Repeat("-", 10)))
	for _, server := range servers {
		sb.WriteString(fmt.Sprintf("%-24s  %10d\n", server, aggregate[server]))
	}
	return sb.String()
}
