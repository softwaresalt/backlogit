package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// newCommentCmd returns the `comment` command group. It is the CLI fallback for
// the MCP append_comment tool and reuses the same shared core path
// (core.AppendComment) so the persisted and indexed comment event — and the
// success-JSON shape — are identical across surfaces.
func newCommentCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "comment",
		Short: "Append comments to an artifact's history",
		Long: `Append a comment event to an artifact's JSONL log and index.

This is the CLI fallback for the MCP append_comment tool. It reuses the same
shared core path (core.AppendComment) so the persisted and indexed comment event
is identical across surfaces; success output is JSON isomorphic to the MCP tool
result.`,
	}
	cmd.AddCommand(newCommentAddCmd(cwd))
	return cmd
}

func newCommentAddCmd(cwd *string) *cobra.Command {
	var actor, comment, commitSHA string
	cmd := &cobra.Command{
		Use:     "add <item-id>",
		Short:   "Append a comment to an artifact",
		Example: `  backlogit comment add 001-F --actor ship --comment "built U4"`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			itemID := args[0]
			if itemID == "" {
				return fmt.Errorf("item-id is required")
			}

			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			if err := core.AppendComment(ctx, ws, itemID, actor, comment, commitSHA); err != nil {
				return fmt.Errorf("append comment: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]bool{"ok": true})
		},
	}
	cmd.Flags().StringVar(&actor, "actor", "", "actor recording the comment")
	cmd.Flags().StringVar(&comment, "comment", "", "comment text")
	cmd.Flags().StringVar(&commitSHA, "commit-sha", "", "associated commit SHA (optional)")
	_ = cmd.MarkFlagRequired("actor")
	_ = cmd.MarkFlagRequired("comment")
	return cmd
}
