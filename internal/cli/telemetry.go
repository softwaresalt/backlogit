package cli

import (
	"context"
	"fmt"
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
			fmt.Fprintf(cmd.OutOrStdout(),
				"Harvested %d session(s), %d tool usage row(s), %d total token(s).\n",
				result.SessionsHarvested, result.ToolCallsIndexed, result.TotalTokens)
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
	cmd.Flags().String("by", "session", "Group output by: session, server, model, class")
	cmd.Flags().String("format", "table", "Output format: table, json, markdown")
	cmd.Flags().Int("limit", 0, "Restrict the number of rows returned (0 = no limit)")
	return cmd
}

func newTelemetryTrendCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trend",
		Short: "Show token usage trends grouped by date or branch",
		Long: `Show token usage trends grouped by date or branch.

Each output row contains:
  - Group (date YYYY-MM-DD or branch name)
  - Session count
  - Total tokens
  - Avg tokens per session
  - Avg tokens per task (when available)
  - Avg peak context utilisation (when available)

Use --by branch to switch from date grouping to branch grouping.`,
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
	cmd.Flags().String("by", "date", "Group output by: date, branch")
	cmd.Flags().String("format", "table", "Output format: table, json, markdown")
	cmd.Flags().Int("limit", 0, "Restrict the number of groups returned (0 = no limit)")
	return cmd
}
