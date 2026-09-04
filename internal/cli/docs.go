package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/docline"
)

// errLintViolations is returned by `docs lint` when the tree has violations so
// the process exits non-zero (CI-friendly) after the findings are printed.
var errLintViolations = errors.New("docline: frontmatter violations found")

// newDocsCommand builds the `backlogit docs` parent command and its thin
// subcommands over the docline application service (internal/docline).
func newDocsCommand(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Lint and migrate documentation frontmatter (docline base schema)",
		Long: `Operate on the repository documentation surface using the docline base
frontmatter contract: lint in-scope docs, plan and apply idempotent migrations,
inspect the active scope, and classify a path's doc_type.`,
		Example: `  backlogit docs lint
  backlogit docs lint --profile ingestion --format json
  backlogit docs migrate
  backlogit docs migrate --apply --yes --path docs/decisions
  backlogit docs scope
  backlogit docs classify docs/decisions/x.md`,
	}
	cmd.AddCommand(newDocsLintCommand(cwd))
	cmd.AddCommand(newDocsMigrateCommand(cwd))
	cmd.AddCommand(newDocsScopeCommand(cwd))
	cmd.AddCommand(newDocsClassifyCommand(cwd))
	return cmd
}

func newDocsLintCommand(cwd *string) *cobra.Command {
	var profile, format, path string
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Validate in-scope documentation frontmatter (retains non-zero exit on violations)",
		Long: `Validate in-scope documentation frontmatter against the docline base schema.

Prints the findings and exits non-zero when any violation exists (CI-friendly).
A per-file frontmatter decode failure (malformed YAML) is reported as a
finding with rule decode_error rather than aborting the scan: the rest of the
corpus is still linted, and the process still exits non-zero because a
corpus containing a decode_error is not a clean tree — the non-zero exit is
retained for this case exactly as for any other violation.`,
		Example: `  backlogit docs lint
  backlogit docs lint --profile ingestion --format json
  backlogit docs lint --path docs/decisions`,
		// We render findings ourselves and signal violations via a non-zero exit
		// (errLintViolations); suppress Cobra's own error/usage noise so the
		// JSON/text payload stays clean for CI consumers.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveDocsRoot(cwd)
			if err != nil {
				return err
			}
			outFormat, err := resolveFormat(cmd.OutOrStdout(), format)
			if err != nil {
				return err
			}
			prof, err := docline.ParseProfile(profile)
			if err != nil {
				return err
			}
			findings, err := docline.LintTree(docline.Options{
				Root:    root,
				Path:    path,
				Profile: prof,
			})
			if err != nil {
				return err
			}
			valid := len(findings) == 0
			if outFormat == "json" {
				if err := writeDocsJSON(cmd.OutOrStdout(), docline.NewLintReport(findings)); err != nil {
					return err
				}
			} else {
				printLintText(cmd, findings)
			}
			if !valid {
				return errLintViolations
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "authoring", "validation profile: authoring, ingestion")
	cmd.Flags().StringVar(&format, "format", "", "output format: text, json (default: text on TTY, json otherwise)")
	cmd.Flags().StringVar(&path, "path", "", "limit to a repo-relative sub-path")
	return cmd
}

func newDocsMigrateCommand(cwd *string) *cobra.Command {
	var apply, yes bool
	var format, path string
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Plan (default) or apply an idempotent frontmatter migration",
		// Suppress Cobra's own error and usage noise so the findings/report
		// payload (JSON or text) stays clean for CI consumers. Mirrors the
		// approach in newDocsLintCommand.
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveDocsRoot(cwd)
			if err != nil {
				return err
			}
			opts := docline.Options{Root: root, Path: path, Now: time.Now().UTC()}
			if _, err := resolveFormat(cmd.OutOrStdout(), format); err != nil {
				return err
			}

			if apply {
				// Apply is guarded: it requires explicit confirmation and an
				// explicit scoped path. A whole-tree apply in one shot is refused.
				if !yes {
					return fmt.Errorf("docs migrate --apply requires --yes to confirm writes")
				}
				if err := docline.ValidateApplyPath(root, path); err != nil {
					return fmt.Errorf("docs migrate --apply requires an explicit scoped --path (whole-tree apply is refused): %w", err)
				}
				plan, err := docline.PlanMigration(opts)
				if err != nil {
					return err
				}
				res, err := docline.ApplyMigration(plan, opts)
				if err != nil {
					if errors.Is(err, docline.ErrPlanHasFindings) {
						// Render the migrate report (including findings) before
						// returning the rejection signal, mirroring the
						// errLintViolations render-then-signal pattern: the caller
						// sees the findings in text/JSON output first, then the
						// non-zero exit signals rejection.
						// dryRun=true: nothing was written; rendering as an apply
						// would misrepresent the outcome to callers.
						if renderErr := writeMigrateResult(cmd, format, plan, nil, true); renderErr != nil {
							return renderErr
						}
						return docline.ErrPlanHasFindings
					}
					return err
				}
				return writeMigrateResult(cmd, format, plan, &res, false)
			}

			// Default (no --apply): compute and report the plan without writing.
			plan, err := docline.PlanMigration(opts)
			if err != nil {
				return err
			}
			return writeMigrateResult(cmd, format, plan, nil, true)
		},
	}
	cmd.Flags().BoolVar(&apply, "apply", false, "write changes; without it migrate only plans (dry-run). Requires --yes and an explicit --path")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm a write (required with --apply)")
	cmd.Flags().StringVar(&format, "format", "", "output format: text, json (default: text on TTY, json otherwise)")
	cmd.Flags().StringVar(&path, "path", "", "limit to a repo-relative sub-path (required for --apply)")
	return cmd
}

func newDocsScopeCommand(cwd *string) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:          "scope",
		Short:        "Print the active docline scope, profiles, and taxonomy",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			sc := docline.Scope()
			outFormat, err := resolveFormat(cmd.OutOrStdout(), format)
			if err != nil {
				return err
			}
			if outFormat == "json" {
				return writeDocsJSON(cmd.OutOrStdout(), sc)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "docline scope")
			fmt.Fprintf(out, "  include dirs:  %v\n", sc.IncludeDirs)
			fmt.Fprintf(out, "  include files: %v\n", sc.IncludeFiles)
			fmt.Fprintf(out, "  exclude dirs:  %v\n", sc.ExcludeDirs)
			fmt.Fprintf(out, "  exclude files: %v\n", sc.ExcludeFiles)
			fmt.Fprintf(out, "  profiles:      %v\n", sc.Profiles)
			fmt.Fprintf(out, "  doc_types:     %v\n", sc.DocTypes)
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "output format: text, json")
	return cmd
}

func newDocsClassifyCommand(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "classify <path>",
		Short:        "Print the derived doc_type for a repo-relative path",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			dt := docline.Classify(args[0])
			fmt.Fprintln(cmd.OutOrStdout(), string(dt))
			return nil
		},
	}
	return cmd
}

// writeMigrateResult renders a migration plan (and optional apply result). The
// JSON form uses the shared docline report types for CLI↔MCP parity.
// In text mode, plan.Findings are rendered after the changes list, mirroring
// the printLintText format, so decode errors surfaced during planning are
// visible in the human-readable output.
func writeMigrateResult(cmd *cobra.Command, format string, plan docline.MigrationPlan, res *docline.Result, dryRun bool) error {
	outFormat, err := resolveFormat(cmd.OutOrStdout(), format)
	if err != nil {
		return err
	}
	if outFormat == "json" {
		return writeDocsJSON(cmd.OutOrStdout(), docline.NewMigrateReport(plan, res, dryRun))
	}

	out := cmd.OutOrStdout()
	if dryRun {
		fmt.Fprintln(out, "docline migrate (dry-run)")
	} else {
		fmt.Fprintln(out, "docline migrate (applied)")
	}
	for _, c := range plan.Changes {
		fmt.Fprintf(out, "  %-6s %s\n", c.Action, c.File)
	}
	if len(plan.Findings) > 0 {
		fmt.Fprintf(out, "docline migrate: %d finding(s)\n", len(plan.Findings))
		for _, f := range plan.Findings {
			fmt.Fprintf(out, "  %s [%s] %s: %s\n", f.File, f.Rule, f.Field, f.Fix)
		}
	}
	if res != nil {
		fmt.Fprintf(out, "applied=%d skipped=%d\n", len(res.Applied), len(res.Skipped))
	}
	return nil
}

// printLintText renders findings as a human-readable list.
func printLintText(cmd *cobra.Command, findings []docline.Finding) {
	out := cmd.OutOrStdout()
	if len(findings) == 0 {
		fmt.Fprintln(out, "docline lint: OK (0 violations)")
		return
	}
	fmt.Fprintf(out, "docline lint: %d violation(s)\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(out, "  %s [%s] %s: %s\n", f.File, f.Rule, f.Field, f.Fix)
	}
}

// writeDocsJSON encodes v as indented JSON to w.
func writeDocsJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode docs json: %w", err)
	}
	return nil
}

// resolveDocsRoot returns the absolute workspace root for docline operations.
func resolveDocsRoot(cwd *string) (string, error) {
	root := "."
	if cwd != nil && *cwd != "" {
		root = *cwd
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve docs root: %w", err)
	}
	return abs, nil
}

// resolveFormat picks the output format: an explicit value wins (validated to
// the allowed set), otherwise text on a TTY and json on a non-TTY (pipe/CI).
// TTY detection is based on the command's actual output writer so redirected
// output (cmd.SetOut) is honored, and an unrecognized explicit format fails
// fast instead of silently falling back to text.
func resolveFormat(out io.Writer, explicit string) (string, error) {
	switch explicit {
	case "":
		if isTerminal(out) {
			return "text", nil
		}
		return "json", nil
	case "text", "json":
		return explicit, nil
	default:
		return "", fmt.Errorf("invalid --format %q: must be text or json", explicit)
	}
}
