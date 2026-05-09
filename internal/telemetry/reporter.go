package telemetry

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
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
	// GroupBy controls the aggregation dimension. Valid values: "session",
	// "server", "model", "class".
	GroupBy string
	// Format controls the output encoding. Valid values: "table", "json", "markdown".
	Format ReportFormat
	// Limit, when > 0, restricts the number of rows returned. Used by the
	// "top" subcommand to implement top-N behaviour.
	Limit int
}

// GenerateReport reads telemetry-sessions.jsonl from
// <workspacePath>/.backlogit/ and produces a formatted report string
// according to opts. Returns an informative message (not an error) when no
// harvested data exists.
//
// GroupBy values backed by telemetry-sessions.jsonl: "session", "server",
// "model", "class". GroupBy values backed by fact JSONL files: "tool",
// "context".
func GenerateReport(workspacePath string, opts ReportOptions) (string, error) {
	// "tool" and "context" are backed by the fact tables, not telemetry-sessions.jsonl.
	if opts.GroupBy == "tool" || opts.GroupBy == "context" {
		format := opts.Format
		if format == "" {
			format = FormatTable
		}
		return GenerateFactsReport(workspacePath, opts.GroupBy, format, opts.Limit)
	}

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
	case "session", "server", "model", "class":
		// supported — backed by telemetry-sessions.jsonl
	default:
		return "", fmt.Errorf("unsupported group-by value %q: supported values are \"session\", \"server\", \"model\", \"class\", \"tool\", \"context\"", groupBy)
	}

	format := opts.Format
	if format == "" {
		format = FormatTable
	}

	switch format {
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
	case "model":
		return formatModelTable(sessions, limit, false), nil
	case "class":
		return formatModelTable(sessions, limit, true), nil
	default:
		return formatSessionTable(sessions, limit), nil
	}
}

func formatReportJSON(sessions []SessionSummaryRecord, _ []ToolUsageRecord, groupBy string, limit int) (string, error) {
	var payload any
	switch groupBy {
	case "server":
		tokensByServer := proportionalServerTokens(sessions)
		// Apply limit: sort by tokens descending before trimming.
		if limit > 0 {
			type serverRow struct {
				name   string
				tokens int
			}
			rows := make([]serverRow, 0, len(tokensByServer))
			for sv, tok := range tokensByServer {
				rows = append(rows, serverRow{sv, tok})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].tokens != rows[j].tokens {
					return rows[i].tokens > rows[j].tokens
				}
				return rows[i].name < rows[j].name
			})
			if len(rows) > limit {
				rows = rows[:limit]
			}
			trimmed := make(map[string]int, len(rows))
			for _, r := range rows {
				trimmed[r.name] = r.tokens
			}
			payload = trimmed
		} else {
			payload = tokensByServer
		}
	case "model":
		payload = buildModelGroups(sessions, false, limit)
	case "class":
		payload = buildModelGroups(sessions, true, limit)
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
	const modelWidth = 20
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-36s  %8s  %11s  %10s  %-*s\n",
		"SESSION", "TOKENS", "MODEL_CALLS", "TOOL_CALLS", modelWidth, "PRIMARY_MODEL"))
	sb.WriteString(fmt.Sprintf("%-36s  %8s  %11s  %10s  %-*s\n",
		strings.Repeat("-", 36), strings.Repeat("-", 8),
		strings.Repeat("-", 11), strings.Repeat("-", 10),
		modelWidth, strings.Repeat("-", modelWidth)))
	for _, s := range rows {
		sessionDisplay := s.SessionID
		if IsGhostSession(s) {
			sessionDisplay = s.SessionID + " [empty]"
		}
		pm := PrimaryModel(s.TokensByModel)
		if pm == "" {
			pm = "-"
		}
		if len(pm) > modelWidth {
			pm = pm[:modelWidth]
		}
		sb.WriteString(fmt.Sprintf("%-36s  %8d  %11d  %10d  %-*s\n",
			sessionDisplay, s.TotalTokens, s.ModelCalls, s.ToolCalls, modelWidth, pm))
	}
	return sb.String()
}

// proportionalServerTokens computes each server's proportional token share across
// all sessions. For each session, a server's share is:
//
//	server_tokens += TotalTokens × (server_calls / total_calls)
//
// Accumulation uses float64 to avoid integer-division truncation. The returned
// map values are rounded to the nearest integer for display.
func proportionalServerTokens(sessions []SessionSummaryRecord) map[string]int {
	raw := make(map[string]float64)
	for _, s := range sessions {
		totalCalls := 0
		for _, count := range s.ToolCallsByServer {
			totalCalls += count
		}
		if totalCalls == 0 {
			continue
		}
		for server, count := range s.ToolCallsByServer {
			raw[server] += float64(s.TotalTokens) * float64(count) / float64(totalCalls)
		}
	}
	result := make(map[string]int, len(raw))
	for sv, v := range raw {
		result[sv] = int(math.Round(v))
	}
	return result
}

func formatServerTable(sessions []SessionSummaryRecord, limit int) string {
	tokensByServer := proportionalServerTokens(sessions)

	// Build a sorted list of servers, descending by proportional token attribution.
	// Ties broken alphabetically for stable output.
	type serverRow struct {
		name   string
		tokens int
	}
	rows := make([]serverRow, 0, len(tokensByServer))
	for sv, tok := range tokensByServer {
		rows = append(rows, serverRow{name: sv, tokens: tok})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].tokens != rows[j].tokens {
			return rows[i].tokens > rows[j].tokens
		}
		return rows[i].name < rows[j].name
	})

	// Apply limit after token sort so the top-N by tokens are returned.
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-24s  %10s\n", "SERVER", "TOKENS"))
	sb.WriteString(fmt.Sprintf("%-24s  %10s\n", strings.Repeat("-", 24), strings.Repeat("-", 10)))
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("%-24s  %10d\n", r.name, r.tokens))
	}
	return sb.String()
}

func formatReportMarkdown(sessions []SessionSummaryRecord, _ []ToolUsageRecord, groupBy string, limit int) (string, error) {
	switch groupBy {
	case "server":
		return formatServerMarkdown(sessions, limit), nil
	case "model":
		return formatModelMarkdown(sessions, limit, false), nil
	case "class":
		return formatModelMarkdown(sessions, limit, true), nil
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
	sb.WriteString("| Session | Tokens | Model Calls | Tool Calls | Primary Model |\n")
	sb.WriteString("|---|---:|---:|---:|---|\n")
	for _, row := range rows {
		sessionDisplay := escapeMarkdownCell(row.SessionID)
		if IsGhostSession(row) {
			sessionDisplay = escapeMarkdownCell(row.SessionID) + " [empty]"
		}
		pm := PrimaryModel(row.TokensByModel)
		if pm == "" {
			pm = "-"
		}
		sb.WriteString(fmt.Sprintf(
			"| %s | %d | %d | %d | %s |\n",
			sessionDisplay,
			row.TotalTokens,
			row.ModelCalls,
			row.ToolCalls,
			escapeMarkdownCell(pm),
		))
	}
	return sb.String()
}

func formatServerMarkdown(sessions []SessionSummaryRecord, limit int) string {
	tokensByServer := proportionalServerTokens(sessions)

	type serverRow struct {
		name   string
		tokens int
	}
	rows := make([]serverRow, 0, len(tokensByServer))
	for sv, tok := range tokensByServer {
		rows = append(rows, serverRow{name: sv, tokens: tok})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].tokens != rows[j].tokens {
			return rows[i].tokens > rows[j].tokens
		}
		return rows[i].name < rows[j].name
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	var sb strings.Builder
	sb.WriteString("# Telemetry Report\n\n")
	sb.WriteString("## Tokens by Server\n\n")
	sb.WriteString("| Server | Tokens |\n")
	sb.WriteString("|---|---:|\n")
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("| %s | %d |\n", escapeMarkdownCell(r.name), r.tokens))
	}
	return sb.String()
}

func escapeMarkdownCell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

// TrendOptions configures the telemetry trend report.
type TrendOptions struct {
	// By controls the grouping dimension. Valid values: "date", "branch", "class".
	// Defaults to "date".
	By string
	// Format controls the output encoding: "table", "json", "markdown".
	Format ReportFormat
	// Limit, when > 0, restricts the number of groups returned.
	Limit int
}

// TrendGroup holds aggregated metrics for one date or branch group.
type TrendGroup struct {
	Group            string   `json:"group"`
	Sessions         int      `json:"sessions"`
	TotalTokens      int      `json:"total_tokens"`
	AvgTokensSession float64  `json:"avg_tokens_per_session"`
	AvgTokensTask    *float64 `json:"avg_tokens_per_task,omitempty"`
	AvgPeakUtil      *float64 `json:"avg_peak_utilization,omitempty"`
	AvgModelCalls    float64  `json:"avg_model_calls"`
	AvgToolCalls     float64  `json:"avg_tool_calls"`
}

// GenerateTrendReport reads telemetry-sessions.jsonl and produces a trend
// report grouped by date or branch. Returns an informative message when no
// harvested data exists.
func GenerateTrendReport(workspacePath string, opts TrendOptions) (string, error) {
	jsonlPath := filepath.Join(workspacePath, ".backlogit", "telemetry-sessions.jsonl")
	sessions, _, err := readSessionJSONL(jsonlPath, nil)
	if err != nil {
		return "", fmt.Errorf("read telemetry data: %w", err)
	}
	if len(sessions) == 0 {
		return "No telemetry data found. Run `backlogit telemetry harvest` first.\n", nil
	}

	by := opts.By
	if by == "" {
		by = "date"
	}
	if by != "date" && by != "branch" && by != "class" {
		return "", fmt.Errorf("unsupported By value %q: valid values are \"date\", \"branch\", \"class\"", by)
	}

	// Group sessions.
	groupIndex := make(map[string]int)
	var groups []TrendGroup

	for _, s := range sessions {
		if IsGhostSession(s) {
			continue
		}
		var key string
		switch by {
		case "branch":
			key = s.Branch
			if key == "" {
				key = "(unknown)"
			}
		case "class":
			key = s.ModelClass
			if key == "" {
				key = DeriveModelClass(PrimaryModel(s.TokensByModel))
			}
			if key == "" {
				key = "(unknown)"
			}
		default: // "date"
			key = s.HarvestedAt.UTC().Format("2006-01-02")
		}
		idx, ok := groupIndex[key]
		if !ok {
			idx = len(groups)
			groupIndex[key] = idx
			groups = append(groups, TrendGroup{Group: key})
		}
		g := &groups[idx]
		g.Sessions++
		g.TotalTokens += s.TotalTokens
		g.AvgModelCalls += float64(s.ModelCalls)
		g.AvgToolCalls += float64(s.ToolCalls)
		if s.TokensPerTask != nil {
			if g.AvgTokensTask == nil {
				v := *s.TokensPerTask
				g.AvgTokensTask = &v
			} else {
				// Running sum; divide later.
				*g.AvgTokensTask += *s.TokensPerTask
			}
		}
		if s.PeakUtilization != nil {
			if g.AvgPeakUtil == nil {
				v := *s.PeakUtilization
				g.AvgPeakUtil = &v
			} else {
				*g.AvgPeakUtil += *s.PeakUtilization
			}
		}
	}

	// Finalise averages.
	taskCounts := make(map[string]int)
	peakCounts := make(map[string]int)
	for _, s := range sessions {
		if IsGhostSession(s) {
			continue
		}
		var key string
		switch by {
		case "branch":
			key = s.Branch
			if key == "" {
				key = "(unknown)"
			}
		case "class":
			key = s.ModelClass
			if key == "" {
				key = DeriveModelClass(PrimaryModel(s.TokensByModel))
			}
			if key == "" {
				key = "(unknown)"
			}
		default:
			key = s.HarvestedAt.UTC().Format("2006-01-02")
		}
		if s.TokensPerTask != nil {
			taskCounts[key]++
		}
		if s.PeakUtilization != nil {
			peakCounts[key]++
		}
	}
	for i := range groups {
		g := &groups[i]
		if g.Sessions > 0 {
			g.AvgTokensSession = float64(g.TotalTokens) / float64(g.Sessions)
			g.AvgModelCalls /= float64(g.Sessions)
			g.AvgToolCalls /= float64(g.Sessions)
		}
		if g.AvgTokensTask != nil && taskCounts[g.Group] > 1 {
			*g.AvgTokensTask /= float64(taskCounts[g.Group])
		}
		if g.AvgPeakUtil != nil && peakCounts[g.Group] > 1 {
			*g.AvgPeakUtil /= float64(peakCounts[g.Group])
		}
	}

	// Sort: chronologically for date, alphabetically for branch.
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Group < groups[j].Group
	})

	// Apply limit.
	if opts.Limit > 0 && len(groups) > opts.Limit {
		groups = groups[:opts.Limit]
	}

	format := opts.Format
	if format == "" {
		format = FormatTable
	}
	switch format {
	case FormatJSON:
		data, err := json.MarshalIndent(groups, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal trend report: %w", err)
		}
		return string(data) + "\n", nil
	case FormatMarkdown:
		return formatTrendMarkdown(groups), nil
	case FormatTable:
		return formatTrendTable(groups), nil
	default:
		return "", fmt.Errorf("unsupported format %q: valid values are \"table\", \"json\", \"markdown\"", format)
	}
}

func formatTrendTable(groups []TrendGroup) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-12s  %8s  %12s  %18s  %15s  %13s  %15s  %14s\n",
		"GROUP", "SESSIONS", "TOTAL_TOKENS", "AVG_TOKENS/SESSION", "AVG_TOKENS/TASK", "AVG_PEAK_UTIL", "AVG_MODEL_CALLS", "AVG_TOOL_CALLS"))
	sb.WriteString(fmt.Sprintf("%-12s  %8s  %12s  %18s  %15s  %13s  %15s  %14s\n",
		strings.Repeat("-", 12), strings.Repeat("-", 8), strings.Repeat("-", 12),
		strings.Repeat("-", 18), strings.Repeat("-", 15), strings.Repeat("-", 13),
		strings.Repeat("-", 15), strings.Repeat("-", 14)))
	for _, g := range groups {
		tpt := "-"
		if g.AvgTokensTask != nil {
			tpt = fmt.Sprintf("%.0f", *g.AvgTokensTask)
		}
		pu := "-"
		if g.AvgPeakUtil != nil {
			pu = fmt.Sprintf("%.1f%%", *g.AvgPeakUtil*100)
		}
		sb.WriteString(fmt.Sprintf("%-12s  %8d  %12d  %18.0f  %15s  %13s  %15.1f  %14.1f\n",
			g.Group, g.Sessions, g.TotalTokens, g.AvgTokensSession, tpt, pu,
			g.AvgModelCalls, g.AvgToolCalls))
	}
	return sb.String()
}

func formatTrendMarkdown(groups []TrendGroup) string {
	var sb strings.Builder
	sb.WriteString("# Telemetry Trend Report\n\n")
	sb.WriteString("| Group | Sessions | Total Tokens | Avg Tokens/Session | Avg Tokens/Task | Avg Peak Util | Avg Model Calls | Avg Tool Calls |\n")
	sb.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, g := range groups {
		tpt := "-"
		if g.AvgTokensTask != nil {
			tpt = fmt.Sprintf("%.0f", *g.AvgTokensTask)
		}
		pu := "-"
		if g.AvgPeakUtil != nil {
			pu = fmt.Sprintf("%.1f%%", *g.AvgPeakUtil*100)
		}
		sb.WriteString(fmt.Sprintf("| %s | %d | %d | %.0f | %s | %s | %.1f | %.1f |\n",
			escapeMarkdownCell(g.Group), g.Sessions, g.TotalTokens, g.AvgTokensSession, tpt, pu,
			g.AvgModelCalls, g.AvgToolCalls))
	}
	return sb.String()
}

// ModelGroup holds aggregated metrics for one model-name or model-class group
// used by --by model and --by class report dimensions.
type ModelGroup struct {
	// Group is the primary model name (--by model) or class label (--by class).
	Group    string `json:"group"`
	Sessions int    `json:"sessions"`
	Tokens   int    `json:"total_tokens"`
}

// buildModelGroups aggregates sessions by primary model name (byClass=false)
// or model class (byClass=true). Rows are sorted by tokens descending then
// name ascending. Limit is applied after sort.
func buildModelGroups(sessions []SessionSummaryRecord, byClass bool, limit int) []ModelGroup {
	index := make(map[string]int)
	var groups []ModelGroup
	for _, s := range sessions {
		if IsGhostSession(s) {
			continue
		}
		key := PrimaryModel(s.TokensByModel)
		if byClass {
			if s.ModelClass != "" {
				key = s.ModelClass
			} else {
				key = DeriveModelClass(PrimaryModel(s.TokensByModel))
			}
		}
		if key == "" {
			key = "(unknown)"
		}
		idx, ok := index[key]
		if !ok {
			idx = len(groups)
			index[key] = idx
			groups = append(groups, ModelGroup{Group: key})
		}
		groups[idx].Sessions++
		groups[idx].Tokens += s.TotalTokens
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Tokens != groups[j].Tokens {
			return groups[i].Tokens > groups[j].Tokens
		}
		return groups[i].Group < groups[j].Group
	})
	if limit > 0 && len(groups) > limit {
		groups = groups[:limit]
	}
	return groups
}

func formatModelTable(sessions []SessionSummaryRecord, limit int, byClass bool) string {
	groups := buildModelGroups(sessions, byClass, limit)
	header := "MODEL"
	if byClass {
		header = "CLASS"
	}
	const nameWidth = 24
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-*s  %8s  %10s\n", nameWidth, header, "TOKENS", "SESSIONS"))
	sb.WriteString(fmt.Sprintf("%-*s  %8s  %10s\n",
		nameWidth, strings.Repeat("-", nameWidth),
		strings.Repeat("-", 8), strings.Repeat("-", 10)))
	for _, g := range groups {
		name := g.Group
		if len(name) > nameWidth {
			name = name[:nameWidth]
		}
		sb.WriteString(fmt.Sprintf("%-*s  %8d  %10d\n", nameWidth, name, g.Tokens, g.Sessions))
	}
	return sb.String()
}

func formatModelMarkdown(sessions []SessionSummaryRecord, limit int, byClass bool) string {
	groups := buildModelGroups(sessions, byClass, limit)
	header := "Model"
	if byClass {
		header = "Class"
	}
	var sb strings.Builder
	sb.WriteString("# Telemetry Report\n\n")
	sb.WriteString(fmt.Sprintf("## Tokens by %s\n\n", header))
	sb.WriteString(fmt.Sprintf("| %s | Tokens | Sessions |\n", header))
	sb.WriteString("|---|---:|---:|\n")
	for _, g := range groups {
		sb.WriteString(fmt.Sprintf("| %s | %d | %d |\n",
			escapeMarkdownCell(g.Group), g.Tokens, g.Sessions))
	}
	return sb.String()
}

// ─── Fact-table reporters ──────────────────────────────────────────────────

// toolCallAggregate holds rolled-up stats for one tool.
type toolCallAggregate struct {
	ToolName     string
	ServerName   string
	IsBuiltin    bool
	CallCount    int
	SuccessCount int
	TotalMs      int64
	Sessions     map[string]struct{}
}

// GenerateFactsReport reads fact JSONL files from .backlogit/telemetry/ and
// produces a formatted report. Supported groupBy values: "tool", "context".
func GenerateFactsReport(workspacePath, groupBy string, format ReportFormat, limit int) (string, error) {
	telDir := filepath.Join(workspacePath, ".backlogit", "telemetry")

	switch groupBy {
	case "tool":
		facts, err := readToolCallFacts(filepath.Join(telDir, "tool-calls.jsonl"))
		if err != nil {
			return "", fmt.Errorf("read tool-calls.jsonl: %w", err)
		}
		if len(facts) == 0 {
			return "No tool call fact data found. Run `backlogit telemetry harvest` first.\n", nil
		}
		return formatToolFacts(facts, format, limit)
	case "context":
		sfacts, err := readSessionFacts(filepath.Join(telDir, "session-facts.jsonl"))
		if err != nil {
			return "", fmt.Errorf("read session-facts.jsonl: %w", err)
		}
		if len(sfacts) == 0 {
			return "No session fact data found. Run `backlogit telemetry harvest` first.\n", nil
		}
		return formatContextFacts(sfacts, format, limit)
	default:
		return "", fmt.Errorf("unsupported group-by %q for facts report: valid values are \"tool\", \"context\"", groupBy)
	}
}

func readToolCallFacts(path string) ([]ToolCallFact, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var facts []ToolCallFact
	dec := json.NewDecoder(f)
	for dec.More() {
		var rec ToolCallFact
		if err := dec.Decode(&rec); err != nil {
			continue // skip corrupt lines
		}
		facts = append(facts, rec)
	}
	return facts, nil
}

func readSessionFacts(path string) ([]SessionFact, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var facts []SessionFact
	dec := json.NewDecoder(f)
	for dec.More() {
		var rec SessionFact
		if err := dec.Decode(&rec); err != nil {
			continue
		}
		facts = append(facts, rec)
	}
	return facts, nil
}

// aggregateToolFacts groups ToolCallFact records by tool name.
func aggregateToolFacts(facts []ToolCallFact) []toolCallAggregate {
	index := make(map[string]*toolCallAggregate)
	for _, f := range facts {
		agg, ok := index[f.ToolName]
		if !ok {
			agg = &toolCallAggregate{
				ToolName:   f.ToolName,
				ServerName: f.ServerName,
				IsBuiltin:  f.IsBuiltin,
				Sessions:   make(map[string]struct{}),
			}
			index[f.ToolName] = agg
		}
		agg.CallCount++
		if f.Success {
			agg.SuccessCount++
		}
		agg.TotalMs += f.DurationMs
		agg.Sessions[f.SessionID] = struct{}{}
	}

	rows := make([]toolCallAggregate, 0, len(index))
	for _, a := range index {
		rows = append(rows, *a)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CallCount != rows[j].CallCount {
			return rows[i].CallCount > rows[j].CallCount
		}
		return rows[i].ToolName < rows[j].ToolName
	})
	return rows
}

func formatToolFacts(facts []ToolCallFact, format ReportFormat, limit int) (string, error) {
	rows := aggregateToolFacts(facts)
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	switch format {
	case FormatJSON:
		type jsonRow struct {
			ToolName        string  `json:"tool_name"`
			ServerName      string  `json:"server_name,omitempty"`
			IsBuiltin       bool    `json:"is_builtin"`
			CallCount       int     `json:"call_count"`
			SuccessPct      float64 `json:"success_pct"`
			AvgDurationMs   float64 `json:"avg_duration_ms"`
			TotalDurationMs int64   `json:"total_duration_ms"`
			Sessions        int     `json:"sessions"`
		}
		out := make([]jsonRow, 0, len(rows))
		for _, r := range rows {
			sp := 0.0
			if r.CallCount > 0 {
				sp = 100.0 * float64(r.SuccessCount) / float64(r.CallCount)
			}
			ad := 0.0
			if r.CallCount > 0 {
				ad = float64(r.TotalMs) / float64(r.CallCount)
			}
			out = append(out, jsonRow{
				ToolName:        r.ToolName,
				ServerName:      r.ServerName,
				IsBuiltin:       r.IsBuiltin,
				CallCount:       r.CallCount,
				SuccessPct:      math.Round(sp*10) / 10,
				AvgDurationMs:   math.Round(ad*10) / 10,
				TotalDurationMs: r.TotalMs,
				Sessions:        len(r.Sessions),
			})
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal tool facts: %w", err)
		}
		return string(data) + "\n", nil

	case FormatMarkdown:
		var sb strings.Builder
		sb.WriteString("# Tool Call Report\n\n")
		sb.WriteString("| Tool | Server | Calls | Success% | Avg ms | Total ms | Sessions |\n")
		sb.WriteString("|---|---|---:|---:|---:|---:|---:|\n")
		for _, r := range rows {
			sp := 0.0
			if r.CallCount > 0 {
				sp = 100.0 * float64(r.SuccessCount) / float64(r.CallCount)
			}
			ad := 0.0
			if r.CallCount > 0 {
				ad = float64(r.TotalMs) / float64(r.CallCount)
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %d | %.1f | %.0f | %d | %d |\n",
				escapeMarkdownCell(r.ToolName),
				escapeMarkdownCell(r.ServerName),
				r.CallCount,
				sp, ad, r.TotalMs,
				len(r.Sessions),
			))
		}
		return sb.String(), nil

	default: // table
		const nameWidth = 36
		const srvWidth = 20
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %6s  %8s  %8s  %11s  %8s\n",
			nameWidth, "TOOL", srvWidth, "SERVER",
			"CALLS", "SUCCESS%", "AVG_MS", "TOTAL_MS", "SESSIONS"))
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %6s  %8s  %8s  %11s  %8s\n",
			nameWidth, strings.Repeat("-", nameWidth),
			srvWidth, strings.Repeat("-", srvWidth),
			strings.Repeat("-", 6), strings.Repeat("-", 8),
			strings.Repeat("-", 8), strings.Repeat("-", 11),
			strings.Repeat("-", 8)))
		for _, r := range rows {
			sp := 0.0
			if r.CallCount > 0 {
				sp = 100.0 * float64(r.SuccessCount) / float64(r.CallCount)
			}
			ad := 0.0
			if r.CallCount > 0 {
				ad = float64(r.TotalMs) / float64(r.CallCount)
			}
			name := r.ToolName
			if len(name) > nameWidth {
				name = name[:nameWidth]
			}
			srv := r.ServerName
			if srv == "" {
				srv = "(builtin)"
			}
			if len(srv) > srvWidth {
				srv = srv[:srvWidth]
			}
			sb.WriteString(fmt.Sprintf("%-*s  %-*s  %6d  %7.1f%%  %8.0f  %11d  %8d\n",
				nameWidth, name,
				srvWidth, srv,
				r.CallCount, sp, ad, r.TotalMs,
				len(r.Sessions),
			))
		}
		return sb.String(), nil
	}
}

func formatContextFacts(facts []SessionFact, format ReportFormat, limit int) (string, error) {
	rows := append([]SessionFact(nil), facts...)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].StartedAt.After(rows[j].StartedAt)
	})
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}

	type contextRow struct {
		SessionID  string  `json:"session_id"`
		Branch     string  `json:"branch,omitempty"`
		SystemPct  float64 `json:"system_pct"`
		ConvPct    float64 `json:"conversation_pct"`
		ToolDefPct float64 `json:"tool_definitions_pct"`
		CachePct   float64 `json:"cache_read_pct"`
		CurrentCtx int     `json:"current_context_tokens"`
		TotalInput int     `json:"total_input_tokens"`
	}

	toRow := func(sf SessionFact) contextRow {
		var totalInput, cacheRead int
		for _, mm := range sf.ModelMetrics {
			totalInput += mm.InputTokens
			cacheRead += mm.CacheReadTokens
		}
		total := sf.SystemTokens + sf.ConversationTokens + sf.ToolDefinitionsTokens
		if total == 0 {
			total = 1
		}
		cachePct := 0.0
		if totalInput > 0 {
			cachePct = 100.0 * float64(cacheRead) / float64(totalInput)
		}
		return contextRow{
			SessionID:  sf.SessionID,
			Branch:     sf.Branch,
			SystemPct:  100.0 * float64(sf.SystemTokens) / float64(total),
			ConvPct:    100.0 * float64(sf.ConversationTokens) / float64(total),
			ToolDefPct: 100.0 * float64(sf.ToolDefinitionsTokens) / float64(total),
			CachePct:   cachePct,
			CurrentCtx: sf.CurrentTokens,
			TotalInput: totalInput,
		}
	}

	switch format {
	case FormatJSON:
		out := make([]contextRow, 0, len(rows))
		for _, sf := range rows {
			out = append(out, toRow(sf))
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return "", fmt.Errorf("marshal context facts: %w", err)
		}
		return string(data) + "\n", nil

	case FormatMarkdown:
		var sb strings.Builder
		sb.WriteString("# Context Window Report\n\n")
		sb.WriteString("| Session | Branch | System% | Conv% | ToolDef% | Cache% | Context | Total Input |\n")
		sb.WriteString("|---|---|---:|---:|---:|---:|---:|---:|\n")
		for _, sf := range rows {
			r := toRow(sf)
			sb.WriteString(fmt.Sprintf("| %s | %s | %.1f | %.1f | %.1f | %.1f | %d | %d |\n",
				escapeMarkdownCell(r.SessionID),
				escapeMarkdownCell(r.Branch),
				r.SystemPct, r.ConvPct, r.ToolDefPct, r.CachePct,
				r.CurrentCtx, r.TotalInput,
			))
		}
		return sb.String(), nil

	default: // table
		const sidWidth = 36
		const branchWidth = 20
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %7s  %5s  %8s  %6s  %7s  %11s\n",
			sidWidth, "SESSION", branchWidth, "BRANCH",
			"SYSTEM%", "CONV%", "TOOLDEF%", "CACHE%", "CTX_TOK", "TOTAL_INPUT"))
		sb.WriteString(fmt.Sprintf("%-*s  %-*s  %7s  %5s  %8s  %6s  %7s  %11s\n",
			sidWidth, strings.Repeat("-", sidWidth),
			branchWidth, strings.Repeat("-", branchWidth),
			strings.Repeat("-", 7), strings.Repeat("-", 5),
			strings.Repeat("-", 8), strings.Repeat("-", 6),
			strings.Repeat("-", 7), strings.Repeat("-", 11)))
		for _, sf := range rows {
			r := toRow(sf)
			sid := r.SessionID
			if len(sid) > sidWidth {
				sid = sid[:sidWidth]
			}
			br := r.Branch
			if len(br) > branchWidth {
				br = br[:branchWidth]
			}
			sb.WriteString(fmt.Sprintf("%-*s  %-*s  %6.1f%%  %4.1f%%  %7.1f%%  %5.1f%%  %7d  %11d\n",
				sidWidth, sid,
				branchWidth, br,
				r.SystemPct, r.ConvPct, r.ToolDefPct, r.CachePct,
				r.CurrentCtx, r.TotalInput,
			))
		}
		return sb.String(), nil
	}
}
