package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/models"
	"github.com/backlogit/backlogit/internal/parser"
)

// newGetCommand creates the `backlogit get` command.
func newGetCommand(cwd *string) *cobra.Command {
	var (
		jsonOutput bool
		section    string
	)

	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Retrieve an artifact by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			filePath, err := core.FindArtifactPath(ctx, ws, id)
			if err != nil {
				return err
			}

			raw, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read artifact file: %w", err)
			}

			fm, body, err := models.ParseFrontmatter(string(raw))
			if err != nil {
				return fmt.Errorf("parse artifact: %w", err)
			}

			if section != "" {
				sections, parseErr := parser.ParseSections(body)
				if parseErr != nil {
					return parseErr
				}
				content, ok := sections[section]
				if !ok {
					return fmt.Errorf("section %q not found in artifact %s", section, id)
				}
				fmt.Fprintln(cmd.OutOrStdout(), content)
				return nil
			}

			if jsonOutput {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(fm)
			}

			fmt.Fprint(cmd.OutOrStdout(), string(raw))
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output frontmatter as JSON")
	cmd.Flags().StringVar(&section, "section", "", "extract a named section from the body")
	return cmd
}
