package core

import (
	"context"
	stderrors "errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core/gate"
	"github.com/softwaresalt/backlogit/internal/events"
)

// eventsFor reads all logged events for an item under the workspace logs root.
func eventsFor(t *testing.T, ws *Workspace, id string) []events.Event {
	t.Helper()
	evs, err := events.ReadAllEvents(context.Background(), WorkspaceLogsRoot(ws.RootPath), id)
	require.NoError(t, err)
	return evs
}

// findEvent returns the first event of the given type, or nil.
func findEvent(evs []events.Event, eventType string) *events.Event {
	for i := range evs {
		if evs[i].EventType == eventType {
			return &evs[i]
		}
	}
	return nil
}

func TestGateEvidence_PassEmitsPassedEvent(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	ev := findEvent(eventsFor(t, ws, id), EventGatePassed)
	require.NotNil(t, ev, "a passing completion must log %s", EventGatePassed)
	assert.Equal(t, "passed", ev.Delta["outcome"])
	assert.Equal(t, "active", ev.Delta["old_status"])
	assert.Equal(t, "done", ev.Delta["new_status"])
	assert.Equal(t, true, ev.Delta["state_changed"])
}

func TestGateEvidence_BlockEmitsBlockedEvent(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	report := `{"repeated_failure":{"count":1,"threshold":3,"reached":false,"action":"block"}}`
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 1, Stdout: []byte(report)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err)

	ev := findEvent(eventsFor(t, ws, id), EventGateBlocked)
	require.NotNil(t, ev, "a blocked completion must log %s", EventGateBlocked)
	assert.Equal(t, "blocked", ev.Delta["outcome"])
	assert.Equal(t, false, ev.Delta["state_changed"])
}

func TestGateEvidence_RequeueEmitsRequeuedEvent(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	report := `{"repeated_failure":{"count":3,"threshold":3,"reached":true,"action":"block"}}`
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 1, Stdout: []byte(report)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err)

	ev := findEvent(eventsFor(t, ws, id), EventGateRequeued)
	require.NotNil(t, ev, "a requeue must log %s", EventGateRequeued)
	assert.Equal(t, "requeued", ev.Delta["outcome"])
	assert.Equal(t, "queued", ev.Delta["new_status"])
	rf, ok := ev.Delta["repeated_failure"].(map[string]any)
	require.True(t, ok, "requeue evidence must carry the repeated_failure object")
	assert.Equal(t, "block", rf["action"])
}

func TestGateEvidence_ErrorEmitsErrorEvent(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	// exit 2 -> config-class error refusal.
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 2, Stderr: []byte("bad gate config")}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err)

	ev := findEvent(eventsFor(t, ws, id), EventGateError)
	require.NotNil(t, ev, "a config-error refusal must log %s so it is traceable", EventGateError)
	assert.Equal(t, "config", ev.Delta["class"])
}

// TestGateEvidence_NoFrontmatterMutation asserts gate evidence is logs-only: the
// artifact's persisted markdown never carries a gate evidence event type (Q3).
func TestGateEvidence_NoFrontmatterMutation(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	path, err := FindArtifactPath(context.Background(), ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(path) //nolint:gosec // test-controlled path
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "pre_task_completion_gate",
		"gate evidence must live in item logs, never in the artifact frontmatter/body")
}

// TestGateEvidence_RequiredRollback asserts that under evidence_required a failed
// evidence append refuses the completion and leaves the item's state unchanged.
func TestGateEvidence_RequiredRollback(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})
	require.True(t, ws.gateConfig.EvidenceRequiredValue(), "evidence_required must default true")

	// Inject an appender that always fails for the passed event.
	ws.gateEvidenceAppend = func(_ context.Context, _ *Workspace, _, eventType string, _ map[string]any) error {
		if eventType == EventGatePassed {
			return stderrors.New("injected evidence append failure")
		}
		return nil
	}

	art, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err, "evidence_required + append failure must refuse the completion")
	assert.Nil(t, art)
	assert.Contains(t, err.Error(), "refusing completion")
	// The durable write never happened: the item is still active.
	assert.Equal(t, "active", statusOf(t, ws, id))
}

// TestGateEvidence_AppendUnderLock asserts the evidence append happens while the
// per-item task lock is held: a non-blocking lock attempt from inside the
// appender observes the lock as busy.
func TestGateEvidence_AppendUnderLock(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	path, err := FindArtifactPath(context.Background(), ws, id)
	require.NoError(t, err)

	var observedBusy bool
	ws.gateEvidenceAppend = func(ctx context.Context, w *Workspace, itemID, eventType string, delta map[string]any) error {
		if eventType == EventGatePassed {
			// A non-blocking acquire must fail because runGatedCompletion holds
			// the lock across the entire gate + evidence + write sequence.
			unlock, lockErr := lockTaskFile(path)
			if lockErr != nil {
				observedBusy = true
			} else {
				_ = unlock()
			}
		}
		return appendItemEventErr(ctx, w, itemID, eventType, delta)
	}

	_, _, err = UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)
	assert.True(t, observedBusy, "evidence append must occur under the held task lock")
}

// TestGateEvidence_ForcedEmitsForcedEvent asserts a forced pass logs both the
// passed and forced events, and the forced event carries the operator reason.
func TestGateEvidence_ForcedEmitsForcedEvent(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	// exit 1 would normally block; force overrides to a pass (autoharness audits
	// separately, backlogit records the forced event).
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"},
		TransitionOptions{Force: true, ForceReason: "operator override for hotfix", ForceSource: ForceSourceCLI})
	require.NoError(t, err)

	evs := eventsFor(t, ws, id)
	forced := findEvent(evs, EventGateForced)
	require.NotNil(t, forced, "a forced completion must log %s", EventGateForced)
	assert.Equal(t, true, forced.Delta["forced"])
	reason, _ := forced.Delta["force_reason"].(string)
	assert.True(t, strings.Contains(reason, "operator override"), "forced event must record the operator reason")
}

// TestGateEvidence_ForcedRequiredAppendFailure_Refuses asserts that under
// evidence_required a failed forced-audit append refuses the completion rather
// than silently persisting a forced transition with no audit trail (parity with
// the pass-evidence path). Force is the operator break-glass; its audit record is
// the whole point of forcing.
func TestGateEvidence_ForcedRequiredAppendFailure_Refuses(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})
	require.True(t, ws.gateConfig.EvidenceRequiredValue(), "evidence_required must default true")

	// The passed-evidence append succeeds; the forced-audit append fails.
	ws.gateEvidenceAppend = func(ctx context.Context, w *Workspace, itemID, eventType string, delta map[string]any) error {
		if eventType == EventGateForced {
			return stderrors.New("injected forced-evidence append failure")
		}
		return appendItemEventErr(ctx, w, itemID, eventType, delta)
	}

	art, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"},
		TransitionOptions{Force: true, ForceReason: "hotfix", ForceSource: ForceSourceCLI})
	require.Error(t, err, "forced-evidence failure under evidence_required must refuse")
	assert.Nil(t, art)
	assert.Contains(t, err.Error(), "forced-gate evidence append failed")
	// The durable write never happened: the item is still active.
	assert.Equal(t, "active", statusOf(t, ws, id))
}

// TestGateEvidence_ExplicitGateBaseEqualDefault_StillAudited asserts that an
// explicit operator --gate-base is audited even when it happens to equal the
// discovered default ref (NonDefault is false but the override was still supplied).
func TestGateEvidence_ExplicitGateBaseEqualDefault_StillAudited(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	// fakeGitAllOK resolves the default to origin/HEAD; pass the same ref as an
	// explicit --gate-base so NonDefault is false yet the override must be audited.
	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"},
		TransitionOptions{GateBase: "origin/HEAD"})
	require.NoError(t, err)

	evs := eventsFor(t, ws, id)
	require.NotNil(t, findEvent(evs, EventGateBaseOverride),
		"an explicit --gate-base must be audited even when it equals the default ref")
}
