package core

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateproof"
)

// TestGateEvidence_FormalGateDisabled_DeltaUnchanged verifies that with formal
// admission neither enabled by config nor required by the environment, the
// evidence delta carries exactly today's fields — no proof, key_id,
// proof_schema, or counter — preserving byte-for-byte backward compatibility
// (106-F F1/U4).
func TestGateEvidence_FormalGateDisabled_DeltaUnchanged(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "")
	unsetTestEnv(t, "BACKLOGIT_GATE_EVIDENCE_KEY")
	unsetTestEnv(t, "BACKLOGIT_FORMAL_GATE_REQUIRED")

	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	ev := findEvent(eventsFor(t, ws, id), EventGatePassed)
	require.NotNil(t, ev)
	_, hasProof := ev.Delta["proof"]
	_, hasKeyID := ev.Delta["key_id"]
	_, hasSchema := ev.Delta["proof_schema"]
	_, hasCounter := ev.Delta["counter"]
	assert.False(t, hasProof, "proof must be absent when formal gate is disabled")
	assert.False(t, hasKeyID, "key_id must be absent when formal gate is disabled")
	assert.False(t, hasSchema, "proof_schema must be absent when formal gate is disabled")
	assert.False(t, hasCounter, "counter must be absent when formal gate is disabled")
}

// TestGateEvidence_FormalGateEnabled_DeltaCarriesProofAndCounter verifies that
// with formal admission enabled via workspace config and a valid key resolved
// from the environment, the evidence delta carries proof, key_id,
// proof_schema, and an incrementing counter (106-F F1/U4).
func TestGateEvidence_FormalGateEnabled_DeltaCarriesProofAndCounter(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	unsetTestEnv(t, "BACKLOGIT_FORMAL_GATE_REQUIRED")

	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	id := newActiveTask(t, ws)
	report := `{"reviewers":[{"persona":"Constitution Reviewer","decision":"pass"}]}`
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(report)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.NoError(t, err)

	ev := findEvent(eventsFor(t, ws, id), EventGatePassed)
	require.NotNil(t, ev)
	proof, _ := ev.Delta["proof"].(string)
	assert.NotEmpty(t, proof, "proof must be present when formal gate is enabled")
	assert.Equal(t, "k1", ev.Delta["key_id"])
	assert.EqualValues(t, gateproof.Schema, ev.Delta["proof_schema"])
	assert.EqualValues(t, 1, ev.Delta["counter"], "first evidence event for this item should be counter 1")
}

// TestGateEvidence_FormalGateEnabled_InvalidReportRefusesCompletion verifies
// that a passing gate decision whose report fails the schema-validated
// formal report contract (106-F F1/U5) is refused when formal admission is
// enabled — a bare exit-0 pass with an empty/non-conforming report is not
// sufficient evidence for a formal proof, even though the underlying
// (non-formal) gate decision itself still says "proceed."
func TestGateEvidence_FormalGateEnabled_InvalidReportRefusesCompletion(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	unsetTestEnv(t, "BACKLOGIT_FORMAL_GATE_REQUIRED")

	ws := newGateTestWorkspace(t)
	ws.Config.FormalGate = &config.FormalGateConfig{Enabled: true, KeyID: "k1"}
	id := newActiveTask(t, ws)
	// Exit 0 with an empty report: Decide() still returns DecisionProceed
	// (unchanged, permissive default), but ValidateFormalReport must reject it.
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte{}}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err, "completion must refuse: formal gate is enabled but the report lacks attributed-review evidence")
	assert.Equal(t, "active", statusOf(t, ws, id), "status must not change on a refused formal-gate completion")
}

// TestGateEvidence_FormalGateRequired_KeyMissing_RefusesCompletion verifies
// that when enforcement is required (via the environment anchor) but the key
// cannot be resolved, the completion is refused and the item's status is left
// unchanged — there is no unauthenticated fallback (106-F F1/U4).
func TestGateEvidence_FormalGateRequired_KeyMissing_RefusesCompletion(t *testing.T) {
	unsetTestEnv(t, "BACKLOGIT_GATE_EVIDENCE_KEY")
	t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", "true")

	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	injectBroker(ws, gate.EnabledAuto, runner, fakeVersion{v: okVersion})

	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id, map[string]any{"status": "done"}, TransitionOptions{})
	require.Error(t, err, "completion must refuse when formal gate is required but the key is unresolvable")

	assert.Equal(t, "active", statusOf(t, ws, id), "status must not change on a refused formal-gate completion")
}

// TestNextGateEvidenceCounter_ConcurrentAllocationsAreUnique verifies the
// dedicated counter-allocation lock: N goroutines racing to allocate a counter
// for the SAME item produce N distinct, gapless values with no duplicates
// (106-F F1/U4), mirroring events.HookEventWriter's combined in-process mutex
// plus cross-process sidecar-lock pattern. Each goroutine appends a real event
// carrying its allocated counter before unlocking (as the real
// augmentDeltaWithFormalProof + append sequence does), since the counter is
// derived by scanning the durable log — without a durable append after
// allocation, every scan would see an empty log and reallocate 1.
func TestNextGateEvidenceCounter_ConcurrentAllocationsAreUnique(t *testing.T) {
	ws := newGateTestWorkspace(t)
	id := newActiveTask(t, ws)
	logsDir := WorkspaceLogsRoot(ws.RootPath)

	const n = 20
	var wg sync.WaitGroup
	results := make([]int64, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			counter, unlock, err := nextGateEvidenceCounter(ctx, ws, id)
			if err != nil {
				errs[idx] = err
				return
			}
			defer unlock()
			results[idx] = counter
			// Simulate the sign+append step that must happen while still
			// holding the lock in the real augmentDeltaWithFormalProof flow.
			writer := NewWorkspaceEventWriter(ws, logsDir)
			appendErr := writer.AppendEvent(ctx, events.Event{
				Timestamp: time.Now(),
				Actor:     "backlogit",
				ItemID:    id,
				EventType: EventGatePassed,
				Delta:     map[string]any{"counter": counter},
			})
			if appendErr != nil {
				errs[idx] = appendErr
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[int64]bool, n)
	for i, err := range errs {
		require.NoError(t, err, "goroutine %d", i)
	}
	for _, c := range results {
		require.False(t, seen[c], "duplicate counter allocated: %d (all: %v)", c, results)
		seen[c] = true
	}
	require.Len(t, seen, n, "expected %d distinct counters, got %v", n, results)
}


// unsetTestEnv removes an environment variable for the duration of the test,
// restoring any prior value afterward via t.Cleanup.
func unsetTestEnv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	_ = os.Unsetenv(key)
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}
