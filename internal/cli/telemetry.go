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
func NewTelemetryCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Inspect Copilot CLI token usage and tool telemetry",
	}
	cmd.AddCommand(newTelemetryHarvestCmd(cwd))
	cmd.AddCommand(newTelemetryListCmd(cwd))
	cmd.AddCommand(newTelemetryTopCmd(cwd))
	return cmd
}

func newTelemetryHarvestCmd(_ *string) *cobra.Command {
	return &cobra.Command{
		Use:   "harvest",
		Short: "Parse Copilot CLI logs and write telemetry-sessions.jsonl",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("telemetry harvest: not yet implemented as CLI; use MCP tool backlogit_telemetry_harvest instead")
		},
	}
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
	return &cobra.Command{
		Use:   "top",
		Short: "Show top N tool calls by token usage",
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("telemetry top: not yet implemented")
		},
	}
}
