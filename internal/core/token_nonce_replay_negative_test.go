package core_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// This file is the persistent RED negative harness for 170.010-T. The two
// tests reproduce the pre-hardening vulnerabilities and stay red until the
// 170.008-T (atomic external consume) and 170.009-T (externally authoritative
// required-auth) implementations land. Only the two helpers below are rewired
// by those tasks; the assertions are the stable contract.

// newReplayNegativeLedger returns the nonce consumer under test. Pre-hardening
// it is the in-workspace file ledger (170.003-T); 170.008-T rewires it to the
// injected external compare-and-consume provider.
func newReplayNegativeLedger(t *testing.T, workspaceLedgerPath string) *core.NonceLedger {
	t.Helper()
	ledger, err := core.OpenNonceLedger(workspaceLedgerPath)
	require.NoError(t, err)
	return ledger
}

// enforcementOutcome is the decision for a live reconcile invocation.
type enforcementOutcome string

const (
	enforcementDenied           enforcementOutcome = "denied"
	enforcementAuthenticated    enforcementOutcome = "authenticated"
	enforcementConfirmationOnly enforcementOutcome = "confirmation-only"
)

// resolveEnforcementForNegative decides enforcement for a workspace whose
// auth policy file lives at policyPath while externalRequired reports the
// externally asserted required-auth state. Pre-hardening there is no external
// assertion: a missing or malformed in-workspace policy falls back to the
// self-suppliable --confirm/TTY guard (internal/cli
// shipment_reconcile_shipped.go). 170.009-T rewires this to the
// externally authoritative resolver.
func resolveEnforcementForNegative(t *testing.T, externalRequired bool, policyPath string) enforcementOutcome {
	t.Helper()
	_ = externalRequired // pre-hardening code ignores the external assertion
	raw, err := os.ReadFile(policyPath)
	if err != nil || string(raw) != "required: true\n" {
		return enforcementConfirmationOnly
	}
	return enforcementAuthenticated
}

func TestTokenNonceReplayNegative_LedgerDeletionDoesNotPermitReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonces.jsonl")
	notAfter := time.Now().Add(5 * time.Minute)

	ledger := newReplayNegativeLedger(t, path)
	require.NoError(t, ledger.Record("00ff", notAfter))

	// A workspace writer deletes the in-workspace ledger; the token is still
	// inside its validity window.
	require.NoError(t, os.Remove(path))

	err := newReplayNegativeLedger(t, path).Record("00ff", notAfter)
	require.ErrorIs(t, err, core.ErrNonceReplayed,
		"deleting the in-workspace nonce ledger must not re-enable replay of a still-valid token")
}

func TestTokenNonceReplayNegative_LedgerTruncationDoesNotPermitReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonces.jsonl")
	notAfter := time.Now().Add(5 * time.Minute)

	ledger := newReplayNegativeLedger(t, path)
	require.NoError(t, ledger.Record("00ff", notAfter))
	require.NoError(t, os.Truncate(path, 0))

	err := newReplayNegativeLedger(t, path).Record("00ff", notAfter)
	require.ErrorIs(t, err, core.ErrNonceReplayed,
		"truncating the in-workspace nonce ledger must not re-enable replay of a still-valid token")
}

func TestTokenNonceReplayNegative_PolicyRemovalDoesNotDowngradeToConfirmation(t *testing.T) {
	dir := t.TempDir()
	policy := filepath.Join(dir, "auth-policy.yaml")

	// Required-auth is externally enabled; the workspace policy is absent.
	got := resolveEnforcementForNegative(t, true, policy)
	require.Equal(t, enforcementDenied, got,
		"removing the workspace auth policy must deny, never downgrade to confirmation-only")
}

func TestTokenNonceReplayNegative_PolicyMalformedDoesNotDowngradeToConfirmation(t *testing.T) {
	dir := t.TempDir()
	policy := filepath.Join(dir, "auth-policy.yaml")
	require.NoError(t, os.WriteFile(policy, []byte("required: [unterminated\n"), 0o600))

	got := resolveEnforcementForNegative(t, true, policy)
	require.Equal(t, enforcementDenied, got,
		"malforming the workspace auth policy must deny, never downgrade to confirmation-only")
}
