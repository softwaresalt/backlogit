package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/softwaresalt/backlogit/internal/parser"
)

// newUpdateCommand creates the `backlogit update` command.
func newUpdateCommand(cwd *string) *cobra.Command {
	var (
		title         string
		status        string
		priority      string
		idFlag        string
		sections      []string
		harnessStatus string
		description   string
		sprint        string
		assignedTo    string
		owner         string
		labels        string
		commit        string
	)

	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update artifact fields or sections",
		Long: `Update frontmatter fields or template-backed body sections on an existing
artifact.

Use repeated --section name=value flags to update named sections without
replacing the rest of the document body.`,
		Example: `  backlogit update 001.001-T --status review
  backlogit update 001.001-T --priority high
  backlogit update 001-F --section goals="Ship passwordless sign-in"
  backlogit update 001-F --harness-status passing`,
		Args: cobra.ExactArgs(1),
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
			if cmd.Flags().Changed("harness-status") {
				updates["harness_status"] = harnessStatus
			}
			if cmd.Flags().Changed("description") {
				updates["description"] = description
			}
			if cmd.Flags().Changed("sprint") {
				updates["sprint"] = sprint
			}
			if cmd.Flags().Changed("assigned-to") {
				updates["assigned_to"] = assignedTo
			}
			if cmd.Flags().Changed("owner") {
				updates["owner"] = owner
			}
			if cmd.Flags().Changed("labels") && labels != "" {
				updates["labels"] = splitCSV(labels)
			}
			if cmd.Flags().Changed("commit") {
				updates["commit"] = commit
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

			// Apply frontmatter updates if any.
			if len(updates) > 0 {
				_, updateErr := core.UpdateArtifact(ctx, ws, id, updates)
				if updateErr != nil {
					return updateErr
				}
			}

			// Apply section updates if any.
			if len(sectionUpdates) > 0 {
				// Resolve the file path after any frontmatter-driven relocation
				// (e.g., a status change moving the file from queue/ to archive/).
				filePath, err := core.FindArtifactPath(ctx, ws, id)
				if err != nil {
					return err
				}
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

				// Bump updated_at so callers can detect the change.
				now := time.Now()
				fm["updated_at"] = now

				newContent := models.SerializeFrontmatter(fm, newBody)
				tmp := filePath + ".tmp"
				if writeErr2 := os.WriteFile(tmp, []byte(newContent), 0o644); writeErr2 != nil {
					return fmt.Errorf("write artifact: %w", writeErr2)
				}
				if renameErr := os.Rename(tmp, filePath); renameErr != nil {
					os.Remove(tmp) //nolint:errcheck
					return fmt.Errorf("rename artifact: %w", renameErr)
				}

				// Sync the updated artifact into the DB index.
				sectionArtifact, parseArtErr := models.ArtifactFromFrontmatter(fm, newBody)
				if parseArtErr != nil {
					return fmt.Errorf("parse artifact after section write: %w", parseArtErr)
				}
				sectionArtifact.UpdatedAt = now
				if upsertErr := db.UpsertItem(ctx, ws.DB, sectionArtifact); upsertErr != nil {
					return fmt.Errorf("sync index after section write: %w", upsertErr)
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
	cmd.Flags().StringVar(&harnessStatus, "harness-status", "", "harness status (pending, scaffolded, passing, failing)")
	cmd.Flags().StringVar(&description, "description", "", "new description")
	cmd.Flags().StringVar(&sprint, "sprint", "", "sprint ID")
	cmd.Flags().StringVar(&assignedTo, "assigned-to", "", "assignee")
	cmd.Flags().StringVar(&owner, "owner", "", "owner")
	cmd.Flags().StringVar(&labels, "labels", "", "comma-separated labels")
	cmd.Flags().StringVar(&commit, "commit", "", "commit SHA")
	return cmd
}
