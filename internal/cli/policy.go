package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/softwaresalt/backlogit/internal/evidence"
	"github.com/softwaresalt/backlogit/internal/evidencedag"
	"github.com/softwaresalt/backlogit/internal/faultline"
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
	// policyRepo resolves the git toplevel containing dir and the URL of its
	// origin remote (empty with an error when there is none).
	policyRepo = func(dir string) (toplevel, origin string, err error) {
		g := policyengine.ExecGit{Dir: dir}
		toplevel, err = g.Toplevel(context.Background())
		if err != nil {
			return "", "", fmt.Errorf("resolve git toplevel: %w", err)
		}
		origin, err = g.OriginURL(context.Background())
		if err != nil {
			return toplevel, "", fmt.Errorf("resolve origin remote: %w", err)
		}
		return toplevel, origin, nil
	}
)

// githubRemote matches the GitHub remote URL forms and captures owner and name.
var githubRemote = regexp.MustCompile(`^(?:https?://(?:[^@/\s]+@)?github\.com/|ssh://git@github\.com/|git@github\.com:)([A-Za-z0-9_.-]+)/([A-Za-z0-9_.-]+?)(?:\.git)?/?$`)

// githubSlug derives owner/name from a GitHub remote URL.
func githubSlug(origin string) (string, bool) {
	if len(origin) > 512 {
		return "", false
	}
	m := githubRemote.FindStringSubmatch(origin)
	if m == nil {
		return "", false
	}
	return m[1] + "/" + m[2], true
}

// checkRepoAgainstOrigin binds --repo to the checkout: when origin resolves to
// a GitHub slug it must equal repo (derived review/CI evidence for a different
// repository is refused). An origin that does not resolve to a GitHub slug
// cannot be checked, so enforce mode fails closed; report-only mode warns.
func checkRepoAgainstOrigin(repo, origin string, originErr error, enforce bool, warn io.Writer) error {
	if repo == "" {
		return nil
	}
	slug, ok := githubSlug(origin)
	if originErr != nil || !ok {
		if enforce {
			return fmt.Errorf("--repo %.80q cannot be checked: the origin remote does not resolve to a github.com owner/name (enforce mode fails closed)", repo)
		}
		_, _ = fmt.Fprintf(warn, "warning: --repo %.80q is not verified against the origin remote: it does not resolve to a github.com owner/name\n", repo)
		return nil
	}
	if !strings.EqualFold(slug, repo) {
		return fmt.Errorf("--repo %.80q does not match the origin remote %q", repo, slug)
	}
	return nil
}

// commandContext returns the command's context, or Background when unset.
func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// repoRoot resolves the git toplevel for the --cwd directory. It is the single
// root for workspace-relative inputs and the forbidden root for the trust,
// ledger and anchor paths, so a checkout subdirectory cannot widen the
// agent-writable area. It fails closed outside a git repository.
func repoRoot(cwd *string) (root, origin string, originErr error, err error) {
	dir, err := resolveDocsRoot(cwd)
	if err != nil {
		return "", "", nil, err
	}
	top, origin, originErr := policyRepo(dir)
	if top == "" || !filepath.IsAbs(top) {
		if originErr == nil {
			originErr = errors.New("git toplevel is unavailable")
		}
		return "", "", nil, fmt.Errorf("policy commands require a git repository: %w", originErr)
	}
	return top, origin, originErr, nil
}

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
derived from the pull request in the evidence bundle (requires --repo, which
must match the github.com owner/name of the origin remote; when origin does
not resolve to one, enforce mode fails closed and report-only mode warns);
self-asserted records for them are refused. Workspace-relative paths resolve
against the git toplevel, not the current directory.

The registry's mode applies (the shipped registry is report-only: violations
are reported and the exit status stays zero). --mode may tighten a report-only
registry but never relax an enforce one. A malformed request is always an
error.

Overrides (P-002.1 harness-exempt) need a signed operator token verified
against --trust, recorded in the hash-chained --ledger and anchored in
--anchor. All three must be absolute paths outside the git toplevel, and the
ledger and anchor must be in different directories. Tokens are
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
	cmd.Flags().StringVar(&f.trust, "trust", "", "absolute path of the operator trust configuration (outside the repository)")
	cmd.Flags().StringVar(&f.ledger, "ledger", "", "absolute path of the override ledger (outside the repository)")
	cmd.Flags().StringVar(&f.anchor, "anchor", "", "absolute path of the ledger anchor (outside the repository)")
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

// readPolicyFile reads a workspace-confined regular file of at most limit
// bytes through the shared bounded reader (no traversal or symlinks, regular
// file checked before a non-blocking open, so a FIFO cannot hang the command).
func readPolicyFile(root, flag, rel string, limit int) ([]byte, error) {
	if err := validateWorkspaceRel(flag, rel); err != nil {
		return nil, err
	}
	data, err := evidencedag.ReadWorkspaceFile(root, rel, limit)
	if err != nil {
		return nil, fmt.Errorf("read --%s %q: %w", flag, clipFlagValue(rel), err)
	}
	return data, nil
}

func clipFlagValue(s string) string {
	if len(s) > 80 {
		return s[:80]
	}
	return s
}

// requireDistinctDirs refuses a ledger and an anchor that share a directory:
// whoever can write one could rewrite the other, defeating the external anchor.
func requireDistinctDirs(ledgerPath, anchorPath string) error {
	ld, lerr := os.Stat(filepath.Dir(ledgerPath))
	ad, aerr := os.Stat(filepath.Dir(anchorPath))
	if lerr == nil && aerr == nil && os.SameFile(ld, ad) {
		return errors.New("--ledger and --anchor must live in different directories")
	}
	return nil
}

func requireAbsolute(flags []struct{ name, path string }) error {
	for _, fl := range flags {
		if fl.path == "" || !filepath.IsAbs(fl.path) {
			return fmt.Errorf("--%s must be an absolute path", fl.name)
		}
	}
	return nil
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
	if err := requireAbsolute([]struct{ name, path string }{{"trust", f.trust}, {"ledger", f.ledger}, {"anchor", f.anchor}}); err != nil {
		return nil, err
	}
	if err := requireDistinctDirs(f.ledger, f.anchor); err != nil {
		return nil, err
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
	root, origin, originErr, err := repoRoot(cwd)
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
	enforce := reg.Mode() == policyengine.ModeEnforce || policyengine.Mode(f.mode) == policyengine.ModeEnforce
	if err := checkRepoAgainstOrigin(f.repo, origin, originErr, enforce, cmd.ErrOrStderr()); err != nil {
		return err
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

	dec, checkErr := eng.Check(commandContext(cmd), req)
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

// safeText makes untrusted text safe for a terminal: C0/C1 control characters
// (including ESC and CSI) are removed or escaped.
func safeText(s string) string {
	s = strings.Map(func(r rune) rune {
		if r >= 0x80 && unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
	return faultline.SanitizeDiagnostic(s)
}

func printPolicyText(w io.Writer, d policyengine.Decision) {
	_, _ = fmt.Fprintf(w, "policy check %s @ %.12s (%s, %s)\n", safeText(d.TaskID), safeText(d.HeadCommit), safeText(string(d.Mode)), safeText(string(d.Surface)))
	for _, g := range d.Gates {
		line := fmt.Sprintf("  %-20s %-14s", safeText(string(g.Node)), safeText(string(g.Status)))
		if g.Waiver != "" {
			line += " waived by " + safeText(g.Waiver)
		}
		_, _ = fmt.Fprintln(w, strings.TrimRight(line, " "))
	}
	for _, v := range d.Errors {
		_, _ = fmt.Fprintf(w, "error [%s] %s: %s\n", safeText(v.Code), safeText(v.Node), safeText(v.Reason))
	}
	for _, dn := range d.OverrideDenials {
		_, _ = fmt.Fprintf(w, "override %d denied: %s\n", dn.Index, safeText(dn.Reason))
	}
	_, _ = fmt.Fprintf(w, "policy (%s): %s\n", safeText(string(d.Disposition)), safeText(d.Reason))
}

func newPolicyVerifyOverridesCommand(cwd *string) *cobra.Command {
	var ledgerPath, anchorPath, format string
	var genesis bool
	cmd := &cobra.Command{
		Use:   "verify-overrides",
		Short: "Verify the tamper-evident override ledger against its external anchor",
		Long: `Verify the append-only, hash-chained override ledger against its external
anchor and list its entries (authenticated principal, justification, gate).
An edited, reordered, extended, truncated, emptied or deleted ledger fails,
and so does a missing anchor unless --genesis declares that no override has
ever been recorded (the ledger must then be empty).
Both paths must be absolute, outside the repository and in different
directories. Verification takes the ledger's lock file, so it needs
write access to the ledger directory.`,
		Example: `  backlogit policy verify-overrides --ledger /secure/ledger/overrides.jsonl --anchor /secure/anchor/anchor.json
  backlogit policy verify-overrides --genesis --ledger /secure/ledger/overrides.jsonl --anchor /secure/anchor/anchor.json`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, _, _, err := repoRoot(cwd)
			if err != nil {
				return err
			}
			outFormat, err := resolveFormat(cmd.OutOrStdout(), format)
			if err != nil {
				return err
			}
			if err := requireAbsolute([]struct{ name, path string }{{"ledger", ledgerPath}, {"anchor", anchorPath}}); err != nil {
				return err
			}
			if err := requireDistinctDirs(ledgerPath, anchorPath); err != nil {
				return err
			}
			ledger, err := evidencedag.OpenLedger(ledgerPath, root)
			if err != nil {
				return fmt.Errorf("open ledger: %w", err)
			}
			anchors, err := evidencedag.NewFileAnchorStore(anchorPath, root)
			if err != nil {
				return fmt.Errorf("open anchor: %w", err)
			}
			var opts []policyengine.VerifyOption
			if genesis {
				opts = append(opts, policyengine.AllowGenesis())
			}
			log, err := policyengine.VerifyOverrideLog(ledger, anchors, opts...)
			if err != nil {
				return fmt.Errorf("policy verify-overrides: %w", err)
			}
			if outFormat == "json" {
				return writeDocsJSON(cmd.OutOrStdout(), log)
			}
			for _, e := range log.Entries {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%d %s %s %s %s\n", e.Seq, safeText(e.TaskID), safeText(e.Gate), safeText(e.Principal), safeText(e.Justification))
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "override log: %d record(s), chain verified\n", log.Records)
			return nil
		},
	}
	cmd.Flags().StringVar(&ledgerPath, "ledger", "", "absolute path of the override ledger (outside the repository)")
	cmd.Flags().StringVar(&anchorPath, "anchor", "", "absolute path of the ledger anchor (outside the repository, not in the ledger's directory)")
	cmd.Flags().BoolVar(&genesis, "genesis", false, "accept a missing anchor as the genesis state (the ledger must then be empty)")
	cmd.Flags().StringVar(&format, "format", "", "output format: text, json (default: text on TTY, json otherwise)")
	return cmd
}
