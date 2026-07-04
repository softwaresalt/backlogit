package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
)

// newHooksCmd returns the `hooks` command group. It is the CLI fallback for the
// MCP poll_hook_events / ack_hook_events tools and reuses the same durable hook
// queue (.backlogit/hooks_queue.jsonl) and per-consumer checkpoints, so consumer
// state is shared across surfaces.
func newHooksCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Poll and acknowledge agent hook events",
		Long: `Poll for unacknowledged hook events and acknowledge processed events.

These commands are the CLI fallback for the MCP poll_hook_events/ack_hook_events
tools. They reuse the same durable queue at .backlogit/hooks_queue.jsonl and the
per-consumer checkpoints, so consumer progress is shared across surfaces.`,
	}
	cmd.AddCommand(newHooksPollCmd(cwd))
	cmd.AddCommand(newHooksAckCmd(cwd))
	return cmd
}

func newHooksPollCmd(cwd *string) *cobra.Command {
	var consumerID string
	cmd := &cobra.Command{
		Use:     "poll",
		Short:   "Poll for unacknowledged hook events since the consumer checkpoint",
		Example: `  backlogit hooks poll --consumer-id ship`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			if consumerID == "" {
				return fmt.Errorf("consumer-id is required")
			}
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			backlogitDir := filepath.Join(ws.RootPath, ".backlogit")
			hw := events.NewHookEventWriter(backlogitDir)
			cs := events.NewCheckpointStore(backlogitDir)
			result, err := events.PollHookEvents(ctx, hw, cs, consumerID, nil)
			if err != nil {
				return fmt.Errorf("poll hook events: %w", err)
			}

			// Normalize nil slices to empty arrays so collection fields marshal as
			// [] rather than null (Rule 3), matching handlePollHookEvents.
			evs := result.Events
			if evs == nil {
				evs = []events.HookEvent{}
			}
			derived := result.DerivedSignals
			if derived == nil {
				derived = []events.HookEvent{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"events":          evs,
				"derived_signals": derived,
			})
		},
	}
	cmd.Flags().StringVar(&consumerID, "consumer-id", "", "agent consumer ID (e.g. ship, stage)")
	_ = cmd.MarkFlagRequired("consumer-id")
	return cmd
}

func newHooksAckCmd(cwd *string) *cobra.Command {
	var consumerID string
	var seq int64
	cmd := &cobra.Command{
		Use:     "ack",
		Short:   "Acknowledge processed hook events up to and including --seq",
		Example: `  backlogit hooks ack --consumer-id ship --seq 12`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			if consumerID == "" {
				return fmt.Errorf("consumer-id is required")
			}
			if seq < 1 {
				return fmt.Errorf("seq must be >= 1")
			}
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			backlogitDir := filepath.Join(ws.RootPath, ".backlogit")
			cs := events.NewCheckpointStore(backlogitDir)
			if err := events.AckHookEvents(ctx, cs, consumerID, seq); err != nil {
				return fmt.Errorf("ack hook events: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{"acked_seq": seq})
		},
	}
	cmd.Flags().StringVar(&consumerID, "consumer-id", "", "agent consumer ID")
	cmd.Flags().Int64Var(&seq, "seq", 0, "highest sequence number processed")
	_ = cmd.MarkFlagRequired("consumer-id")
	_ = cmd.MarkFlagRequired("seq")
	return cmd
}
