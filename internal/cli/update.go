package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

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
		complexity    string
		sizeSource    string
		sizeRuleset   string
		gateBase      string
		forceGates    bool
		forceReason   string
		jsonOut       bool
		checkSelf     bool
		targetTag     string
	)

	cmd := &cobra.Command{
		Use:   "update [id]",
		Short: "Self-update backlogit or update artifact fields",
		Long: `Update the installed backlogit binary when called without an artifact ID,
or update frontmatter fields and template-backed body sections on an existing
artifact when an ID is supplied.

Use repeated --section name=value flags to update named sections without
replacing the rest of the document body.

Complexity is task-only planning metadata:
size = implementation volume; complexity = implementation difficulty and uncertainty;
priority = urgency. Default queue ordering does not change
when complexity is set.`,
		Example: `  backlogit update
  backlogit update --check
  backlogit update --to v1.2.3
  backlogit update 001.001-T --status review
  backlogit update 001.001-T --priority high
  backlogit update 001-F --section goals="Ship passwordless sign-in"
  backlogit update 001-F --harness-status passing`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if idFlag != "" {
				return fmt.Errorf("field \"id\" is immutable and cannot be changed")
			}
			if len(args) == 0 {
				if conflicts := artifactUpdateFlagsChanged(cmd); len(conflicts) > 0 {
					return fmt.Errorf("artifact ID is required when using %s", strings.Join(conflicts, ", "))
				}
				ctx, cancel := context.WithTimeout(cmd.Context(), selfUpdateTimeout)
				defer cancel()
				opts := defaultSelfUpdateOptions()
				opts.CheckOnly = checkSelf
				opts.TargetTag = targetTag
				result, err := selfUpdateRun(ctx, opts)
				if err != nil {
					return err
				}
				return writeSelfUpdateResult(cmd.OutOrStdout(), result)
			}
			if checkSelf || targetTag != "" {
				return fmt.Errorf("--check and --to can only be used without an artifact ID")
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
			if sizeMutationFlagsChanged(cmd) {
				if conflicts := conflictingSizeFlags(cmd); len(conflicts) > 0 {
					return fmt.Errorf(
						"--size cannot be combined with %s: run the size mutation separately to preserve the body",
						strings.Join(conflicts, ", "))
				}
				var sizeErr error
				if cmd.Flags().Changed("size-source") || cmd.Flags().Changed("size-ruleset-version") {
					// Provenance-carrying mutations route through the typed seam
					// (SE-3a/SE-5); implemented in the build phase.
					mutation := core.SizeMutation{Actor: core.ActorContextHuman}
					if cmd.Flags().Changed("size") {
						mutation.Size = &size
					}
					if cmd.Flags().Changed("size-source") {
						mutation.Source = &sizeSource
					}
					if cmd.Flags().Changed("size-ruleset-version") {
						mutation.RulesetVersion = &sizeRuleset
					}
					_, sizeErr = core.SetArtifactSizeWithProvenance(ctx, ws, id, mutation)
				} else {
					// Plain size mutation preserves the existing body-preserving seam.
					_, sizeErr = core.SetArtifactSize(ctx, ws, id, size)
				}
				if sizeErr != nil {
					// A busy task lock surfaces the same non-zero exit code as the
					// doctor --target table (4) instead of blocking, so the
					// autoharness sizing hook sees deterministic contention.
					if errors.Is(sizeErr, core.ErrTaskBusy) {
						cmd.SilenceErrors = true
						return &ExitError{Code: 4, Msg: fmt.Sprintf("task %s is busy: %v", id, sizeErr)}
					}
					return sizeErr
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", id); err != nil {
					return fmt.Errorf("write update confirmation: %w", err)
				}
				return nil
			}

			if cmd.Flags().Changed("complexity") {
				if conflicts := conflictingComplexityFlags(cmd); len(conflicts) > 0 {
					return fmt.Errorf(
						"--complexity cannot be combined with %s: run the complexity mutation separately to preserve the body",
						strings.Join(conflicts, ", "))
				}
				if _, complexityErr := core.SetArtifactComplexity(ctx, ws, id, complexity); complexityErr != nil {
					if errors.Is(complexityErr, core.ErrTaskBusy) {
						cmd.SilenceErrors = true
						return &ExitError{Code: 4, Msg: fmt.Sprintf("task %s is busy: %v", id, complexityErr)}
					}
					return fmt.Errorf("set complexity: %w", complexityErr)
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Updated %s\n", id); err != nil {
					return fmt.Errorf("write update confirmation: %w", err)
				}
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

			// Apply frontmatter updates if any. The gate outcome (when the
			// transition is gated) is captured and its --json payload deferred
			// until after any section updates run, so a combined
			// `--status done --section name=value --json` invocation never
			// silently drops the section write behind an early return.
			var gateOutcome *core.GateOutcome
			if len(updates) > 0 {
				opts := core.TransitionOptions{GateBase: gateBase}
				if forceGates {
					if forceReason == "" {
						return fmt.Errorf("--force-gates requires --force-reason")
					}
					opts.Force = true
					opts.ForceReason = forceReason
					opts.ForceSource = core.ForceSourceCLI
				}
				_, outcome, updateErr := core.UpdateArtifactWithGate(ctx, ws, id, updates, opts)
				if updateErr != nil {
					return moveGateError(cmd, id, updateErr, jsonOut)
				}
				gateOutcome = outcome
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
				now := models.NowUTC()
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

			// Emit the deferred gate --json payload now that any section updates
			// have been persisted, so the machine-readable gate outcome reflects a
			// fully-applied mutation rather than preempting it.
			if jsonOut && gateOutcome != nil {
				payload, mErr := renderGatePassJSON(id, gateOutcome)
				if mErr != nil {
					return mErr
				}
				fmt.Fprintln(cmd.OutOrStdout(), payload)
				return nil
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
	cmd.Flags().StringVar(&complexity, "complexity", "", "implementation difficulty/uncertainty (trivial, low, medium, high); body-preserving, mutually exclusive with other field flags")
	cmd.Flags().StringVar(&sizeSource, "size-source", "", "size provenance source (human, agent, derived)")
	cmd.Flags().StringVar(&sizeRuleset, "size-ruleset-version", "", "size ruleset version")
	cmd.Flags().StringVar(&gateBase, "gate-base", "", "operator-only base ref override for the completion gate (audited)")
	cmd.Flags().BoolVar(&forceGates, "force-gates", false, "operator-only: force completion past the gate (requires --force-reason)")
	cmd.Flags().StringVar(&forceReason, "force-reason", "", "justification recorded in the forced-gate audit event")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit the machine-readable gate outcome contract on a gated completion")
	cmd.Flags().BoolVar(&checkSelf, "check", false, "check whether a backlogit binary update is available without applying it")
	cmd.Flags().StringVar(&targetTag, "to", "", "update the backlogit binary to a specific release tag")
	return cmd
}

func artifactUpdateFlagsChanged(cmd *cobra.Command) []string {
	candidates := []string{
		"title", "status", "priority", "harness-status", "description",
		"sprint", "assigned-to", "owner", "labels", "commit", "section",
		"size", "complexity", "size-source", "size-ruleset-version", "gate-base", "force-gates", "force-reason", "json",
	}
	var changed []string
	for _, name := range candidates {
		if cmd.Flags().Changed(name) {
			changed = append(changed, "--"+name)
		}
	}
	return changed
}

func sizeMutationFlagsChanged(cmd *cobra.Command) bool {
	return cmd.Flags().Changed("size") ||
		cmd.Flags().Changed("size-source") ||
		cmd.Flags().Changed("size-ruleset-version")
}

// conflictingSizeFlags returns the set of frontmatter-mutating flags (rendered as
// --name) that were set alongside --size. --size is single-purpose, so any of
// these makes the invocation ambiguous and must error before any write.
func conflictingSizeFlags(cmd *cobra.Command) []string {
	candidates := []string{
		"title", "status", "priority", "harness-status", "description",
		"sprint", "assigned-to", "owner", "labels", "commit", "section",
		"complexity",
	}
	var conflicts []string
	for _, name := range candidates {
		if cmd.Flags().Changed(name) {
			conflicts = append(conflicts, "--"+name)
		}
	}
	return conflicts
}

// conflictingComplexityFlags returns the set of frontmatter-mutating flags
// (rendered as --name) that were set alongside --complexity.
func conflictingComplexityFlags(cmd *cobra.Command) []string {
	candidates := []string{
		"title", "status", "priority", "harness-status", "description",
		"sprint", "assigned-to", "owner", "labels", "commit", "section",
		"size", "size-source", "size-ruleset-version",
	}
	var conflicts []string
	for _, name := range candidates {
		if cmd.Flags().Changed(name) {
			conflicts = append(conflicts, "--"+name)
		}
	}
	return conflicts
}
