package core

import (
	"context"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// --- gate seams (package core copies; the gate package's own fakes are unexported
// to that package) -------------------------------------------------------------

type fakeGateRunner struct {
	res     gate.GateResult
	err     error
	lastCmd []string
}

func (f *fakeGateRunner) Run(_ context.Context, args []string, _ string, _ []string) (gate.GateResult, error) {
	f.lastCmd = args
	return f.res, f.err
}

// fakeGitAllOK verifies every ref, so base resolution lands on origin/HEAD.
type fakeGitAllOK struct{}

func (fakeGitAllOK) Verify(_ context.Context, _ string) (bool, error) { return true, nil }

type fakeVersion struct {
	v   string
	err error
}

func (f fakeVersion) Version(_ context.Context) (string, error) { return f.v, f.err }

// --- workspace with an injected gate broker ------------------------------------

func newGateTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	tmp := t.TempDir()
	backlogDir := filepath.Join(tmp, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))
	ws, err := NewWorkspace(context.Background(), tmp)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return ws
}

// injectBroker wires a fake-seam broker and a normalized gate config onto ws.
func injectBroker(ws *Workspace, enabled gate.EnabledMode, runner gate.GateRunner, version gate.VersionRunner) {
	ws.GateBroker = &gate.Broker{
		Runner:         runner,
		Git:            fakeGitAllOK{},
		Version:        version,
		Enabled:        enabled,
		TimeoutSeconds: 30,
	}
	cfg := config.PreTaskCompletionGateConfig{Enabled: string(enabled)}
	cfg.Normalize()
	ws.gateConfig = cfg
}

// newActiveTask creates a feature+task and moves the task to active (ungated).
func newActiveTask(t *testing.T, ws *Workspace) string {
	t.Helper()
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Gate feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Gate task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	// Move to active WITHOUT the gate (queued->active is not a terminal entry).
	_, err = UpdateArtifact(ctx, ws, task.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	return task.ID
}

func statusOf(t *testing.T, ws *Workspace, id string) string {
	t.Helper()
	a, err := findArtifact(context.Background(), ws, id)
	require.NoError(t, err)
	return string(a.Status)
}

const okVersion = "1.4.7"

func TestGate_NoBroker_AllowsWithoutRunning(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	// Simulate a bare workspace (no broker wired): the gate is skipped entirely.
	ws.GateBroker = nil
	require.Nil(t, ws.GateBroker)
	art, outcome, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	require.Nil(t, outcome, "no gate outcome when the gate does not run")
	assert.Equal(t, "done", string(art.Status))
}

func TestGate_Pass_CompletesToDone(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	art, outcome, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	require.NotNil(t, outcome)
	assert.Equal(t, "passed", outcome.Outcome)
	assert.True(t, outcome.StateChanged)
	assert.Equal(t, "done", string(art.Status))
	assert.Equal(t, "done", statusOf(t, ws, id))
	// The gate actually ran with an argv array containing --task.
	assert.Contains(t, runner.lastCmd, "--task")
}

func TestGate_Block_RefusesAndRetainsStatus(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	// exit 1, repeated_failure below threshold -> plain block.
	report := `{"repeated_failure":{"count":1,"threshold":3,"reached":false,"action":"block"}}`
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 1, Stdout: []byte(report)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	art, outcome, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err)
	assert.Nil(t, art, "no artifact returned on a plain block")

	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "want *GateBlockedError, got %T", err)
	assert.Equal(t, "blocked", blocked.Outcome)
	assert.False(t, blocked.StateChanged)
	assert.Equal(t, "active", blocked.OldStatus)
	assert.Equal(t, "active", blocked.NewStatus, "item retains its prior status on a plain block")
	require.NotNil(t, outcome)
	assert.Equal(t, "blocked", outcome.Outcome)
	// File + DB unchanged: still active.
	assert.Equal(t, "active", statusOf(t, ws, id))
}

func TestGate_RepeatedFailure_Block_RequeuesToQueued(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	report := `{"repeated_failure":{"count":3,"threshold":3,"reached":true,"action":"block"}}`
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 1, Stdout: []byte(report)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	art, outcome, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err)
	require.NotNil(t, art, "redirect returns the moved artifact")
	assert.Equal(t, "queued", string(art.Status))

	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked))
	assert.Equal(t, "requeued", blocked.Outcome)
	assert.True(t, blocked.StateChanged)
	assert.Equal(t, "queued", blocked.NewStatus)
	require.NotNil(t, outcome)
	assert.Equal(t, "requeued", outcome.Outcome)
	assert.Equal(t, "queued", statusOf(t, ws, id))
}

func TestGate_RepeatedFailure_Escalate_MovesToBlocked(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	report := `{"repeated_failure":{"count":5,"threshold":3,"reached":true,"action":"escalate"}}`
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 1, Stdout: []byte(report)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	art, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err)
	require.NotNil(t, art)
	assert.Equal(t, "blocked", string(art.Status))

	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked))
	assert.Equal(t, "escalated", blocked.Outcome)
	assert.Equal(t, "blocked", statusOf(t, ws, id))
}

func TestGate_MissingBinary_StrictEnabled_ConfigError(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	notFound := fmt.Errorf("resolve autoharness: %w", bkerrors.ErrGateBinaryNotFound)
	// enabled:true + unresolvable binary -> setup-class refusal at probe.
	injectBroker(ws, gate.EnabledTrue, &fakeGateRunner{}, fakeVersion{err: notFound})

	art, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err)
	assert.Nil(t, art)
	var ge *bkerrors.GateError
	require.True(t, stderrors.As(err, &ge), "want *GateError, got %T", err)
	assert.Equal(t, "setup", ge.Class)
	assert.True(t, stderrors.Is(err, bkerrors.ErrGateSetup))
	// Item untouched.
	assert.Equal(t, "active", statusOf(t, ws, id))
}

func TestGate_MissingBinary_Auto_FailsOpen(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	notFound := fmt.Errorf("resolve autoharness: %w", bkerrors.ErrGateBinaryNotFound)
	// enabled:auto + unresolvable binary -> proceed (fail open).
	injectBroker(ws, gate.EnabledAuto, &fakeGateRunner{}, fakeVersion{err: notFound})

	art, outcome, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	require.NotNil(t, outcome)
	assert.Equal(t, "passed", outcome.Outcome)
	assert.False(t, outcome.Ran, "a fail-open under auto did not run the gate")
	assert.Equal(t, "done", string(art.Status))
}

func TestGate_ExitTwo_ConfigError(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 2, Stderr: []byte("bad gate config")}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	art, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err)
	assert.Nil(t, art)
	var ge *bkerrors.GateError
	require.True(t, stderrors.As(err, &ge))
	assert.Equal(t, "config", ge.Class)
	assert.Equal(t, "active", statusOf(t, ws, id))
}

func TestGate_ForceFromNonCLI_Rejected(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"},
		TransitionOptions{Force: true, ForceReason: "x", ForceSource: ForceSourceNone})
	require.Error(t, err)
	var ge *bkerrors.GateError
	require.True(t, stderrors.As(err, &ge))
	assert.Equal(t, "config", ge.Class)
	// Never ran the gate; item unchanged.
	assert.Empty(t, runner.lastCmd)
	assert.Equal(t, "active", statusOf(t, ws, id))
}

func TestGate_NonTaskType_NotGated(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Plain feature", "feature")
	require.NoError(t, err)
	_, err = UpdateArtifact(ctx, ws, feat.ID, map[string]any{"status": "active"})
	require.NoError(t, err)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 1}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	// A feature completion is not gated even with exit 1 configured.
	_, outcome, err := UpdateArtifactWithGate(ctx, ws, feat.ID, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	assert.Nil(t, outcome)
	assert.Empty(t, runner.lastCmd, "gate must not run for a feature")
}

// 144.001-T (U1) RED harness: UpdateArtifactWithGate must refuse a shipment →
// shipped transition unconditionally, even when the formal gate is OFF.
// These tests compile immediately (sentinel defined in errors.go) but FAIL until
// U2 removes the formalGateEnforced() condition from the shipment guard.

func TestUpdateArtifactWithGate_ShipmentToShipped_Refused_GateOff(t *testing.T) {
	ws := newGateTestWorkspace(t)
	// Explicitly no formal gate config — formalGateEnforced() is false.
	ws.Config.FormalGate = nil
	ctx := context.Background()

	shipment, err := CreateShipment(ctx, ws, "Guard-test shipment", nil)
	require.NoError(t, err)
	require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentActive))

	_, _, err = UpdateArtifactWithGate(ctx, ws, shipment.ID,
		map[string]any{"status": string(ShipmentShipped)},
		TransitionOptions{})
	require.Error(t, err, "shipment must not be moved to shipped via the generic gate path (gate OFF)")
	assert.True(t, stderrors.Is(err, bkerrors.ErrShipmentShippedRequiresEnvelope),
		"want ErrShipmentShippedRequiresEnvelope; got %v", err)
}

func TestUpdateArtifactWithGate_ShipmentToNonShipped_Unaffected(t *testing.T) {
	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = nil
	ctx := context.Background()

	shipment, err := CreateShipment(ctx, ws, "Transition-guard non-shipped shipment", nil)
	require.NoError(t, err)

	// queued→active must not be refused by guard 1.
	_, _, err = UpdateArtifactWithGate(ctx, ws, shipment.ID,
		map[string]any{"status": string(ShipmentActive)},
		TransitionOptions{})
	require.NoError(t, err, "non-shipped transition must not be refused by guard 1")
}
