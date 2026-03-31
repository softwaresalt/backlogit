package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
	"github.com/backlogit/backlogit/internal/parser"
)

// newUpdateCommand creates the `backlogit update` command.
func newUpdateCommand(cwd *string) *cobra.Command {
	var (
		title    string
		status   string
		priority string
		idFlag   string
		sections []string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update artifact fields or sections",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if idFlag != "" {
				return fmt.Errorf("field \"id\" is immutable and cannot be changed")
			}

			id := args[0]
			ctx := context.Background()
			ws, err := core.NewWorkspace(ctx, *cwd)
			if err != nil {
				return fmt.Errorf("open workspace: %w", err)
			}
			defer ws.Close()

			// Build frontmatter updates map.
			updates := map[string]any{}
			if cmd.Flags().Changed("title") {
				updates["title"] = title
			}
			if cmd.Flags().Changed("status") {
				updates["status"] = status
			}
			if cmd.Flags().Changed("priority") {
				updates["priority"] = priority
			}

			// Parse section updates: name=value pairs.
			sectionUpdates := map[string]string{}
			for _, sec := range sections {
				name, value, found := strings.Cut(sec, "=")
				if !found {
					return fmt.Errorf("invalid --section format %q: expected name=value", sec)
				}
				sectionUpdates[name] = value
			}

			if len(updates) == 0 && len(sectionUpdates) == 0 {
				return fmt.Errorf("no updates specified")
			}

			// Find the artifact file.
			filePath, err := core.FindArtifactPath(ctx, ws, id)
			if err != nil {
				return err
			}

			// Apply frontmatter updates if any.
			if len(updates) > 0 {
				artifact, updateErr := core.UpdateArtifact(ctx, ws, id, updates)
				if updateErr != nil {
					return updateErr
				}
				if writeErr := core.WriteArtifactFile(artifact, filePath); writeErr != nil {
					return writeErr
				}
				if upsertErr := db.UpsertItem(ctx, ws.DB, artifact); upsertErr != nil {
					return upsertErr
				}
			}

			// Apply section updates if any.
			if len(sectionUpdates) > 0 {
				raw, readErr := os.ReadFile(filePath)
				if readErr != nil {
					return fmt.Errorf("read artifact file: %w", readErr)
				}
				fm, body, parseErr := models.ParseFrontmatter(string(raw))
				if parseErr != nil {
					return parseErr
				}
				newBody, writeErr := parser.WriteSections(body, sectionUpdates)
				if writeErr != nil {
					// Section not found: append new sections.
					for name, value := range sectionUpdates {
						body += "\n\n<!-- BEGIN:" + name + " -->\n" + value + "\n<!-- END:" + name + " -->"
					}
					newBody = body
				}
				newContent := models.SerializeFrontmatter(fm, newBody)
				tmp := filePath + ".tmp"
				if writeErr2 := os.WriteFile(tmp, []byte(newContent), 0o644); writeErr2 != nil {
					return fmt.Errorf("write artifact: %w", writeErr2)
				}
				if renameErr := os.Rename(tmp, filePath); renameErr != nil {
					os.Remove(tmp) //nolint:errcheck
					return fmt.Errorf("rename artifact: %w", renameErr)
				}
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&status, "status", "", "new status")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority")
	cmd.Flags().StringVar(&idFlag, "id", "", "artifact ID (immutable, always rejected)")
	cmd.Flags().StringArrayVar(&sections, "section", nil, "section update as name=value (repeatable)")
	return cmd
}
