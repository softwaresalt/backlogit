package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

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
  backlogit docs migrate --dry-run
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
		Short: "Validate in-scope documentation frontmatter",
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
			findings, err := docline.LintTree(docline.Options{
				Root:    root,
				Path:    path,
				Profile: docline.Profile(profile),
			})
			if err != nil {
				return err
			}
			valid := len(findings) == 0
			if resolveFormat(format) == "json" {
				payload := map[string]any{
					"valid":           valid,
					"violation_count": len(findings),
					"findings":        findingsPayload(findings),
				}
				if err := writeDocsJSON(cmd.OutOrStdout(), payload); err != nil {
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
	var apply, yes, dryRun bool
	var format, path string
	cmd := &cobra.Command{
		Use:          "migrate",
		Short:        "Plan (default) or apply an idempotent frontmatter migration",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveDocsRoot(cwd)
			if err != nil {
				return err
			}
			opts := docline.Options{Root: root, Path: path, Now: time.Now().UTC()}

			if apply {
				// Apply is guarded: it requires explicit confirmation and an
				// explicit scoped path. A whole-tree apply in one shot is refused.
				if !yes {
					return fmt.Errorf("docs migrate --apply requires --yes to confirm writes")
				}
				if path == "" {
					return fmt.Errorf("docs migrate --apply requires an explicit --path (whole-tree apply is refused)")
				}
				plan, err := docline.PlanMigration(opts)
				if err != nil {
					return err
				}
				res, err := docline.ApplyMigration(plan, opts)
				if err != nil {
					return err
				}
				return writeMigrateResult(cmd, format, plan, &res, false)
			}

			// Default: dry-run plan, no writes.
			_ = dryRun
			plan, err := docline.PlanMigration(opts)
			if err != nil {
				return err
			}
			return writeMigrateResult(cmd, format, plan, nil, true)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "compute the plan without writing (default)")
	cmd.Flags().BoolVar(&apply, "apply", false, "write changes; requires --yes and an explicit --path")
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
			if resolveFormat(format) == "json" {
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

// findingPayload is the pinned JSON shape for a lint finding, shared with the
// MCP docs tools (065.008-T) for surface parity.
type findingPayload struct {
	File     string `json:"file"`
	Field    string `json:"field"`
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Fix      string `json:"fix"`
}

func findingsPayload(findings []docline.Finding) []findingPayload {
	out := make([]findingPayload, 0, len(findings))
	for _, f := range findings {
		out = append(out, findingPayload{
			File:     f.File,
			Field:    f.Field,
			Rule:     f.Rule,
			Severity: string(f.Severity),
			Fix:      f.Fix,
		})
	}
	return out
}

// changePayload is the pinned JSON shape for a migration change.
type changePayload struct {
	File             string `json:"file"`
	Action           string `json:"action"`
	BodyBytesChanged bool   `json:"body_bytes_changed"`
}

func changesPayload(changes []docline.Change) []changePayload {
	out := make([]changePayload, 0, len(changes))
	for _, c := range changes {
		out = append(out, changePayload{
			File:             c.File,
			Action:           string(c.Action),
			BodyBytesChanged: c.BodyBytesChanged,
		})
	}
	return out
}

// writeMigrateResult renders a migration plan (and optional apply result).
func writeMigrateResult(cmd *cobra.Command, format string, plan docline.MigrationPlan, res *docline.Result, dryRun bool) error {
	if resolveFormat(format) == "json" {
		payload := map[string]any{
			"dry_run": dryRun,
			"changes": changesPayload(plan.Changes),
		}
		if res != nil {
			payload["applied"] = res.Applied
			payload["skipped"] = res.Skipped
		}
		return writeDocsJSON(cmd.OutOrStdout(), payload)
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

// resolveFormat picks the output format: an explicit value wins; otherwise text
// on a TTY and json on a non-TTY (pipe/CI).
func resolveFormat(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return "text"
	}
	return "json"
}
