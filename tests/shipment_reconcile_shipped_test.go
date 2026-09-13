// Package tests_test contains top-level integration tests for backlogit that
// exercise the real CLI command tree (internal/cli) against a real temp git
// repository and a real temp backlogit workspace.
//
// 167.005-T (#423, 167-F, "U5"): integration test for `backlogit shipment
// reconcile-shipped`, reproducing the real graphtor 048-S closure-evidence
// shape end-to-end: a legacy archived shipment (archived_status: active, all
// manifest members terminal) plus a real git history containing a TRUE merge
// commit (>=2 parents) for the delivery ("feature") role and a second TRUE
// merge commit for the post-merge-closure role, narrated in a closure file
// using the exact role-annotated grammar
// verifyShipmentReconcileClosureDeliveryMerge /
// shipmentReconcileFeatureMergeFromClosure / shipmentReconcileShipmentSection
// (internal/core/shipment_reconcile_evidence.go) parse:
//
//	Merged as PR #101 (feature), commit <sha>, and PR #102 (post-merge
//	closure), commit <sha>.
//
// This test exercises the CLI layer directly (cli.NewRootCommand(), the same
// pattern tests/integration/workflow_test.go and
// tests/contract/dep_type_parity_test.go already use), NOT
// core.ReconcileShipmentToShipped directly: internal/cli exports
// NewShipmentCmd/NewRootCommand precisely for test-harness access, and the
// `shipment reconcile-shipped` subcommand is registered under NewShipmentCmd
// (internal/cli/shipment.go), so it is reachable from this package without
// any Go internal-package visibility problem (tests/ shares the module root
// with internal/, which is the scope internal packages are visible from).
//
// internal/cli/shipment_reconcile_shipped_test.go (167.004-T) already covers
// every CLI-boundary concern (flag validation, the confirmation guard, exit
// codes) at the unit level, and explicitly documents that a fully
// evidence-verified "reconciled" happy path needs a real git repository and
// real evidence files that are "impractical to construct in this CLI-layer
// unit test" — this file is exactly that follow-up.
package tests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// --- git fixture plumbing ----------------------------------------------------

func reconcileShippedRequireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not available: %v", err)
	}
}

func reconcileShippedRunGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	reconcileShippedRequireGit(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmdArgs := append([]string{"-C", root}, args...)
	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		require.NoError(t, ctx.Err(), "git %s timed out", strings.Join(args, " "))
	}
	require.NoErrorf(t, err, "git %s failed:\n%s", strings.Join(args, " "), string(out))
	return string(out)
}

func reconcileShippedInitGitRepo(t *testing.T, root string) {
	t.Helper()
	reconcileShippedRequireGit(t)
	reconcileShippedRunGit(t, root, "-c", "init.defaultBranch=main", "init")
	reconcileShippedRunGit(t, root, "config", "user.email", "backlogit-tests@example.invalid")
	reconcileShippedRunGit(t, root, "config", "user.name", "Backlogit Tests")
}

// reconcileShippedShortSHA truncates a full git object name to a 7-character
// abbreviation, matching the shape the real graphtor closure narrative uses
// and that shipmentReconcilePRCommitRe accepts ([0-9a-f]{7,64}).
func reconcileShippedShortSHA(full string) string {
	if len(full) < 7 {
		return full
	}
	return full[:7]
}

// reconcileGitFixture is a real git repository reproducing the graphtor
// 048-S shape: a TRUE merge commit (>=2 parents) delivering the feature
// scope, a second TRUE merge commit delivering the post-merge closure, a
// plain (single-parent) follow-on commit reachable from the trusted ref, and
// a merge commit that is real but never merged into the trusted "main"
// branch (unreachable).
type reconcileGitFixture struct {
	root           string
	baseSHA        string
	deliverySHA    string
	closureSHA     string
	nonMergeSHA    string
	hiddenMergeSHA string
}

func setupReconcileGitFixture(t *testing.T, root string) reconcileGitFixture {
	t.Helper()
	reconcileShippedInitGitRepo(t, root)

	writeCommit := func(rel, content, msg string) string {
		t.Helper()
		abs := filepath.Join(root, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
		reconcileShippedRunGit(t, root, "add", filepath.ToSlash(rel))
		reconcileShippedRunGit(t, root, "commit", "-m", msg)
		return strings.TrimSpace(reconcileShippedRunGit(t, root, "rev-parse", "HEAD"))
	}
	mergeBranch := func(branch, msg string) string {
		t.Helper()
		reconcileShippedRunGit(t, root, "merge", "--no-ff", branch, "-m", msg)
		return strings.TrimSpace(reconcileShippedRunGit(t, root, "rev-parse", "HEAD"))
	}

	baseSHA := writeCommit("README.md", "base\n", "base")

	reconcileShippedRunGit(t, root, "checkout", "-b", "feature-delivery")
	_ = writeCommit("delivery/scope.txt", "feature delivery work\n", "feature delivery work")
	reconcileShippedRunGit(t, root, "checkout", "main")
	deliverySHA := mergeBranch("feature-delivery", "merge feature delivery")

	reconcileShippedRunGit(t, root, "checkout", "-b", "post-merge-closure")
	_ = writeCommit("closures/summary.txt", "post merge closure work\n", "post merge closure work")
	reconcileShippedRunGit(t, root, "checkout", "main")
	closureSHA := mergeBranch("post-merge-closure", "merge post-merge closure")

	nonMergeSHA := writeCommit("post/plain.txt", "plain follow-on commit\n", "plain follow-on commit")

	// A merge commit that genuinely exists (>=2 parents) but was never
	// merged into "main": reachable from nothing the trusted-ref resolver
	// will ever walk.
	reconcileShippedRunGit(t, root, "checkout", "-b", "hidden-main", baseSHA)
	_ = writeCommit("hidden/main.txt", "hidden main work\n", "hidden main work")
	reconcileShippedRunGit(t, root, "checkout", "-b", "hidden-feature")
	_ = writeCommit("hidden/feature.txt", "hidden feature work\n", "hidden feature work")
	reconcileShippedRunGit(t, root, "checkout", "hidden-main")
	hiddenMergeSHA := mergeBranch("hidden-feature", "merge hidden feature")
	reconcileShippedRunGit(t, root, "checkout", "main")

	return reconcileGitFixture{
		root:           root,
		baseSHA:        baseSHA,
		deliverySHA:    deliverySHA,
		closureSHA:     closureSHA,
		nonMergeSHA:    nonMergeSHA,
		hiddenMergeSHA: hiddenMergeSHA,
	}
}

// --- workspace + shipment fixture plumbing -----------------------------------

// reconcileShippedFixture bundles a real git repository, a real backlogit
// workspace rooted at the SAME directory, and an archived shipment whose
// pre-state exactly matches the legacy shape 167-F governs: status=archived,
// archived_status=active, every manifest member terminal (unless the caller
// asked for a non-terminal member via memberFinalStatus).
type reconcileShippedFixture struct {
	reconcileGitFixture
	ws         *core.Workspace
	shipmentID string
	memberID   string
	closureRel string
}

// newReconcileShippedFixture builds a full end-to-end fixture: a real git
// repository reproducing the graphtor 048-S shape, a real backlogit
// workspace, an archived shipment with one manifest member at
// memberFinalStatus, and a closure file at the returned closureRel path
// whose shipment-scoped section narrates BOTH the delivery and post-merge
// closure commits with the exact role annotations
// shipmentReconcileFeatureMergeFromClosure requires.
func newReconcileShippedFixture(t *testing.T, memberFinalStatus string) *reconcileShippedFixture {
	t.Helper()
	root := t.TempDir()
	backlogDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))
	require.NoError(t, os.MkdirAll(filepath.Join(backlogDir, "archive"), 0o755))

	gitFixture := setupReconcileGitFixture(t, root)

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	feature, err := core.CreateArtifact(ctx, ws, "167.005-T fixture covering feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(ctx, ws, "167.005-T fixture member task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	if memberFinalStatus != string(models.StatusQueued) {
		_, err = core.UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": string(models.StatusActive)})
		require.NoError(t, err)
	}
	if memberFinalStatus != string(models.StatusQueued) && memberFinalStatus != string(models.StatusActive) {
		_, err = core.UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": memberFinalStatus})
		require.NoError(t, err)
	}

	shipment, err := core.CreateShipment(ctx, ws, "167.005-T reconcile fixture shipment", []string{member.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	_, err = core.ArchiveItem(ctx, ws.DB, ws, shipment.ID)
	require.NoError(t, err)

	fx := &reconcileShippedFixture{
		reconcileGitFixture: gitFixture,
		ws:                  ws,
		shipmentID:          shipment.ID,
		memberID:            member.ID,
		closureRel:          filepath.Join("docs", "closure", "167-005-t-closure-summary.md"),
	}
	fx.writeClosureFile(t, fx.closureRel, fx.syntheticClosureContent(fx.deliverySHA, fx.closureSHA))
	return fx
}

// syntheticClosureContent reproduces the EXACT real graphtor 048-S
// closure-file narrative grammar: a shipment-scoped Markdown section headed
// by the literal shipment ID, naming both the delivery ("feature") and
// post-merge-closure commits via shipmentReconcilePRCommitRe's
// `PR #<n> (<role>), commit <sha>` shape.
func (fx *reconcileShippedFixture) syntheticClosureContent(deliverySHA, closureSHA string) string {
	return fmt.Sprintf(`---
shipments:
  - %s
---

# Closure summary

### %s
Merged as PR #101 (feature), commit %s, and PR #102 (post-merge closure), commit %s.
`, fx.shipmentID, fx.shipmentID, reconcileShippedShortSHA(deliverySHA), reconcileShippedShortSHA(closureSHA))
}

func (fx *reconcileShippedFixture) writeClosureFile(t *testing.T, rel, content string) {
	t.Helper()
	abs := filepath.Join(fx.root, filepath.FromSlash(filepath.ToSlash(rel)))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

// archivedPath returns the on-disk path of the shipment's archived Markdown
// artifact.
func (fx *reconcileShippedFixture) archivedPath(t *testing.T) string {
	t.Helper()
	path, err := core.FindArtifactPath(context.Background(), fx.ws, fx.shipmentID)
	require.NoError(t, err)
	return path
}

// readArchivedStatusFromMarkdown is the "predecessor-status probe": it
// re-reads the shipment's RAW Markdown bytes from disk and parses
// archived_status directly out of the YAML frontmatter, never going through
// any backlogit-core in-memory status predicate or cache.
func (fx *reconcileShippedFixture) readArchivedStatusFromMarkdown(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(fx.archivedPath(t))
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	status, _ := fm["archived_status"].(string)
	return status
}

func (fx *reconcileShippedFixture) itemLogPath() string {
	return events.LogPathForItem(core.WorkspaceLogsRoot(fx.ws.RootPath), fx.shipmentID)
}

// args builds the base (non-confirmed, non-dry-run) CLI argument vector for
// `backlogit shipment reconcile-shipped`.
func (fx *reconcileShippedFixture) args(mergeSHA, closureRel, idempotencyKey, actor, reason string) []string {
	return []string{
		"shipment", "reconcile-shipped", fx.shipmentID,
		"--reason", reason,
		"--actor", actor,
		"--idempotency-key", idempotencyKey,
		"--merge-sha", mergeSHA,
		"--closure-evidence", filepath.ToSlash(closureRel),
	}
}

func (fx *reconcileShippedFixture) confirmArg() []string {
	return []string{"--confirm", "reconcile-shipped " + fx.shipmentID}
}

// --- CLI invocation plumbing --------------------------------------------------

// runReconcileShippedCLI invokes the REAL cobra command tree
// (cli.NewRootCommand()) — the same entry point main.go wires via
// cli.Execute() — rather than calling core.ReconcileShipmentToShipped
// directly, so this test genuinely exercises the CLI boundary (flag
// parsing, the confirmation guard, JSON encoding, and exit-code mapping) in
// addition to the governed transaction underneath it.
func runReconcileShippedCLI(t *testing.T, root string, args ...string) (stdout string, stderr string, err error) {
	t.Helper()
	cmd := cli.NewRootCommand()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	allArgs := append([]string{"--cwd", root}, args...)
	cmd.SetArgs(allArgs)
	err = cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func decodeReconcileShippedResult(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result), "stdout must be the documented JSON result shape: %s", stdout)
	return result
}

// snapshotFileOrAbsent reads a file's bytes, treating "does not exist" as a
// distinct, comparable sentinel rather than an error, so a dry-run guard can
// assert "still absent" as easily as "byte-identical".
func snapshotFileOrAbsent(t *testing.T, path string) ([]byte, bool) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		require.NoError(t, err)
	}
	return raw, true
}

// --- Scenario 1: happy path ---------------------------------------------------

func TestShipmentReconcileShipped_HappyPath_RealMergeAndClosureEvidence(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-167-005-happy", "operator", "governed repair"), fx.confirmArg()...)
	stdout, stderr, err := runReconcileShippedCLI(t, fx.root, args...)
	require.NoError(t, err, "stderr: %s", stderr)

	result := decodeReconcileShippedResult(t, stdout)
	assert.Equal(t, "reconciled", result["outcome"])
	assert.Equal(t, fx.shipmentID, result["shipment_id"])
	assert.Equal(t, false, result["dry_run"])

	// Discriminating post-condition 1: the predecessor-status probe reads
	// archived_status DIRECTLY from the Markdown frontmatter and resolves to
	// "shipped".
	assert.Equal(t, "shipped", fx.readArchivedStatusFromMarkdown(t))

	// Discriminating post-condition 2: backlogit doctor's shipped-event
	// completeness audit reports NO missing_shipped_event finding for this
	// shipment.
	report, err := core.Doctor(context.Background(), fx.ws, &core.DoctorOptions{CheckShippedEventCompleteness: true})
	require.NoError(t, err)
	for _, finding := range report.Findings {
		if finding.Type == core.FindingMissingShippedEvent {
			assert.Fail(t, "unexpected missing_shipped_event finding after a real reconciliation", "%+v", finding)
		}
	}
}

// --- Scenario 2: unsupported pre-state ---------------------------------------

func TestShipmentReconcileShipped_Reject_UnsupportedPreState(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	// Mutate archived_status away from the only supported legacy value
	// ("active") directly in the Markdown frontmatter, simulating a
	// shipment archived from a status governed reconciliation does not
	// (yet) support.
	path := fx.archivedPath(t)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	mutated := strings.Replace(string(raw), "archived_status: active", "archived_status: done", 1)
	require.NotEqual(t, string(raw), mutated, "fixture must literally carry archived_status: active before mutation")
	require.NoError(t, os.WriteFile(path, []byte(mutated), 0o644))

	args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-167-005-prestate", "operator", "governed repair"), fx.confirmArg()...)
	_, _, err = runReconcileShippedCLI(t, fx.root, args...)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrUnsupportedLegacyPreState), "got: %v", err)

	var ee *cli.ExitError
	require.True(t, errors.As(err, &ee))
	assert.Equal(t, cli.ExitReconcileConflict, ee.Code)
}

// --- Scenario 3: non-terminal manifest member ---------------------------------

func TestShipmentReconcileShipped_Reject_NonTerminalMember(t *testing.T) {
	reconcileShippedRequireGit(t)
	// Leave the member at "active" (never done/accepted): a non-terminal
	// status the manifest-member terminality allowlist must reject.
	fx := newReconcileShippedFixture(t, string(models.StatusActive))

	args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-167-005-nonterminal", "operator", "governed repair"), fx.confirmArg()...)
	_, _, err := runReconcileShippedCLI(t, fx.root, args...)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrValidation), "got: %v", err)
	assert.Contains(t, err.Error(), fx.memberID)

	var ee *cli.ExitError
	require.True(t, errors.As(err, &ee))
	assert.Equal(t, cli.ExitReconcileConflict, ee.Code)
}

// --- Scenario 4: unbound/unresolvable evidence --------------------------------

func TestShipmentReconcileShipped_Reject_UnresolvableEvidence(t *testing.T) {
	reconcileShippedRequireGit(t)

	t.Run("closure_file_missing", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, string(models.StatusDone))
		missingRel := filepath.Join("docs", "closure", "does-not-exist.md")

		args := append(fx.args(fx.deliverySHA, missingRel, "idem-167-005-missing", "operator", "governed repair"), fx.confirmArg()...)
		_, _, err := runReconcileShippedCLI(t, fx.root, args...)
		require.Error(t, err)
		assert.True(t, errors.Is(err, blerrors.ErrShipmentReconcileEvidence), "got: %v", err)

		var ee *cli.ExitError
		require.True(t, errors.As(err, &ee))
		assert.Equal(t, cli.ExitReconcileConflict, ee.Code)
	})

	t.Run("closure_file_unparseable", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, string(models.StatusDone))
		unparseableRel := filepath.Join("docs", "closure", "unparseable.md")
		fx.writeClosureFile(t, unparseableRel, "This closure document never mentions any shipment section at all.\n")

		args := append(fx.args(fx.deliverySHA, unparseableRel, "idem-167-005-unparseable", "operator", "governed repair"), fx.confirmArg()...)
		_, _, err := runReconcileShippedCLI(t, fx.root, args...)
		require.Error(t, err)
		assert.True(t, errors.Is(err, blerrors.ErrShipmentReconcileEvidence), "got: %v", err)
	})
}

// --- Scenario 5: conflict (same idempotency key, different request identity) -
//
// DISCOVERED PRODUCTION BEHAVIOR (see the 167.005-T completion report):
// reconcileShipmentPhaseB (internal/core/shipment_reconcile_transaction.go,
// 167.008-T) calls validateShipmentReconcileShipmentPreState
// UNCONDITIONALLY, before classifyShipmentReconcileState (167.010-T) ever
// runs. That precondition gate requires archived_status to still be in
// supportedLegacyShippedPreStates={active} (167.014-T). Once the FIRST call
// below has committed archived_status:shipped, EVERY subsequent call —
// including this conflicting-identity replay — is rejected by that gate
// with ErrUnsupportedLegacyPreState, before the classifier's designed
// conflict detection (classifyShipmentReconcileEventPresent) ever runs.
// 167.014-T's own backlog task text scopes this precondition gate to
// "transaction Phase D" (branch 2d/reconciled only); the shipped code
// additionally applies the SAME full gate at the top of Phase B for every
// outcome, which is why the classifier-level ErrShipmentReconcileConflict
// sentinel is not observed here. This test accepts either sentinel (the
// originally-intended one, or the one the gate ordering actually produces)
// and instead pins the property that matters operationally: a
// conflicting-identity replay is ALWAYS rejected — never silently accepted,
// never a second write — and the CLI still maps it to the documented
// ExitReconcileConflict exit code either way.
func TestShipmentReconcileShipped_Reject_Conflict(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	firstArgs := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-167-005-conflict", "operator", "governed repair"), fx.confirmArg()...)
	_, stderr, err := runReconcileShippedCLI(t, fx.root, firstArgs...)
	require.NoError(t, err, "stderr: %s", stderr)

	archiveAfterFirst, _ := snapshotFileOrAbsent(t, fx.archivedPath(t))
	logAfterFirst, _ := snapshotFileOrAbsent(t, fx.itemLogPath())

	// Same idempotency key, but a different actor changes the request
	// identity digest (167.001-T's scalar-field digest is sensitive to
	// actor). With Phase B's identity/legacy-value gate ordering fixed
	// (167.008-T), this now reaches classifyShipmentReconcileState against
	// the already-shipped shipment and is correctly classified as
	// ShipmentReconcileOutcomeConflict (classifyShipmentReconcileEventPresent:
	// the single logged event's IdempotencyKey matches the request's, but
	// its RequestIdentityDigest does not) — a same-key, different-identity
	// replay, not a pre-state rejection.
	secondArgs := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-167-005-conflict", "someone-else", "governed repair"), fx.confirmArg()...)
	_, _, err = runReconcileShippedCLI(t, fx.root, secondArgs...)
	require.Error(t, err, "a same-key, different-identity replay must still be rejected, now via the conflict outcome rather than a pre-state gate")

	var ee *cli.ExitError
	require.True(t, errors.As(err, &ee), "a conflict outcome must carry the typed reconcile-conflict exit code")
	assert.Equal(t, cli.ExitReconcileConflict, ee.Code)
	assert.Contains(t, ee.Msg, "conflicts with a different already-recorded reconciliation",
		"the rejection must originate from classifyShipmentReconcileState's conflict outcome, not the legacy pre-state gate")

	archiveAfterSecond, _ := snapshotFileOrAbsent(t, fx.archivedPath(t))
	logAfterSecond, _ := snapshotFileOrAbsent(t, fx.itemLogPath())
	assert.Equal(t, archiveAfterFirst, archiveAfterSecond, "a rejected conflicting replay must not mutate the archive file")
	assert.Equal(t, logAfterFirst, logAfterSecond, "a rejected conflicting replay must not append a second event")
}

// --- Scenario 6: wrong-shipment SHA (unreachable from any trusted ref) --------

func TestShipmentReconcileShipped_Reject_UnreachableMergeSHA(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	// hiddenMergeSHA is a REAL merge commit (>=2 parents) but was never
	// merged into "main" — it is not reachable from any trusted ref at all.
	args := append(fx.args(fx.hiddenMergeSHA, fx.closureRel, "idem-167-005-unreachable", "operator", "governed repair"), fx.confirmArg()...)
	_, _, err := runReconcileShippedCLI(t, fx.root, args...)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrShipmentReconcileEvidence), "got: %v", err)
	assert.Contains(t, err.Error(), "not reachable", "the rejection must originate from trusted-ref reachability, not narrative parsing")
}

// --- Scenario 7: post-merge-closure SHA supplied as the delivery merge SHA ---

func TestShipmentReconcileShipped_Reject_ClosureRoleSHASuppliedAsMergeSHA(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	// closureSHA IS reachable from "main" and IS a true merge commit, but it
	// carries the "(post-merge closure)" role, not "(feature)". Supplying it
	// as --merge-sha must be rejected rather than silently accepted as the
	// delivery merge.
	args := append(fx.args(fx.closureSHA, fx.closureRel, "idem-167-005-wrongrole", "operator", "governed repair"), fx.confirmArg()...)
	_, _, err := runReconcileShippedCLI(t, fx.root, args...)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrShipmentReconcileEvidence), "got: %v", err)
	assert.Contains(t, err.Error(), "does not match merge_sha")
}

// --- Scenario 8: reachable non-merge (single-parent) commit as MergeSHA ------

func TestShipmentReconcileShipped_Reject_ReachableNonMergeCommit(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	// nonMergeSHA is reachable from "main" (it is main's own tip) but has
	// only one parent, so it is not a true merge commit.
	args := append(fx.args(fx.nonMergeSHA, fx.closureRel, "idem-167-005-nonmerge", "operator", "governed repair"), fx.confirmArg()...)
	_, _, err := runReconcileShippedCLI(t, fx.root, args...)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrShipmentReconcileEvidence), "got: %v", err)
	assert.Contains(t, err.Error(), "not a true merge commit")
}

// --- Scenario 9: idempotent replay (same key, same identity) -> no_op --------
//
// With Phase B's gate ordering fixed (167.008-T: the identity check runs
// unconditionally, but the legacy archived_status-value check is deferred
// until AFTER classification, and only re-applied on the fresh-repair
// ("reconciled") branch), a second call with the SAME idempotency key and the
// SAME request-identity-affecting fields now reaches
// classifyShipmentReconcileState against the already-shipped shipment and is
// classified via classifyShipmentReconcileEventPresent: exactly one logged
// event whose IdempotencyKey/RequestIdentityDigest match both the request and
// the persisted frontmatter markers, yielding ShipmentReconcileOutcomeNoOp.
// reconcileShipmentPhaseBNoOp then finds the durable event already present
// (branch 1b) and takes no further action — no second write, exit 0.
func TestShipmentReconcileShipped_IdempotentReplay_NoOp(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-167-005-replay", "operator", "governed repair"), fx.confirmArg()...)
	stdout, stderr, err := runReconcileShippedCLI(t, fx.root, args...)
	require.NoError(t, err, "stderr: %s", stderr)
	first := decodeReconcileShippedResult(t, stdout)
	require.Equal(t, "reconciled", first["outcome"])

	archiveBefore, _ := snapshotFileOrAbsent(t, fx.archivedPath(t))
	logBefore, _ := snapshotFileOrAbsent(t, fx.itemLogPath())

	// Identical request (same idempotency key, same identity-affecting
	// fields): must resolve to no_op, with no second write.
	secondStdout, secondStderr, err := runReconcileShippedCLI(t, fx.root, args...)
	require.NoError(t, err, "an idempotent replay must succeed as a no-op, not error: stderr: %s", secondStderr)
	second := decodeReconcileShippedResult(t, secondStdout)
	assert.Equal(t, "no_op", second["outcome"], "a same-key, same-identity replay must resolve to no_op")

	archiveAfter, _ := snapshotFileOrAbsent(t, fx.archivedPath(t))
	logAfter, _ := snapshotFileOrAbsent(t, fx.itemLogPath())
	assert.Equal(t, archiveBefore, archiveAfter, "a no_op replay must not rewrite the archive file")
	assert.Equal(t, logBefore, logAfter, "a no_op replay must not append a second event")
}

// --- Scenario 10: idempotent replay after the closure file is deleted -------
//
// Same as TestShipmentReconcileShipped_IdempotentReplay_NoOp above, but the
// closure-evidence file is deleted between the two calls. The replay must
// still resolve to no_op: classifyShipmentReconcileState's
// classifyShipmentReconcileEventPresent decision is made purely from the
// already-read item log and frontmatter (both re-read fresh under Phase B's
// held locks) plus the cheap, closure-path-free request identity digest
// (shipmentReconcileRequestIdentityDigest, 167.001-T) — it never re-reads the
// closure file. Phase C (the slow evidence gathering that DOES read the
// closure file) only ever runs on the fresh-repair ("reconciled") branch,
// which a no_op outcome never reaches, so the deleted closure file is simply
// never consulted for this decision.
func TestShipmentReconcileShipped_IdempotentReplay_AfterClosureFileDeleted(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-167-005-replay-deleted", "operator", "governed repair"), fx.confirmArg()...)
	stdout, stderr, err := runReconcileShippedCLI(t, fx.root, args...)
	require.NoError(t, err, "stderr: %s", stderr)
	first := decodeReconcileShippedResult(t, stdout)
	require.Equal(t, "reconciled", first["outcome"])

	// Delete the closure file: a replay decision must be reachable from the
	// item log + frontmatter + the request's scalar identity digest alone
	// (shipmentReconcileRequestIdentityDigest never reads the closure path),
	// with no dependency on the closure file still existing.
	closureAbs := filepath.Join(fx.root, filepath.FromSlash(fx.closureRel))
	require.NoError(t, os.Remove(closureAbs))
	_, statErr := os.Stat(closureAbs)
	require.True(t, os.IsNotExist(statErr), "closure file must actually be gone for this assertion to be meaningful")

	archiveBefore, _ := snapshotFileOrAbsent(t, fx.archivedPath(t))
	logBefore, _ := snapshotFileOrAbsent(t, fx.itemLogPath())

	secondStdout, secondStderr, err := runReconcileShippedCLI(t, fx.root, args...)
	require.NoError(t, err, "an idempotent replay must succeed as a no-op even with the closure file deleted: stderr: %s", secondStderr)
	second := decodeReconcileShippedResult(t, secondStdout)
	assert.Equal(t, "no_op", second["outcome"], "a same-key, same-identity replay must resolve to no_op regardless of closure file presence")

	archiveAfter, _ := snapshotFileOrAbsent(t, fx.archivedPath(t))
	logAfter, _ := snapshotFileOrAbsent(t, fx.itemLogPath())
	assert.Equal(t, archiveBefore, archiveAfter, "a no_op replay must not rewrite the archive file")
	assert.Equal(t, logBefore, logAfter, "a no_op replay must not append a second event")
}

// --- Scenario 11: dry run makes zero reconciliation writes -------------------

func TestShipmentReconcileShipped_DryRun_ZeroWrites(t *testing.T) {
	reconcileShippedRequireGit(t)
	fx := newReconcileShippedFixture(t, string(models.StatusDone))

	archiveBefore, archiveExistedBefore := snapshotFileOrAbsent(t, fx.archivedPath(t))
	logBefore, logExistedBefore := snapshotFileOrAbsent(t, fx.itemLogPath())

	args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-167-005-dryrun", "operator", "governed repair"), "--dry-run")
	stdout, stderr, err := runReconcileShippedCLI(t, fx.root, args...)
	require.NoError(t, err, "a dry run needs no --confirm and must succeed: stderr: %s", stderr)

	result := decodeReconcileShippedResult(t, stdout)
	assert.Equal(t, true, result["dry_run"])
	assert.Equal(t, "reconciled", result["outcome"], "dry run reports the outcome that WOULD occur")

	archiveAfter, archiveExistedAfter := snapshotFileOrAbsent(t, fx.archivedPath(t))
	logAfter, logExistedAfter := snapshotFileOrAbsent(t, fx.itemLogPath())
	assert.Equal(t, archiveExistedBefore, archiveExistedAfter)
	assert.Equal(t, archiveBefore, archiveAfter, "dry run must leave the archive file byte-identical")
	assert.Equal(t, logExistedBefore, logExistedAfter, "dry run must leave the item event log presence unchanged")
	assert.Equal(t, logBefore, logAfter, "dry run must leave the item event log byte-identical")

	// The predecessor-status probe must still show the PRE-reconciliation
	// value: dry run never flips archived_status.
	assert.Equal(t, "active", fx.readArchivedStatusFromMarkdown(t))
}
