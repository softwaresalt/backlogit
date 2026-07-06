package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core/gate"
)

// hasFinding reports whether the report contains a finding of the given type for id.
func hasGateEvidenceFinding(report *DoctorReport, id string) bool {
	for _, f := range report.Findings {
		if f.Type == FindingMissingGateEvidence && f.ArtifactID == id {
			return true
		}
	}
	return false
}

func TestDoctorGateEvidence_TerminalWithEvidence_NoWarning(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})
	ctx := context.Background()

	id := newActiveTask(t, ws)
	_, _, err := UpdateArtifactWithGate(ctx, ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	report, err := Doctor(ctx, ws, &DoctorOptions{CheckGateEvidence: true})
	require.NoError(t, err)
	assert.False(t, hasGateEvidenceFinding(report, id), "a terminal task WITH evidence must not warn")
}

func TestDoctorGateEvidence_TerminalWithoutEvidence_Warns(t *testing.T) {
	ws := newGateTestWorkspace(t)
	// gates configured (auto default); complete the task WITHOUT gate evidence.
	ctx := context.Background()
	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)

	report, err := Doctor(ctx, ws, &DoctorOptions{CheckGateEvidence: true})
	require.NoError(t, err, "advisory mode never returns an error (exit code unaffected)")
	assert.True(t, hasGateEvidenceFinding(report, id), "a terminal task WITHOUT evidence must warn while gates are configured")
}

func TestDoctorGateEvidence_GatesDisabled_NoWarning(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()
	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)

	// Disable gates: the advisory audit must not run.
	ws.gateConfig.Enabled = "false"
	report, err := Doctor(ctx, ws, &DoctorOptions{CheckGateEvidence: true})
	require.NoError(t, err)
	assert.False(t, hasGateEvidenceFinding(report, id), "no gate-evidence warning when gates are disabled")
}

func TestDoctorGateEvidence_NotEnabled_NoWarning(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()
	id := newActiveTask(t, ws)
	_, err := updateArtifactUngated(ctx, ws, id, map[string]any{"status": "done"})
	require.NoError(t, err)

	// CheckGateEvidence defaults off: opting out means no gate-evidence findings.
	report, err := Doctor(ctx, ws, &DoctorOptions{})
	require.NoError(t, err)
	assert.False(t, hasGateEvidenceFinding(report, id), "the audit is opt-in; off by default")
}
