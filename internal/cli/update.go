package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
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
		size          string
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

			// --size is a single-purpose, body-preserving mutation routed through
			// core.SetArtifactSize. It is MUTUALLY EXCLUSIVE with every other
			// frontmatter-mutating flag (and --section), because those route through
			// the generic UpdateArtifact -> WriteArtifactFile rebuild path; running
			// both in one invocation would double-write and negate body
			// preservation. The exclusion is checked BEFORE any write.
			if cmd.Flags().Changed("size") {
				if conflicts := conflictingSizeFlags(cmd); len(conflicts) > 0 {
					return fmt.Errorf(
						"--size cannot be combined with %s: run the size mutation separately to preserve the body",
						strings.Join(conflicts, ", "))
				}
				if _, sizeErr := core.SetArtifactSize(ctx, ws, id, size); sizeErr != nil {
					// A busy task lock surfaces the same non-zero exit code as the
					// doctor --target table (4) instead of blocking, so the
					// autoharness sizing hook sees deterministic contention.
					if errors.Is(sizeErr, core.ErrTaskBusy) {
						cmd.SilenceErrors = true
						return &ExitError{Code: 4, Msg: fmt.Sprintf("task %s is busy: %v", id, sizeErr)}
					}
					return sizeErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", id)
				return nil
			}

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
				// Apply each section update independently so that a missing
				// section is appended without re-appending sections that already
				// exist (which would duplicate them), and a malformed section
				// (BEGIN with no matching END) surfaces an error instead of being
				// masked by a blind append. Sort for deterministic output.
				sectionNames := make([]string, 0, len(sectionUpdates))
				for name := range sectionUpdates {
					sectionNames = append(sectionNames, name)
				}
				sort.Strings(sectionNames)
				// Reject names that would produce unparseable markers before any
				// write, matching the MCP path so neither surface can persist a
				// corrupt section.
				for _, name := range sectionNames {
					if nameErr := parser.ValidateSectionName(name); nameErr != nil {
						return nameErr
					}
				}
				newBody := body
				for _, name := range sectionNames {
					value := sectionUpdates[name]
					updated, writeErr := parser.WriteSection(newBody, name, value)
					if writeErr != nil {
						// A genuinely absent section is appended; any other error
						// (malformed markers or otherwise) is surfaced so the write
						// never silently duplicates or masks corruption.
						if errors.Is(writeErr, parser.ErrSectionNotFound) {
							newBody += "\n\n<!-- BEGIN:" + name + " -->\n" + value + "\n<!-- END:" + name + " -->"
							continue
						}
						return fmt.Errorf("update section %q: %w", name, writeErr)
					}
					newBody = updated
				}

				// Bump updated_at so callers can detect the change.
				now := time.Now()
				fm["updated_at"] = now

				newContent := models.SerializeFrontmatter(fm, newBody)
				tmp := filePath + ".tmp"
				if writeErr2 := os.WriteFile(tmp, []byte(newContent), 0o644); writeErr2 != nil {
					os.Remove(tmp) //nolint:errcheck
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
	cmd.Flags().StringVar(&size, "size", "", "T-shirt size (XS, S, M, L, XL); body-preserving, mutually exclusive with other field flags")
	return cmd
}

// conflictingSizeFlags returns the set of frontmatter-mutating flags (rendered as
// --name) that were set alongside --size. --size is single-purpose, so any of
// these makes the invocation ambiguous and must error before any write.
func conflictingSizeFlags(cmd *cobra.Command) []string {
	candidates := []string{
		"title", "status", "priority", "harness-status", "description",
		"sprint", "assigned-to", "owner", "labels", "commit", "section",
	}
	var conflicts []string
	for _, name := range candidates {
		if cmd.Flags().Changed(name) {
			conflicts = append(conflicts, "--"+name)
		}
	}
	return conflicts
}
