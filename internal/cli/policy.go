package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/softwaresalt/backlogit/internal/evidence"
	"github.com/softwaresalt/backlogit/internal/evidencedag"
	"github.com/softwaresalt/backlogit/internal/policyengine"
)

// defaultPolicyRegistry is the checked-in workflow-policy registry.
const defaultPolicyRegistry = ".github/policies/workflow-policy-registry.yaml"

// Test seams: the CLI wires the real git executable, the GitHub derivation
// source, the terminal check and the clock; tests substitute in-memory ones.
var (
	policyGit         = func(root string) policyengine.Git { return policyengine.ExecGit{Dir: root} }
	policyDerivation  = func(repo string) policyengine.Derivation { return &evidence.GitHub{Repo: repo} }
	policyInteractive = func(out io.Writer) bool { return term.IsTerminal(int(os.Stdin.Fd())) && isTerminal(out) }
	policyNow         = time.Now
)

// errPolicyBlocked is returned by `policy check` when the transition is not
// allowed, so the process exits non-zero.
var errPolicyBlocked = errors.New("policy: transition blocked by workflow-policy gates")

// policyBlockedError carries the blocking decision so structured (JSON-RPC)
// callers receive the gate errors, not only the sentinel message.
type policyBlockedError struct {
	Decision policyengine.Decision
}

func (e *policyBlockedError) Error() string { return errPolicyBlocked.Error() }

// Unwrap lets errors.Is(err, errPolicyBlocked) keep matching.
func (e *policyBlockedError) Unwrap() error { return errPolicyBlocked }

// newPolicyCommand builds the `backlogit policy` parent command, a thin layer
// over the shared engine (internal/policyengine).
func newPolicyCommand(cwd *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Enforce workflow-policy gates (harness, red/green, review, CI, merge-ready)",
	}
	cmd.AddCommand(newPolicyCheckCommand(cwd), newPolicyVerifyOverridesCommand(cwd))
	return cmd
}

type policyCheckFlags struct {
	registry, evidence, repo, mode, format string
	trust, ledger, anchor                  string
	protectedRefs, tokenFiles              []string
}

func newPolicyCheckCommand(cwd *string) *cobra.Command {
	var f policyCheckFlags
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Evaluate one task's commit-bound evidence against the workflow-policy graph",
		Long: `Evaluate the prerequisite graph harness-scaffold -> red-phase-verify ->
green-phase-verify -> review-complete -> ci-pass -> merge-ready for one task.

This is the same evaluation the MCP and agent surfaces run (the shared
policyengine Engine.Check); no surface has a separate path. Every gate fails
closed: absent, ambiguous, unverifiable or out-of-order evidence blocks.
Evidence commits are verified against the protected remote-tracking refs
(--protected-ref), never only a local branch. review-complete and ci-pass are
derived from the pull request in the evidence bundle (requires --repo);
self-asserted records for them are refused.

The registry's mode applies (the shipped registry is report-only: violations
are reported and the exit status stays zero). --mode may tighten a report-only
registry but never relax an enforce one. A malformed request is always an
error.

Overrides (P-002.1 harness-exempt) need a signed operator token verified
against --trust, recorded in the hash-chained --ledger and anchored in
--anchor. All three must be absolute paths outside the workspace. Tokens are
refused outside an interactive terminal; operator identity is never read from
the evidence or the command line.`,
		Example: `  backlogit policy check --evidence evidence/163.003-T.json --protected-ref refs/remotes/origin/main --protected-ref refs/remotes/origin/feat/x --repo owner/name
  backlogit policy check --evidence bundle.json --mode enforce --format json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runPolicyCheck(cmd, cwd, f)
		},
	}
	cmd.Flags().StringVar(&f.registry, "registry", defaultPolicyRegistry, "workspace-relative workflow-policy registry")
	cmd.Flags().StringVar(&f.evidence, "evidence", "", "workspace-relative JSON evidence bundle (required)")
	cmd.Flags().StringArrayVar(&f.protectedRefs, "protected-ref", []string{"refs/remotes/origin/main"}, "remote-tracking ref the head commit must be reachable from (repeatable)")
	cmd.Flags().StringVar(&f.repo, "repo", "", "owner/name repository used to derive review and CI evidence")
	cmd.Flags().StringVar(&f.mode, "mode", "", "enforcement mode: report-only, enforce (may only tighten the registry mode)")
	cmd.Flags().StringArrayVar(&f.tokenFiles, "override-token", nil, "workspace-relative signed operator override token file (repeatable)")
	cmd.Flags().StringVar(&f.trust, "trust", "", "absolute path of the operator trust configuration (outside the workspace)")
	cmd.Flags().StringVar(&f.ledger, "ledger", "", "absolute path of the override ledger (outside the workspace)")
	cmd.Flags().StringVar(&f.anchor, "anchor", "", "absolute path of the ledger anchor (outside the workspace)")
	cmd.Flags().StringVar(&f.format, "format", "", "output format: text, json (default: text on TTY, json otherwise)")
	return cmd
}

// validateWorkspaceRel rejects empty, absolute, NUL-bearing and
// parent-traversing paths on the raw input, before any resolver can clean
// them away.
func validateWorkspaceRel(flag, rel string) error {
	if rel == "" {
		return fmt.Errorf("--%s is required", flag)
	}
	if strings.ContainsRune(rel, 0) || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) || filepath.VolumeName(rel) != "" {
		return fmt.Errorf("--%s must be a workspace-relative path", flag)
	}
	for _, seg := range strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' }) {
		if seg == ".." {
			return fmt.Errorf("--%s must not contain '..' segments", flag)
		}
	}
	return nil
}

// readPolicyFile reads a workspace-confined regular file of at most max bytes.
func readPolicyFile(root, flag, rel string, max int) ([]byte, error) {
	if err := validateWorkspaceRel(flag, rel); err != nil {
		return nil, err
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open workspace root: %w", err)
	}
	defer func() { _ = r.Close() }()
	f, err := r.Open(filepath.ToSlash(rel))
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", rel, err)
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %q: %w", rel, err)
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%q is not a regular file", rel)
	}
	data, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", rel, err)
	}
	if len(data) > max {
		return nil, fmt.Errorf("%q exceeds %d bytes", rel, max)
	}
	return data, nil
}

func buildWaiverService(root string, f policyCheckFlags) (*evidencedag.WaiverService, error) {
	given := 0
	for _, p := range []string{f.trust, f.ledger, f.anchor} {
		if p != "" {
			given++
		}
	}
	switch given {
	case 0:
		return nil, nil
	case 3:
	default:
		return nil, errors.New("--trust, --ledger and --anchor must be given together")
	}
	for flag, p := range map[string]string{"trust": f.trust, "ledger": f.ledger, "anchor": f.anchor} {
		if !filepath.IsAbs(p) {
			return nil, fmt.Errorf("--%s must be an absolute path", flag)
		}
	}
	trust, err := evidencedag.LoadTrust(f.trust, root)
	if err != nil {
		return nil, fmt.Errorf("load trust: %w", err)
	}
	ledger, err := evidencedag.OpenLedger(f.ledger, root)
	if err != nil {
		return nil, fmt.Errorf("open ledger: %w", err)
	}
	anchors, err := evidencedag.NewFileAnchorStore(f.anchor, root)
	if err != nil {
		return nil, fmt.Errorf("open anchor: %w", err)
	}
	svc, err := evidencedag.NewWaiverService(trust, ledger, anchors, policyNow)
	if err != nil {
		return nil, fmt.Errorf("waiver service: %w", err)
	}
	return svc, nil
}

func runPolicyCheck(cmd *cobra.Command, cwd *string, f policyCheckFlags) error {
	root, err := resolveDocsRoot(cwd)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	outFormat, err := resolveFormat(out, f.format)
	if err != nil {
		return err
	}
	regData, err := readPolicyFile(root, "registry", f.registry, evidencedag.MaxDeclarationBytes)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}
	reg, err := policyengine.LoadRegistry(regData)
	if err != nil {
		return fmt.Errorf("load registry: %w", err)
	}
	bundleData, err := readPolicyFile(root, "evidence", f.evidence, policyengine.MaxBundleBytes)
	if err != nil {
		return fmt.Errorf("load evidence: %w", err)
	}
	bundle, err := policyengine.DecodeBundle(bundleData)
	if err != nil {
		return fmt.Errorf("load evidence: %w", err)
	}
	tokens := make([][]byte, 0, len(f.tokenFiles))
	for _, name := range f.tokenFiles {
		tok, err := readPolicyFile(root, "override-token", name, evidencedag.MaxWaiverTokenBytes)
		if err != nil {
			return fmt.Errorf("load override token: %w", err)
		}
		tokens = append(tokens, tok)
	}
	svc, err := buildWaiverService(root, f)
	if err != nil {
		return err
	}
	opts := policyengine.Options{
		Git: policyGit(root), ProtectedRefs: f.protectedRefs, Waivers: svc,
		Mode: policyengine.Mode(f.mode), Now: policyNow,
	}
	if f.repo != "" {
		opts.Derivation = policyDerivation(f.repo)
	}
	eng, err := policyengine.NewEngine(reg, opts)
	if err != nil {
		return fmt.Errorf("policy check: %w", err)
	}
	req := bundle.Request(policyengine.SurfaceCLI)
	req.Interactive = policyInteractive(out)
	req.OverrideTokens = tokens

	dec, checkErr := eng.Check(context.Background(), req)
	if outFormat == "json" {
		if err := writeDocsJSON(out, dec); err != nil {
			return err
		}
	} else {
		printPolicyText(out, dec)
	}
	if checkErr != nil {
		cmd.SilenceErrors = true
		return fmt.Errorf("policy check: %w", checkErr)
	}
	if !dec.Allowed {
		cmd.SilenceErrors = true
		return &policyBlockedError{Decision: dec}
	}
	return nil
}

func printPolicyText(w io.Writer, d policyengine.Decision) {
	_, _ = fmt.Fprintf(w, "policy check %s @ %.12s (%s, %s)\n", d.TaskID, d.HeadCommit, d.Mode, d.Surface)
	for _, g := range d.Gates {
		line := fmt.Sprintf("  %-20s %-14s", g.Node, g.Status)
		if g.Waiver != "" {
			line += " waived by " + g.Waiver
		}
		_, _ = fmt.Fprintln(w, strings.TrimRight(line, " "))
	}
	for _, v := range d.Errors {
		_, _ = fmt.Fprintf(w, "error [%s] %s: %s\n", v.Code, v.Node, v.Reason)
	}
	for _, dn := range d.OverrideDenials {
		_, _ = fmt.Fprintf(w, "override %d denied: %s\n", dn.Index, dn.Reason)
	}
	_, _ = fmt.Fprintf(w, "policy (%s): %s\n", d.Disposition, d.Reason)
}

func newPolicyVerifyOverridesCommand(cwd *string) *cobra.Command {
	var ledgerPath, anchorPath, format string
	cmd := &cobra.Command{
		Use:   "verify-overrides",
		Short: "Verify the tamper-evident override ledger against its external anchor",
		Long: `Verify the append-only, hash-chained override ledger against its external
anchor and list its entries (authenticated principal, justification, gate).
An edited, reordered, extended, truncated, emptied or deleted ledger fails.
Both paths must be absolute and outside the workspace.`,
		Example:      `  backlogit policy verify-overrides --ledger /secure/overrides.jsonl --anchor /secure/anchor.json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := resolveDocsRoot(cwd)
			if err != nil {
				return err
			}
			outFormat, err := resolveFormat(cmd.OutOrStdout(), format)
			if err != nil {
				return err
			}
			for flag, p := range map[string]string{"ledger": ledgerPath, "anchor": anchorPath} {
				if p == "" || !filepath.IsAbs(p) {
					return fmt.Errorf("--%s must be an absolute path", flag)
				}
			}
			ledger, err := evidencedag.OpenLedger(ledgerPath, root)
			if err != nil {
				return fmt.Errorf("open ledger: %w", err)
			}
			anchors, err := evidencedag.NewFileAnchorStore(anchorPath, root)
			if err != nil {
				return fmt.Errorf("open anchor: %w", err)
			}
			log, err := policyengine.VerifyOverrideLog(ledger, anchors)
			if err != nil {
				return fmt.Errorf("policy verify-overrides: %w", err)
			}
			if outFormat == "json" {
				return writeDocsJSON(cmd.OutOrStdout(), log)
			}
			for _, e := range log.Entries {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d %s %s %s %s\n", e.Seq, e.TaskID, e.Gate, e.Principal, e.Justification)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "override log: %d record(s), chain verified\n", log.Records)
			return nil
		},
	}
	cmd.Flags().StringVar(&ledgerPath, "ledger", "", "absolute path of the override ledger (outside the workspace)")
	cmd.Flags().StringVar(&anchorPath, "anchor", "", "absolute path of the ledger anchor (outside the workspace)")
	cmd.Flags().StringVar(&format, "format", "", "output format: text, json (default: text on TTY, json otherwise)")
	return cmd
}
