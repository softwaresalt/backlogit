package core

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	"github.com/softwaresalt/backlogit/internal/models"
)

// archiveMemberFromStatus archives a freshly created member and rewrites its
// persisted archived_status to the requested provenance value. ArchiveItem stamps
// archived_status with the pre-archive status (active), so patching the Markdown
// source is the deterministic way to exercise a member archived from an arbitrary
// status without depending on the transition matrix. archived_status is read from
// the Markdown source by archivedFromDescopeEligibleStatus, so this drives the real
// code path.
func archiveMemberFromStatus(t *testing.T, ctx context.Context, ws *Workspace, archivedStatus string) string {
	t.Helper()
	id := newActiveTask(t, ws)
	_, err := ArchiveItem(ctx, ws.DB, ws, id)
	require.NoError(t, err)
	require.Equal(t, "archived", statusOf(t, ws, id),
		"the member must be archived for this scenario")

	path, ferr := FindArtifactPath(ctx, ws, id)
	require.NoError(t, ferr)
	raw, rerr := os.ReadFile(path)
	require.NoError(t, rerr)
	require.Contains(t, string(raw), "archived_status: active",
		"precondition: a member archived from active carries archived_status: active")
	patched := strings.Replace(string(raw), "archived_status: active",
		"archived_status: "+archivedStatus, 1)
	require.NoError(t, os.WriteFile(path, []byte(patched), 0o644))
	return id
}

// TestValidateMemberGateEvidence_ArchivedFromShippedNotExempt pins the SHIP-2
// bug fix: `shipped` is a COMPLETION status, so a member archived from `shipped`
// with no valid gate evidence MUST still refuse. The prior predicate keyed the
// exemption on `!isTerminalReleaseStatus`, whose terminal set omitted `shipped`,
// wrongly exempting a shipped-then-archived member from the per-member F4 gate
// evidence requirement. Completions (done/accepted/shipped) are never a descope.
func TestValidateMemberGateEvidence_ArchivedFromShippedNotExempt(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := archiveMemberFromStatus(t, ctx, ws, "shipped")

	verr := validateMemberGateEvidence(ctx, ws, []string{id}, "")
	require.Error(t, verr,
		"an archived-from-shipped member (a completion) must NOT be exempt from gate evidence")
	assert.Contains(t, verr.Error(), "missing passing gate evidence")
}

// TestValidateMemberGateEvidence_ArchivedFromRejectedExempt pins that `rejected`
// is a NON-completion terminal — an item ended without shipping a deliverable —
// so a member archived from `rejected` is a genuine descope and is exempt from the
// per-member gate-evidence requirement, consistent with `abandoned`. The prior
// predicate classified `rejected` as terminal and refused it, permanently blocking
// a parent feature's ship when a descendant was rejected then archived.
func TestValidateMemberGateEvidence_ArchivedFromRejectedExempt(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := archiveMemberFromStatus(t, ctx, ws, "rejected")

	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{id}, ""),
		"an archived-from-rejected member (a non-completion terminal) must be exempt")
}

// TestValidateMemberGateEvidence_ArchivedFromAbandonedExempt is a regression pin:
// `abandoned` is a non-completion terminal and a member archived from it is a
// genuine descope, so it stays exempt across the SHIP-2 predicate reconciliation.
func TestValidateMemberGateEvidence_ArchivedFromAbandonedExempt(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := archiveMemberFromStatus(t, ctx, ws, "abandoned")

	require.NoError(t, validateMemberGateEvidence(ctx, ws, []string{id}, ""),
		"an archived-from-abandoned member (a non-completion terminal) must be exempt")
}

// TestIsDescopeEligibleStatus pins the full per-status classification of the
// descope-eligibility predicate. Descope-eligible = in-flight statuses (queued,
// active, blocked, review) plus non-completion terminals (abandoned, rejected).
// Completions (done, accepted, shipped) and the archived sink are NEVER eligible.
func TestIsDescopeEligibleStatus(t *testing.T) {
	tests := []struct {
		status models.ArtifactStatus
		want   bool
	}{
		{models.StatusQueued, true},
		{models.StatusActive, true},
		{models.StatusBlocked, true},
		{models.StatusReview, true},
		{models.StatusAbandoned, true},
		{models.StatusRejected, true},
		{models.StatusDone, false},
		{models.StatusAccepted, false},
		{models.StatusShipped, false},
		{models.StatusArchived, false},
		{models.ArtifactStatus("dne"), false},
		{models.ArtifactStatus(""), false},
	}
	for _, tc := range tests {
		t.Run(string(tc.status), func(t *testing.T) {
			assert.Equal(t, tc.want, isDescopeEligibleStatus(tc.status))
		})
	}
}
