package cli

import (
	"context"
	"encoding/json"
	"errors"
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
	var artifact string
	var force bool

	cmd := &cobra.Command{
		Use:   "correct [stash-id]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Record a stash provenance correction",
		Long: `Record that a stash entry's canonical actual delivery artifact differs from the
historically auto-harvested artifact. Preserves the original harvested_artifact_id and
appends an append-only correction record to provenance_corrections.jsonl.`,
		Example: `  backlogit stash correct --stash-id 11FFF601 --canonical-delivery 150-F \
    --reason "Actual delivery was 150-F/133-S, not auto-harvested 151-F" --actor "ship-agent"`,
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			ctx := context.Background()
			if len(args) == 1 {
				if stashID != "" && stashID != args[0] {
					return fmt.Errorf("stash correct: positional stash-id %q conflicts with --stash-id %q", args[0], stashID)
				}
				stashID = args[0]
			}
			if artifact != "" {
				if canonicalDelivery != "" && canonicalDelivery != artifact {
					return fmt.Errorf("stash correct: --artifact %q conflicts with --canonical-delivery %q", artifact, canonicalDelivery)
				}
				canonicalDelivery = artifact
			}
			if stashID == "" {
				return errors.New("stash correct: stash ID is required (positional argument or --stash-id)")
			}
			if canonicalDelivery == "" {
				return errors.New("stash correct: --canonical-delivery (or --artifact) is required")
			}
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer func() {
				if cerr := ws.Close(); cerr != nil {
					err = errors.Join(err, fmt.Errorf("close workspace: %w", cerr))
				}
			}()

			req := core.StashProvenanceCorrectionRequest{
				StashID:                     stashID,
				CanonicalDeliveryArtifactID: canonicalDelivery,
				Reason:                      reason,
				Actor:                       actor,
				Force:                       force,
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
	cmd.Flags().StringVar(&stashID, "stash-id", "", "Stash entry ID to correct (or pass as positional argument)")
	cmd.Flags().StringVar(&canonicalDelivery, "canonical-delivery", "", "Canonical delivery artifact ID (required unless --artifact is used)")
	cmd.Flags().StringVar(&artifact, "artifact", "", "Alias for --canonical-delivery")
	cmd.Flags().BoolVar(&force, "force", false, "Stamp source_stash_id on the target artifact and override conflicting provenance")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for the correction (required)")
	cmd.Flags().StringVar(&actor, "actor", "", "Actor performing the correction (required)")
	_ = cmd.MarkFlagRequired("reason")
	_ = cmd.MarkFlagRequired("actor")
	return cmd
}
