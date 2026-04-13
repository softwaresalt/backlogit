package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/telemetry"
)

// NewTelemetryCmd returns the `telemetry` parent command and its subcommands.
//
//	backlogit telemetry harvest  -- parse Copilot CLI logs and write telemetry-sessions.jsonl
//	backlogit telemetry list     -- list harvested session summaries
//	backlogit telemetry top      -- show top N tool calls by token usage
//	backlogit telemetry report   -- generate a formatted telemetry report
func NewTelemetryCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Inspect Copilot CLI token usage and tool telemetry",
	}
	cmd.AddCommand(newTelemetryHarvestCmd(cwd))
	cmd.AddCommand(newTelemetryListCmd(cwd))
	cmd.AddCommand(newTelemetryTopCmd(cwd))
	cmd.AddCommand(newTelemetryReportCmd(cwd))
	return cmd
}

func newTelemetryHarvestCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "harvest",
		Short: "Parse Copilot CLI logs and write telemetry-sessions.jsonl",
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
				Format:  "table",
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
		Short: "Show top N tool calls by token usage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			n, _ := cmd.Flags().GetInt("n")
			out, err := telemetry.GenerateReport(*cwd, telemetry.ReportOptions{
				GroupBy: "server",
				Format:  "table",
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
				Format:    format,
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
	cmd.Flags().String("by", "session", "Group output by: session, server")
	cmd.Flags().String("format", "table", "Output format: table, json")
	cmd.Flags().Int("limit", 0, "Restrict the number of rows returned (0 = no limit)")
	return cmd
}
