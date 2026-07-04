package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
)

// newLinkCmd returns the `link` command group. It is the CLI fallback for the
// MCP add_link / remove_link / get_links tools and reuses the same shared core
// path (core.AddArtifactLink / core.RemoveArtifactLink / core.GetLinks) so the
// behavior and success-JSON shapes are identical across surfaces.
func newLinkCmd(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link",
		Short: "Manage directed semantic links between artifacts",
		Long: `Create, remove, and list directed semantic links between artifacts.

These commands are the CLI fallback for the MCP add_link/remove_link/get_links
tools. They reuse the same shared core path so behavior is identical across
surfaces; success output is JSON isomorphic to the MCP tool results.`,
	}
	cmd.AddCommand(newLinkAddCmd(cwd))
	cmd.AddCommand(newLinkRemoveCmd(cwd))
	cmd.AddCommand(newLinkListCmd(cwd))
	return cmd
}

func newLinkAddCmd(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:     "add <source> <target> <type>",
		Short:   "Create a directed semantic link from source to target",
		Example: `  backlogit link add 001-F 002-F related_to`,
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			sourceID, targetID, linkType := args[0], args[1], args[2]
			if err := core.AddArtifactLink(ctx, ws, sourceID, targetID, linkType); err != nil {
				return fmt.Errorf("add link: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{
				"source_id": sourceID,
				"target_id": targetID,
				"link_type": linkType,
			})
		},
	}
}

func newLinkRemoveCmd(cwd *string) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <source> <target> <type>",
		Short:   "Remove a directed semantic link",
		Example: `  backlogit link remove 001-F 002-F related_to`,
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			sourceID, targetID, linkType := args[0], args[1], args[2]
			if err := core.RemoveArtifactLink(ctx, ws, sourceID, targetID, linkType); err != nil {
				return fmt.Errorf("remove link: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{
				"source_id": sourceID,
				"target_id": targetID,
				"link_type": linkType,
				"status":    "removed",
			})
		},
	}
}

func newLinkListCmd(cwd *string) *cobra.Command {
	var linkType string
	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List outgoing semantic links for an artifact",
		Example: `  backlogit link list 001-F
  backlogit link list 001-F --type related_to`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			id := args[0]
			// core.GetLinks normalizes a nil result to an empty slice so the
			// "links" field always marshals as [] rather than null (Rule 3).
			edges, err := core.GetLinks(ctx, ws, id, linkType)
			if err != nil {
				return fmt.Errorf("list links: %w", err)
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]any{
				"id":    id,
				"links": edges,
			})
		},
	}
	cmd.Flags().StringVar(&linkType, "type", "", "filter to a single link_type (optional)")
	return cmd
}
