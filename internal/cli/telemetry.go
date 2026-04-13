package cli

import (
	"fmt"

	"github.com/spf13/cobra"
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

func newTelemetryHarvestCmd(_ *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "harvest",
		Short: "Parse Copilot CLI logs and write telemetry-sessions.jsonl",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("telemetry harvest: not yet implemented as CLI; use MCP tool backlogit_telemetry_harvest instead")
		},
	}
	cmd.Flags().String("since", "", "Exclude events before this RFC3339 timestamp (e.g. 2026-04-01T00:00:00Z)")
	cmd.Flags().Bool("force", false, "Re-process all logs from the beginning, ignoring the saved checkpoint")
	return cmd
}

func newTelemetryListCmd(_ *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List harvested session summaries",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("telemetry list: not yet implemented")
		},
	}
}

func newTelemetryTopCmd(_ *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "top",
		Short: "Show top N tool calls by token usage",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("telemetry top: not yet implemented")
		},
	}
	cmd.Flags().Int("n", 10, "Number of top entries to display")
	return cmd
}

func newTelemetryReportCmd(_ *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a formatted telemetry report from harvested data",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("telemetry report: not yet implemented as CLI; use MCP tool backlogit_generate_report instead")
		},
	}
	cmd.Flags().String("session", "", "Filter report to a single session ID")
	cmd.Flags().String("by", "session", "Group output by: session, server, model, tool")
	cmd.Flags().String("format", "table", "Output format: table, json")
	cmd.Flags().Int("limit", 0, "Restrict the number of rows returned (0 = no limit)")
	return cmd
}
