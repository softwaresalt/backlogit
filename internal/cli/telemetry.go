package cli

import (
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			panic("not implemented: telemetry harvest")
		},
	}
}

func newTelemetryListCmd(_ *string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List harvested session summaries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			panic("not implemented: telemetry list")
		},
	}
}

func newTelemetryTopCmd(_ *string) *cobra.Command {
	return &cobra.Command{
		Use:   "top",
		Short: "Show top N tool calls by token usage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			panic("not implemented: telemetry top")
		},
	}
}
