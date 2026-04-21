package telemetry

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	berrors "github.com/softwaresalt/backlogit/internal/errors"
)

// ReportFormat identifies a supported report output encoding.
type ReportFormat string

const (
	// FormatTable renders aligned plaintext tables.
	FormatTable ReportFormat = "table"
	// FormatJSON renders JSON output.
	FormatJSON ReportFormat = "json"
	// FormatMarkdown renders GitHub-flavored Markdown tables.
	FormatMarkdown ReportFormat = "markdown"
)

// ReportOptions configures the telemetry report output produced by
// GenerateReport.
type ReportOptions struct {
	// SessionID, when non-empty, restricts the report to a single session.
	SessionID string
	// GroupBy controls the aggregation dimension. Valid values: "server",
	// "model", "session", "tool".
	GroupBy string
	// Format controls the output encoding. Valid values: "table", "json", "markdown".
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

	groupBy := opts.GroupBy
	if groupBy == "" {
		groupBy = "session"
	}
	// Validate groupBy before allocating aggregation structures.
	switch groupBy {
	case "session", "server":
		// supported
	default:
		return "", fmt.Errorf("unsupported group-by value %q: supported values are \"session\", \"server\"", groupBy)
	}

	format := opts.Format
	if format == "" {
		format = string(FormatTable)
	}

	switch ReportFormat(format) {
	case FormatTable:
		return formatReportTable(sessions, tools, groupBy, opts.Limit)
	case FormatJSON:
		return formatReportJSON(sessions, tools, groupBy, opts.Limit)
	case FormatMarkdown:
		return formatReportMarkdown(sessions, tools, groupBy, opts.Limit)
	default:
		return "", fmt.Errorf(
			"unsupported format value %q: supported values are %q, %q, %q: %w",
			format,
			FormatTable,
			FormatJSON,
			FormatMarkdown,
			berrors.ErrValidation,
		)
	}
}

func formatReportTable(sessions []SessionSummaryRecord, _ []ToolUsageRecord, groupBy string, limit int) (string, error) {
	switch groupBy {
	case "server":
		return formatServerTable(sessions, limit), nil
	default:
		return formatSessionTable(sessions, limit), nil
	}
}

func formatReportJSON(sessions []SessionSummaryRecord, _ []ToolUsageRecord, groupBy string, limit int) (string, error) {
	var payload any
	switch groupBy {
	case "server":
		aggregate := make(map[string]int)
		for _, s := range sessions {
			for server, count := range s.ToolCallsByServer {
				aggregate[server] += count
			}
		}
		// Apply limit to aggregated server rows.
		if limit > 0 {
			servers := make([]string, 0, len(aggregate))
			for sv := range aggregate {
				servers = append(servers, sv)
			}
			sort.Strings(servers)
			if len(servers) > limit {
				servers = servers[:limit]
			}
			trimmed := make(map[string]int, len(servers))
			for _, sv := range servers {
				trimmed[sv] = aggregate[sv]
			}
			payload = trimmed
		} else {
			payload = aggregate
		}
	default:
		rows := sessions
		if limit > 0 && len(rows) > limit {
			rows = rows[:limit]
		}
		payload = rows
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}
	return string(data) + "\n", nil
}

func formatSessionTable(sessions []SessionSummaryRecord, limit int) string {
	rows := sessions
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-36s  %8s  %11s  %10s\n",
		"SESSION", "TOKENS", "MODEL_CALLS", "TOOL_CALLS"))
	sb.WriteString(fmt.Sprintf("%-36s  %8s  %11s  %10s\n",
		strings.Repeat("-", 36), strings.Repeat("-", 8),
		strings.Repeat("-", 11), strings.Repeat("-", 10)))
	for _, s := range rows {
		sb.WriteString(fmt.Sprintf("%-36s  %8d  %11d  %10d\n",
			s.SessionID, s.TotalTokens, s.ModelCalls, s.ToolCalls))
	}
	return sb.String()
}

func formatServerTable(sessions []SessionSummaryRecord, limit int) string {
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
	// Apply limit to the aggregated server list.
	if limit > 0 && len(servers) > limit {
		servers = servers[:limit]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-24s  %10s\n", "SERVER", "TOOL_CALLS"))
	sb.WriteString(fmt.Sprintf("%-24s  %10s\n", strings.Repeat("-", 24), strings.Repeat("-", 10)))
	for _, server := range servers {
		sb.WriteString(fmt.Sprintf("%-24s  %10d\n", server, aggregate[server]))
	}
	return sb.String()
}

func formatReportMarkdown(sessions []SessionSummaryRecord, _ []ToolUsageRecord, groupBy string, limit int) (string, error) {
	switch groupBy {
	case "server":
		return formatServerMarkdown(sessions, limit), nil
	default:
		return formatSessionMarkdown(sessions, limit), nil
	}
}

func formatSessionMarkdown(sessions []SessionSummaryRecord, limit int) string {
	rows := append([]SessionSummaryRecord(nil), sessions...)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].SessionID < rows[j].SessionID
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	var sb strings.Builder
	sb.WriteString("# Telemetry Report\n\n")
	sb.WriteString("## Session Summary\n\n")
	sb.WriteString("| Session | Tokens | Model Calls | Tool Calls |\n")
	sb.WriteString("|---|---:|---:|---:|\n")
	for _, row := range rows {
		sb.WriteString(fmt.Sprintf(
			"| %s | %d | %d | %d |\n",
			escapeMarkdownCell(row.SessionID),
			row.TotalTokens,
			row.ModelCalls,
			row.ToolCalls,
		))
	}
	return sb.String()
}

func formatServerMarkdown(sessions []SessionSummaryRecord, limit int) string {
	aggregate := make(map[string]int)
	for _, session := range sessions {
		for server, count := range session.ToolCallsByServer {
			aggregate[server] += count
		}
	}

	servers := make([]string, 0, len(aggregate))
	for server := range aggregate {
		servers = append(servers, server)
	}
	sort.Strings(servers)
	if limit > 0 && len(servers) > limit {
		servers = servers[:limit]
	}

	var sb strings.Builder
	sb.WriteString("# Telemetry Report\n\n")
	sb.WriteString("## Tool Calls by Server\n\n")
	sb.WriteString("| Server | Tool Calls |\n")
	sb.WriteString("|---|---:|\n")
	for _, server := range servers {
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", escapeMarkdownCell(server), aggregate[server]))
	}
	return sb.String()
}

func escapeMarkdownCell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}
