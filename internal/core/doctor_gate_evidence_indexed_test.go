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

// TestDoctorGateEvidence_PositiveIndexConsulted pins the Q3.3 (083.005.004-ST)
// fast path: the advisory --check-gate-evidence audit trusts a POSITIVE
// gate_evidence projection row (passed/forced/forced_no_run) without re-scanning
// the item's logs. Item logs are append-only, so a positive row can never be
// stale-wrong, which makes the fast path safe.
func TestDoctorGateEvidence_PositiveIndexConsulted(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	// Item logs LACK evidence (ungated completion) but the projection says
	// passed -> the positive index must suppress the warning that a raw log-scan
	// would otherwise raise, proving the projection is the source consulted.
	idPassedRow := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, idPassedRow, map[string]any{"status": "done"})
	require.NoError(t, err)
	insertGateEvidenceRow(t, ws, idPassedRow, gateevidence.StatusPassed)

	report, err := Doctor(ctx, ws, &DoctorOptions{CheckGateEvidence: true})
	require.NoError(t, err, "advisory mode never returns an error (exit code unaffected)")

	assert.False(t, hasGateEvidenceFinding(report, idPassedRow),
		"a 'passed' projection row must suppress the warning (positive index consulted, not the log-scan)")
}

// TestDoctorGateEvidence_StaleMissingRowLogsWin pins the Q3.3 correctness
// contract that item logs remain the single source of truth: a "missing"
// projection row is NEVER trusted to override the logs, because it can be stale
// in the pass direction (the live completion path appends a pass to the log but
// does not touch this disposable projection between syncs). The audit therefore
// re-verifies against the authoritative log-scan for any non-positive row.
func TestDoctorGateEvidence_StaleMissingRowLogsWin(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()

	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	// Item B: logs HAVE passing evidence (real gate pass) but a STALE projection
	// row still says missing -> the audit must NOT warn, because the log-scan
	// (source of truth) confirms the pass.
	idStaleMissing := newActiveTask(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, idStaleMissing, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	insertGateEvidenceRow(t, ws, idStaleMissing, gateevidence.StatusMissing)

	// Item C: logs LACK evidence and the projection also says missing -> the
	// log-scan confirms the gap, so the audit must warn.
	idTrulyMissing := newActiveTask(t, ws)
	_, err = updateArtifactUngated(ctx, ws, idTrulyMissing, map[string]any{"status": "done"})
	require.NoError(t, err)
	insertGateEvidenceRow(t, ws, idTrulyMissing, gateevidence.StatusMissing)

	report, err := Doctor(ctx, ws, &DoctorOptions{CheckGateEvidence: true})
	require.NoError(t, err)

	assert.False(t, hasGateEvidenceFinding(report, idStaleMissing),
		"a stale 'missing' row must NOT override passing logs (logs are the source of truth)")
	assert.True(t, hasGateEvidenceFinding(report, idTrulyMissing),
		"a 'missing' row confirmed by the log-scan must warn")
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
