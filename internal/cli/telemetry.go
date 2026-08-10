package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// NewTelemetryCmd returns the `telemetry` parent command and its subcommands.
//
//	backlogit telemetry harvest  -- parse Copilot CLI logs and write telemetry-sessions.jsonl
//	backlogit telemetry list     -- list harvested session summaries
//	backlogit telemetry top      -- show top N servers by token usage
//	backlogit telemetry report   -- generate a formatted telemetry report
func NewTelemetryCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Inspect Copilot CLI token usage and tool telemetry",
		Long: `Inspect Copilot CLI token usage and tool telemetry

Use telemetry harvest to parse logs, telemetry report for machine-readable
session and server summaries, and telemetry top to rank servers by
proportional token attribution.

See https://github.com/softwaresalt/backlogit/blob/main/docs/telemetry-fields.md
for harvested field definitions and SQLite column mappings.`,
	}
	cmd.AddCommand(newTelemetryHarvestCmd(cwd))
	cmd.AddCommand(newTelemetryListCmd(cwd))
	cmd.AddCommand(newTelemetryTopCmd(cwd))
	cmd.AddCommand(newTelemetryReportCmd(cwd))
	cmd.AddCommand(newTelemetryTrendCmd(cwd))
	cmd.AddCommand(newTelemetryBranchCmd(cwd))
	cmd.AddCommand(newTelemetrySchemaCmd())
	return cmd
}

func newTelemetryHarvestCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "harvest",
		Short: "Parse Copilot CLI logs and write telemetry-sessions.jsonl",
		Long: `Parse Copilot CLI logs and write telemetry-sessions.jsonl

Each harvest run performs two writes:

1. Primary output: appends new session_summary and tool_usage JSONL records to
   .backlogit/telemetry-sessions.jsonl. Incremental by default, only sessions
   seen since the last checkpoint are appended. Use --force to rewrite from scratch.

2. SQLite rehydration (side effect): after writing the JSONL, harvest calls
   EnsureTelemetrySchema and RehydrateTelemetry to rebuild the telemetry_sessions
   and telemetry_tool_usage tables in .backlogit/backlogit.db. The tables are
   cleared and repopulated from the full JSONL on every run.

The SQLite tables are ephemeral cache. They can be deleted and will be recreated
on the next telemetry harvest or backlogit sync.

Use backlogit telemetry report or backlogit telemetry top to query the harvested
data after running harvest.

### Checkpoint

A harvest checkpoint is saved to .backlogit/.telemetry-checkpoint.json after each
successful run. The checkpoint records file offsets for each parsed log file so
subsequent runs read only new log entries. Delete the checkpoint or use --force to
reparse all logs from the beginning.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			since, _ := cmd.Flags().GetString("since")
			force, _ := cmd.Flags().GetBool("force")

			opts := telemetry.HarvestOptions{Force: force}
			if since != "" {
				t, err := time.Parse(time.RFC3339, since)
				if err != nil {
					return fmt.Errorf("invalid --since timestamp: %w", err)
				}
				opts.Since = &t
			}

			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if ws.Config != nil && ws.Config.Telemetry != nil {
				opts.AttributionPrefixes = ws.Config.Telemetry.AttributionPrefixes
			}

			copilotPath := ws.RootPath + "/.copilot"
			result, err := telemetry.HarvestTelemetry(ctx, ws.RootPath, copilotPath, ws.DB, opts)
			if err != nil {
				return fmt.Errorf("harvest: %w", err)
			}
			msg := fmt.Sprintf(
				"Harvested %d session(s), %d tool usage row(s), %d total token(s).\n",
				result.SessionsHarvested, result.ToolCallsIndexed, result.TotalTokens)
			if result.FactToolCallsAdded > 0 || result.FactSessionsAdded > 0 {
				msg += fmt.Sprintf("Facts indexed: %d tool call(s), %d session(s).\n",
					result.FactToolCallsAdded, result.FactSessionsAdded)
			}
			fmt.Fprint(cmd.OutOrStdout(), msg)
			return nil
		},
	}
	cmd.Flags().String("since", "", "Exclude events before this RFC3339 timestamp (e.g. 2026-04-01T00:00:00Z)")
	cmd.Flags().Bool("force", false, "Re-process all logs from the beginning, ignoring the saved checkpoint")
	return cmd
}

func newTelemetryListCmd(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List harvested session summaries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out, err := telemetry.GenerateReport(*cwd, telemetry.ReportOptions{
				GroupBy: "session",
				Format:  telemetry.FormatTable,
			})
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
}

func newTelemetryTopCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show top N servers by token usage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n, _ := cmd.Flags().GetInt("n")
			out, err := telemetry.GenerateReport(*cwd, telemetry.ReportOptions{
				GroupBy: "server",
				Format:  telemetry.FormatTable,
				Limit:   n,
			})
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().Int("n", 10, "Number of top entries to display")
	return cmd
}

func newTelemetryReportCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a formatted telemetry report from harvested data",
		RunE: func(cmd *cobra.Command, _ []string) error {
			session, _ := cmd.Flags().GetString("session")
			by, _ := cmd.Flags().GetString("by")
			format, _ := cmd.Flags().GetString("format")
			limit, _ := cmd.Flags().GetInt("limit")

			out, err := telemetry.GenerateReport(*cwd, telemetry.ReportOptions{
				SessionID: session,
				GroupBy:   by,
				Format:    telemetry.ReportFormat(format),
				Limit:     limit,
			})
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().String("session", "", "Filter report to a single session ID")
	cmd.Flags().String("by", "session", "Group output by: session, server, model, class, tool, context")
	cmd.Flags().String("format", "table", "Output format: table, json, markdown")
	cmd.Flags().Int("limit", 0, "Restrict the number of rows returned (0 = no limit)")
	return cmd
}

func newTelemetryTrendCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Show token usage trends grouped by date, branch, or model class",
		Long: `Show token usage trends grouped by date, branch, or model class.

Each output row contains:
  - Group (date YYYY-MM-DD, branch name, or model class)
  - Session count
  - Total tokens
  - Avg tokens per session
  - Avg tokens per task (when available)
  - Avg peak context utilisation (when available)

Use --by branch to switch from date grouping to branch grouping.
Use --by class to group by model class (sonnet, haiku, gpt, o-series, etc.).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			by, _ := cmd.Flags().GetString("by")
			format, _ := cmd.Flags().GetString("format")
			limit, _ := cmd.Flags().GetInt("limit")

			out, err := telemetry.GenerateTrendReport(*cwd, telemetry.TrendOptions{
				By:     by,
				Format: telemetry.ReportFormat(format),
				Limit:  limit,
			})
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().String("by", "date", "Group output by: date, branch, class, branch-type")
	cmd.Flags().String("format", "table", "Output format: table, json, markdown")
	cmd.Flags().Int("limit", 0, "Restrict the number of groups returned (0 = no limit)")
	return cmd
}

func newTelemetryBranchCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "Show per-branch telemetry metrics with type classification and enrichment",
		Long: `Show per-branch telemetry metrics with type classification and enrichment.

Aggregates all sessions by branch name and enriches each branch profile with:
  - Branch type classification (feature, chore, stage, ship, post-merge, main, other)
  - Git PR number (from merge commit history)
  - Backlogit artifact IDs (shipment/feature extracted from branch naming conventions)
  - Lifespan (first to last session)

Use --type to filter by branch type (e.g. --type feature).
Use --format json for machine-readable output.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			format, _ := cmd.Flags().GetString("format")
			typeFilter, _ := cmd.Flags().GetString("type")
			limit, _ := cmd.Flags().GetInt("limit")

			// Validate format early.
			switch telemetry.ReportFormat(format) {
			case telemetry.FormatTable, telemetry.FormatJSON, telemetry.FormatMarkdown:
			default:
				return fmt.Errorf("unsupported format %q: supported values are \"table\", \"json\", \"markdown\"", format)
			}

			jsonlPath := filepath.Join(core.WorkspaceStorageRoot(*cwd), "telemetry-sessions.jsonl")
			sessions, err := telemetry.ReadSessionsOnly(jsonlPath)
			if err != nil {
				return fmt.Errorf("read telemetry data: %w", err)
			}
			if len(sessions) == 0 {
				fmt.Fprint(cmd.OutOrStdout(), "No telemetry data found. Run `backlogit telemetry harvest` first.\n")
				return nil
			}

			profiles := telemetry.AggregateBranches(sessions)

			// Git PR enrichment (graceful failure).
			prMap, _ := telemetry.ParseGitMergePRs(*cwd)
			telemetry.EnrichBranchProfiles(profiles, prMap)

			// Filter by branch type if requested.
			if typeFilter != "" {
				var filtered []telemetry.BranchProfile
				for _, p := range profiles {
					if p.BranchType == typeFilter {
						filtered = append(filtered, p)
					}
				}
				profiles = filtered
			}

			// Apply limit.
			if limit > 0 && len(profiles) > limit {
				profiles = profiles[:limit]
			}

			switch telemetry.ReportFormat(format) {
			case telemetry.FormatJSON:
				return renderBranchJSON(cmd, profiles)
			case telemetry.FormatMarkdown:
				renderBranchMarkdown(cmd, profiles)
			default:
				renderBranchTable(cmd, profiles)
			}
			return nil
		},
	}
	cmd.Flags().String("format", "table", "Output format: table, json, markdown")
	cmd.Flags().String("type", "", "Filter by branch type: feature, chore, stage, ship, post-merge, main, other")
	cmd.Flags().Int("limit", 0, "Restrict the number of branches returned (0 = no limit)")
	return cmd
}

func renderBranchTable(cmd *cobra.Command, profiles []telemetry.BranchProfile) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%-40s %-12s %8s %12s %14s %5s %-6s %-8s %-8s %s\n",
		"BRANCH", "TYPE", "SESSIONS", "TOKENS", "AVG/SESSION", "TASKS", "PR", "FEATURE", "SHIPMENT", "LIFESPAN")
	fmt.Fprintf(w, "%-40s %-12s %8s %12s %14s %5s %-6s %-8s %-8s %s\n",
		"----------------------------------------", "------------", "--------", "------------",
		"--------------", "-----", "------", "--------", "--------", "--------")
	for _, p := range profiles {
		branch := p.Branch
		if len(branch) > 40 {
			branch = branch[:37] + "..."
		}
		pr := p.PRNumber
		if pr == "" {
			pr = "-"
		}
		feat := p.FeatureID
		if feat == "" {
			feat = "-"
		}
		ship := p.ShipmentID
		if ship == "" {
			ship = "-"
		}
		lifespan := formatLifespan(p.LastSeen.Sub(p.FirstSeen))
		fmt.Fprintf(w, "%-40s %-12s %8d %12d %14.0f %5d %-6s %-8s %-8s %s\n",
			branch, p.BranchType, p.Sessions, p.TotalTokens, p.AvgTokens,
			p.TaskCount, pr, feat, ship, lifespan)
	}
}

func renderBranchJSON(cmd *cobra.Command, profiles []telemetry.BranchProfile) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(profiles)
}

func renderBranchMarkdown(cmd *cobra.Command, profiles []telemetry.BranchProfile) {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "## Branch Telemetry Metrics")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "| Branch | Type | Sessions | Tokens | Avg/Session | Tasks | PR | Feature | Shipment | Lifespan |")
	fmt.Fprintln(w, "|---|---|---|---|---|---|---|---|---|---|")
	for _, p := range profiles {
		pr := p.PRNumber
		if pr == "" {
			pr = "-"
		}
		feat := p.FeatureID
		if feat == "" {
			feat = "-"
		}
		ship := p.ShipmentID
		if ship == "" {
			ship = "-"
		}
		lifespan := formatLifespan(p.LastSeen.Sub(p.FirstSeen))
		fmt.Fprintf(w, "| %s | %s | %d | %d | %.0f | %d | %s | %s | %s | %s |\n",
			escapeMDCell(p.Branch), escapeMDCell(p.BranchType), p.Sessions, p.TotalTokens, p.AvgTokens,
			p.TaskCount, pr, feat, ship, lifespan)
	}
}

// escapeMDCell replaces pipe characters that would break Markdown table cells.
func escapeMDCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func formatLifespan(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	return fmt.Sprintf("%dd%dh", days, hours)
}

func newTelemetrySchemaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Show telemetry JSONL and SQL table schemas",
		Long: `Show telemetry JSONL fact table and SQL table schemas.

Lists every field in the telemetry fact tables (session_summary, tool_usage,
tool_call_fact, session_fact) and the SQLite cache tables (telemetry_sessions,
telemetry_tool_usage). Useful for agents and operators building queries
against harvested telemetry data.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			formatFlag, _ := cmd.Flags().GetString("format")

			factTables := telemetry.DescribeFactTables()
			sqlTables := telemetry.DescribeTelemetrySQLTables()

			switch formatFlag {
			case "json":
				return renderSchemaJSON(cmd, factTables, sqlTables)
			case "markdown":
				return renderSchemaMarkdown(cmd, factTables, sqlTables)
			default:
				return renderSchemaText(cmd, factTables, sqlTables)
			}
		},
	}
	cmd.Flags().String("format", "text", "Output format: text, json, markdown")
	return cmd
}

func renderSchemaText(cmd *cobra.Command, factTables, sqlTables []telemetry.FactTableSchema) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "JSONL Fact Tables")
	fmt.Fprintln(w, "=================")
	for _, tbl := range factTables {
		fmt.Fprintf(w, "\n%s (record_type: %s, file: %s)\n", tbl.Name, tbl.RecordType, tbl.File)
		fmt.Fprintf(w, "  %-30s %-30s %s\n", "FIELD", "TYPE", "JSON KEY")
		for _, f := range tbl.Fields {
			opt := ""
			if f.Optional {
				opt = " (optional)"
			}
			fmt.Fprintf(w, "  %-30s %-30s %s%s\n", f.Name, f.Type, f.JSONKey, opt)
		}
	}
	fmt.Fprintln(w, "\nSQL Tables")
	fmt.Fprintln(w, "==========")
	for _, tbl := range sqlTables {
		fmt.Fprintf(w, "\n%s (db: %s)\n", tbl.Name, tbl.File)
		fmt.Fprintf(w, "  %-30s %-10s %s\n", "COLUMN", "TYPE", "JSON KEY")
		for _, f := range tbl.Fields {
			fmt.Fprintf(w, "  %-30s %-10s %s\n", f.Name, f.Type, f.JSONKey)
		}
	}
	return nil
}

func renderSchemaJSON(cmd *cobra.Command, factTables, sqlTables []telemetry.FactTableSchema) error {
	out := struct {
		FactTables []telemetry.FactTableSchema `json:"fact_tables"`
		SQLTables  []telemetry.FactTableSchema `json:"sql_tables"`
	}{
		FactTables: factTables,
		SQLTables:  sqlTables,
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func renderSchemaMarkdown(cmd *cobra.Command, factTables, sqlTables []telemetry.FactTableSchema) error {
	w := cmd.OutOrStdout()
	fmt.Fprintln(w, "## JSONL Fact Tables")
	for _, tbl := range factTables {
		fmt.Fprintf(w, "\n### %s\n\n", tbl.Name)
		fmt.Fprintf(w, "File: `%s` | Record type: `%s`\n\n", tbl.File, tbl.RecordType)
		fmt.Fprintln(w, "| Field | Type | JSON Key | Optional |")
		fmt.Fprintln(w, "|---|---|---|---|")
		for _, f := range tbl.Fields {
			opt := ""
			if f.Optional {
				opt = "yes"
			}
			fmt.Fprintf(w, "| %s | %s | %s | %s |\n", f.Name, f.Type, f.JSONKey, opt)
		}
	}
	fmt.Fprintln(w, "\n## SQL Tables")
	for _, tbl := range sqlTables {
		fmt.Fprintf(w, "\n### %s\n\n", tbl.Name)
		fmt.Fprintf(w, "Database: `%s`\n\n", tbl.File)
		fmt.Fprintln(w, "| Column | Type | JSON Key |")
		fmt.Fprintln(w, "|---|---|---|")
		for _, f := range tbl.Fields {
			fmt.Fprintf(w, "| %s | %s | %s |\n", f.Name, f.Type, f.JSONKey)
		}
	}
	return nil
}
