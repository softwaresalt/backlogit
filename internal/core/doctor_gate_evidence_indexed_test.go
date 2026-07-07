package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
)

// insertGateEvidenceRow writes a synthetic projection row so the test can prove
// which source (indexed projection vs. authoritative log-scan) the advisory
// doctor audit consults.
func insertGateEvidenceRow(t *testing.T, ws *Workspace, itemID, status string) {
	t.Helper()
	_, err := ws.DB.ExecContext(context.Background(),
		`INSERT OR REPLACE INTO gate_evidence (item_id, gate_status, evidence_sha, head_sha) VALUES (?, ?, ?, ?)`,
		itemID, status, "", "")
	require.NoError(t, err)
}

// TestDoctorGateEvidence_IndexedProjection_Consulted pins Q3.3 (083.005.004-ST):
// the advisory --check-gate-evidence audit reads the derived gate_evidence
// projection when a row exists (indexed path), rather than scanning each item's
// logs. Two divergence cases prove the projection is the source consulted:
//   - a "passed" row SUPPRESSES the warning even though the item's logs carry no
//     evidence;
//   - a "missing" row WARNS even though the item's logs carry passing evidence.
func TestDoctorGateEvidence_IndexedProjection_Consulted(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	// Item A: logs LACK evidence (ungated completion) but the projection says
	// passed -> the indexed path must suppress the warning.
	idPassedRow := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, idPassedRow, map[string]any{"status": "done"})
	require.NoError(t, err)
	insertGateEvidenceRow(t, ws, idPassedRow, gateevidence.StatusPassed)

	// Item B: logs HAVE passing evidence (real gate pass) but the projection says
	// missing -> the indexed path must warn, overriding the log-scan.
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})
	idMissingRow := newActiveTask(t, ws)
	_, _, err = UpdateArtifactWithGate(ctx, ws, idMissingRow, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	insertGateEvidenceRow(t, ws, idMissingRow, gateevidence.StatusMissing)

	report, err := Doctor(ctx, ws, &DoctorOptions{CheckGateEvidence: true})
	require.NoError(t, err, "advisory mode never returns an error (exit code unaffected)")

	assert.False(t, hasGateEvidenceFinding(report, idPassedRow),
		"a 'passed' projection row must suppress the warning (indexed path consulted, not the log-scan)")
	assert.True(t, hasGateEvidenceFinding(report, idMissingRow),
		"a 'missing' projection row must warn even though the logs carry evidence (indexed path consulted)")
}

// TestDoctorGateEvidence_AbsentRow_FallsBackToLogScan pins the Q3.3 correctness
// fallback: when an item is ABSENT from the projection (never gated, or gated
// since the last sync), the audit falls back to the authoritative log-scan rather
// than no-op, so a stale/absent projection never silently stops auditing.
func TestDoctorGateEvidence_AbsentRow_FallsBackToLogScan(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	// No projection rows are written at all (empty index, e.g. pre-first-sync).
	// A terminal task WITHOUT evidence must still be flagged via the log-scan.
	idNoEvidence := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, idNoEvidence, map[string]any{"status": "done"})
	require.NoError(t, err)

	// A terminal task WITH evidence must NOT be flagged (no false negative).
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})
	idWithEvidence := newActiveTask(t, ws)
	_, _, err = UpdateArtifactWithGate(ctx, ws, idWithEvidence, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	report, err := Doctor(ctx, ws, &DoctorOptions{CheckGateEvidence: true})
	require.NoError(t, err)

	assert.True(t, hasGateEvidenceFinding(report, idNoEvidence),
		"absent projection row must fall back to the log-scan and flag a missing-evidence terminal item")
	assert.False(t, hasGateEvidenceFinding(report, idWithEvidence),
		"absent projection row must fall back to the log-scan and NOT flag an item that has evidence")
}
