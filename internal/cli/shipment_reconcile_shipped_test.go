package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// This file is intentionally `package cli` (white-box), not `cli_test`: the
// confirmation-guard tests below need to override the unexported
// shipmentReconcileStdinIsInteractive seam so the TTY-prompt branch never
// engages against the real (possibly-interactive) test-process stdin,
// regardless of how `go test` itself is invoked.

// reconcileShippedSetupWorkspace creates a bare, initialized backlogit
// workspace directory (no shipment fixture) for tests that only need to
// exercise the CLI-boundary validation and confirmation guard, which both
// run BEFORE any workspace open / core call.
func reconcileShippedSetupWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	return root
}

// reconcileShippedBuildFixture builds a minimal archived-shipment-with-
// terminal-member fixture matching the shape the internal/core reconcile
// preconditions/transaction tests already exercise (one direct `done`
// member, wrapped in a shipment archived with archived_status=active). It
// uses only core's PUBLIC API (CreateArtifact/UpdateArtifact/CreateShipment/
// ClaimShipment/ArchiveItem), since this file cannot reach package core's
// unexported test-only fixture helpers.
func reconcileShippedBuildFixture(t *testing.T, root string) string {
	t.Helper()
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	feature, err := core.CreateArtifact(ctx, ws, "Reconcile CLI feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(ctx, ws, "Reconcile CLI member", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": "done"})
	require.NoError(t, err)

	shipment, err := core.CreateShipment(ctx, ws, "Reconcile CLI shipment", []string{member.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	_, err = core.ArchiveItem(ctx, ws.DB, ws, shipment.ID)
	require.NoError(t, err)
	return shipment.ID
}

// reconcileShippedBuildUnarchivedFixture builds a shipment that is CLAIMED
// (status=active) but never archived — an intentionally-wrong pre-state for
// governed reconciliation (validateShipmentReconcileShipmentPreState requires
// status=archived), reachable without any git repository.
func reconcileShippedBuildUnarchivedFixture(t *testing.T, root string) string {
	t.Helper()
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	feature, err := core.CreateArtifact(ctx, ws, "Reconcile CLI unarchived feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(ctx, ws, "Reconcile CLI unarchived member", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": "done"})
	require.NoError(t, err)

	shipment, err := core.CreateShipment(ctx, ws, "Reconcile CLI unarchived shipment", []string{member.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	return shipment.ID
}

// reconcileShippedArchivePath returns the on-disk path of the archived
// shipment's Markdown file, used to prove a dry-run performed zero writes by
// comparing raw file bytes before/after.
func reconcileShippedArchivePath(t *testing.T, root, shipmentID string) string {
	t.Helper()
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()
	path, err := core.FindArtifactPath(ctx, ws, shipmentID)
	require.NoError(t, err)
	return path
}

// reconcileShippedRunCmd executes a fresh root command against root and
// returns (stdout, stderr, execution error). It never touches os.Stdin/
// os.Stdout: SetOut/SetErr/SetIn all point at in-memory buffers, per the
// existing shipment CLI test convention (shipment_add_test.go).
func reconcileShippedRunCmd(t *testing.T, root string, args ...string) (string, string, error) {
	t.Helper()
	cmd := NewRootCommand()
	out := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetIn(new(bytes.Buffer))
	cmd.SetArgs(append([]string{"--cwd", root}, args...))
	err := cmd.Execute()
	return out.String(), errBuf.String(), err
}

// reconcileShippedForceNonInteractiveStdin overrides the
// shipmentReconcileStdinIsInteractive seam to always report non-interactive,
// restoring the previous value on test cleanup. Every test below needs a
// deterministic non-TTY seam regardless of how the test binary's own stdin
// happens to be connected.
func reconcileShippedForceNonInteractiveStdin(t *testing.T) {
	t.Helper()
	previous := shipmentReconcileStdinIsInteractive
	shipmentReconcileStdinIsInteractive = func() bool { return false }
	t.Cleanup(func() { shipmentReconcileStdinIsInteractive = previous })
}

const reconcileShippedValidMergeSHA = "0123456789abcdef0123456789abcdef01234567"

// reconcileShippedValidArgs returns a full set of required-flag arguments
// (all trimmed-non-empty and shape-valid) for shipmentID, so a test can
// override/omit exactly the flag(s) it wants to exercise.
func reconcileShippedValidArgs(shipmentID string) []string {
	return []string{
		"shipment", "reconcile-shipped", shipmentID,
		"--reason", "governed repair",
		"--actor", "operator",
		"--idempotency-key", "idem-cli-1",
		"--merge-sha", reconcileShippedValidMergeSHA,
		"--closure-evidence", filepath.Join("docs", "closure", "cli-test.md"),
	}
}

// --- 1. Required-flag omission ---------------------------------------------

func TestShipmentReconcileShipped_RequiredFlagOmission(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)

	allFlags := map[string]string{
		"--reason":           "governed repair",
		"--actor":            "operator",
		"--idempotency-key":  "idem-cli-1",
		"--merge-sha":        reconcileShippedValidMergeSHA,
		"--closure-evidence": filepath.Join("docs", "closure", "cli-test.md"),
	}

	for omit := range allFlags {
		t.Run("omit_"+strings.TrimPrefix(omit, "--"), func(t *testing.T) {
			args := []string{"shipment", "reconcile-shipped", "000-S"}
			for flag, value := range allFlags {
				if flag == omit {
					continue
				}
				args = append(args, flag, value)
			}
			_, _, err := reconcileShippedRunCmd(t, root, args...)
			require.Error(t, err, "omitting required flag %s must fail", omit)
			assert.Contains(t, err.Error(), "required flag", "error should name the missing required flag")
		})
	}
}

// --- 2. Empty-string and whitespace-only values are rejected ----------------

func TestShipmentReconcileShipped_WhitespaceOnlyFieldsRejected(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)

	fields := []string{"--reason", "--actor", "--idempotency-key", "--merge-sha", "--closure-evidence"}
	values := map[string]string{"empty": "", "spaces": "   ", "tabs": "\t\t", "newline": "\n"}

	for _, field := range fields {
		for name, blank := range values {
			t.Run(strings.TrimPrefix(field, "--")+"_"+name, func(t *testing.T) {
				args := reconcileShippedValidArgs("000-S")
				args = reconcileShippedOverrideFlag(args, field, blank)
				_, _, err := reconcileShippedRunCmd(t, root, args...)
				require.Error(t, err, "%s=%q must be rejected", field, blank)
				assert.True(t, errors.Is(err, corerrors.ErrValidation),
					"whitespace-only %s must surface the validation sentinel, got: %v", field, err)
			})
		}
	}
}

// reconcileShippedOverrideFlag replaces the value following an existing
// `--flag` occurrence in args with newValue (used to blank out one required
// field while leaving the rest of reconcileShippedValidArgs untouched).
func reconcileShippedOverrideFlag(args []string, flag, newValue string) []string {
	out := make([]string, len(args))
	copy(out, args)
	for i, a := range out {
		if a == flag && i+1 < len(out) {
			out[i+1] = newValue
			return out
		}
	}
	return append(out, flag, newValue)
}

// --- 3. No --confirm, non-TTY stdin => denied -------------------------------

func TestShipmentReconcileShipped_NoConfirm_NonInteractive_Denied(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)

	_, stderr, err := reconcileShippedRunCmd(t, root, reconcileShippedValidArgs("000-S")...)
	require.Error(t, err)
	assert.True(t, errors.Is(err, corerrors.ErrConfirmationRequired),
		"missing --confirm with non-interactive stdin must deny via ErrConfirmationRequired, got: %v", err)

	var ee *ExitError
	require.True(t, errors.As(err, &ee), "expected *ExitError so main resolves a distinct non-zero exit code")
	assert.Equal(t, ExitConfirmationRequired, ee.Code)
	assert.NotEqual(t, 0, ExitCodeFor(err))
	assert.Contains(t, stderr, "confirmation", "stderr should explain the confirmation denial")
}

// --- 4. --confirm token that does not match the shipment phrase => denied --

func TestShipmentReconcileShipped_ConfirmTokenMismatch_Denied(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)

	args := append(reconcileShippedValidArgs("048-S"), "--confirm", "reconcile-shipped 999-S")
	_, _, err := reconcileShippedRunCmd(t, root, args...)
	require.Error(t, err)
	assert.True(t, errors.Is(err, corerrors.ErrConfirmationRequired),
		"a --confirm token for a DIFFERENT shipment must not confirm this one, got: %v", err)
	assert.Equal(t, ExitConfirmationRequired, ExitCodeFor(err))
}

// --- 5. Matching --confirm token permits the mutation to proceed -----------

func TestShipmentReconcileShipped_ConfirmTokenMatches_PermitsCoreCall(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)

	// No fixture is created for this shipment ID: the point of this test is
	// only to prove the confirmation gate is NOT what stops execution once
	// the token matches. core.ReconcileShipmentToShipped is expected to fail
	// deterministically for a nonexistent shipment — as long as that failure
	// is NOT ErrConfirmationRequired, the gate demonstrably let the request
	// through to the core call.
	args := append(reconcileShippedValidArgs("999-NOPE"), "--confirm", "reconcile-shipped 999-NOPE")
	_, _, err := reconcileShippedRunCmd(t, root, args...)
	require.Error(t, err, "a nonexistent shipment id must still fail, just not on the confirmation gate")
	assert.False(t, errors.Is(err, corerrors.ErrConfirmationRequired),
		"a matching --confirm token must not be blocked by the confirmation gate, got: %v", err)
	assert.Contains(t, err.Error(), "reconcile shipment to shipped",
		"the error must originate from the core call, proving the CLI reached it")
}

// --- 6. --dry-run needs no confirmation and performs zero writes -----------

func TestShipmentReconcileShipped_DryRun_NoConfirmationNoWrites(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)
	shipmentID := reconcileShippedBuildFixture(t, root)

	archivePath := reconcileShippedArchivePath(t, root, shipmentID)
	before, err := os.ReadFile(archivePath)
	require.NoError(t, err)

	args := append(reconcileShippedValidArgs(shipmentID), "--dry-run")
	stdout, _, err := reconcileShippedRunCmd(t, root, args...)
	require.NoError(t, err, "a dry run against a valid fixture must succeed (exit 0)")
	assert.Contains(t, stdout, `"dry_run": true`)
	assert.Contains(t, stdout, shipmentID)

	after, err := os.ReadFile(archivePath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "dry-run must make zero reconciliation writes to the archive file")
}

// --- 7. Success path (best-effort) ------------------------------------------
//
// A fully "reconciled" outcome requires prepareShipmentReconcileEvidence
// (167.001-T) to verify the supplied merge SHA is reachable from a
// configured trusted git ref AND that the closure-evidence file narrates a
// matching merge — both require a real git repository and real evidence
// files, impractical to construct in this CLI-layer unit test. This test
// intentionally settles for a legitimate business-logic rejection at the
// evidence-verification boundary instead (per the task's own documented
// allowance). What it DOES prove end-to-end: CLI validation passed, the
// confirmation gate was satisfied, and the core call ran the full
// precondition chain (shipment pre-state, classifier, manifest members) up
// to evidence verification.
func TestShipmentReconcileShipped_SuccessPath_BestEffort(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)
	shipmentID := reconcileShippedBuildFixture(t, root)

	args := append(reconcileShippedValidArgs(shipmentID), "--confirm", "reconcile-shipped "+shipmentID)
	_, _, err := reconcileShippedRunCmd(t, root, args...)
	require.Error(t, err, "no real git repository/trusted refs are available in this fixture, so evidence verification is expected to reject")
	assert.True(t, errors.Is(err, corerrors.ErrShipmentReconcileEvidence),
		"expected the legitimate business-logic evidence-verification rejection, got: %v", err)

	var ee *ExitError
	require.True(t, errors.As(err, &ee), "an evidence-verification reject must carry a distinct typed exit code")
	assert.Equal(t, ExitReconcileConflict, ee.Code)
}

// --- 8. Reject exit code path (non-generic) ---------------------------------

func TestShipmentReconcileShipped_RejectExitCode_UnarchivedPreState(t *testing.T) {
	reconcileShippedForceNonInteractiveStdin(t)
	root := reconcileShippedSetupWorkspace(t)
	shipmentID := reconcileShippedBuildUnarchivedFixture(t, root)

	args := append(reconcileShippedValidArgs(shipmentID), "--confirm", "reconcile-shipped "+shipmentID)
	_, _, err := reconcileShippedRunCmd(t, root, args...)
	require.Error(t, err)
	// A shipment that was never archived fails at the earliest precondition
	// check (its file does not resolve under the archive directory at all),
	// before the shipment.Status==archived / archived_status allowlist check
	// even runs — so this surfaces the general precondition ErrValidation
	// sentinel rather than the more specific ErrUnsupportedLegacyPreState.
	// Both are precondition-reject classes mapped to the same exit code.
	assert.True(t, errors.Is(err, corerrors.ErrValidation),
		"a shipment that was never archived must be rejected as a precondition validation failure, got: %v", err)

	code := ExitCodeFor(err)
	assert.Equal(t, ExitReconcileConflict, code)
	assert.NotEqual(t, 1, code, "a typed reject must not collapse to the generic exit code 1")
}

// --- Registration ------------------------------------------------------------

func TestShipmentReconcileShipped_RegisteredInShipmentGroup(t *testing.T) {
	cmd := NewShipmentCmd()
	names := make([]string, 0)
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "reconcile-shipped")
}
