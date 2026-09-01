package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// newStashCorrectCommand returns the `backlogit stash correct` subcommand.
// It records that a stash entry's canonical actual delivery artifact differs
// from the historically auto-harvested artifact, preserving the original
// harvested_artifact_id and appending an append-only correction record.
func newStashCorrectCommand(cwd *string) *cobra.Command {
	var stashID string
	var canonicalDelivery string
	var reason string
	var actor string

	cmd := &cobra.Command{
		Use:   "correct",
		Short: "Record a stash provenance correction",
		Long: `Record that a stash entry's canonical actual delivery artifact differs from the
historically auto-harvested artifact. Preserves the original harvested_artifact_id and
appends an append-only correction record to provenance_corrections.jsonl.`,
		Example: `  backlogit stash correct --stash-id 11FFF601 --canonical-delivery 150-F \
    --reason "Actual delivery was 150-F/133-S, not auto-harvested 151-F" --actor "ship-agent"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			req := core.StashProvenanceCorrectionRequest{
				StashID:                     stashID,
				CanonicalDeliveryArtifactID: canonicalDelivery,
				Reason:                      reason,
				Actor:                       actor,
			}
			result, err := core.CorrectStashProvenance(ctx, ws, req)
			if err != nil {
				return fmt.Errorf("stash correct: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&stashID, "stash-id", "", "Stash entry ID to correct (required)")
	cmd.Flags().StringVar(&canonicalDelivery, "canonical-delivery", "", "Canonical delivery artifact ID (required)")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for the correction (required)")
	cmd.Flags().StringVar(&actor, "actor", "", "Actor performing the correction (required)")
	_ = cmd.MarkFlagRequired("stash-id")
	_ = cmd.MarkFlagRequired("canonical-delivery")
	_ = cmd.MarkFlagRequired("reason")
	_ = cmd.MarkFlagRequired("actor")
	return cmd
}
