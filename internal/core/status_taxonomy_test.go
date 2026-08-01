package core

// F3 (106.002-T) red-before-green tests for the NEW context-specific taxonomy
// predicates and the immutable cascade accessor. These fail to compile until
// status_taxonomy.go is implemented (RED), then pass (GREEN).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// goldenCascadeStatuses is the authoritative 6-status cascade set. This literal is
// the golden truth-table pin: any change here (or a divergence of the accessor from
// it) is the named rollback trigger for F3.
var goldenCascadeStatuses = []string{"abandoned", "accepted", "archived", "done", "rejected", "shipped"}

// TestIsCascadeTerminalStatus pins the children-terminal / parent-completion
// predicate over the full lifecycle universe: the six cascade-terminal statuses
// are true, the four in-flight statuses are false, and unknown/empty fail closed.
func TestIsCascadeTerminalStatus(t *testing.T) {
	want := map[string]bool{
		"done": true, "accepted": true, "archived": true,
		"shipped": true, "abandoned": true, "rejected": true,
		"queued": false, "active": false, "blocked": false, "review": false,
	}
	for _, status := range allLifecycleStatuses {
		t.Run(string(status), func(t *testing.T) {
			assert.Equal(t, want[string(status)], IsCascadeTerminalStatus(string(status)))
		})
	}
	t.Run("unknown_fails_closed", func(t *testing.T) {
		assert.False(t, IsCascadeTerminalStatus("garbage"))
		assert.False(t, IsCascadeTerminalStatus(""))
	})
}

// TestIsNoLongerBlockingStatus pins the queue dependency-resolution predicate and
// asserts it currently ALIASES the cascade set (both share the 6-status set today).
func TestIsNoLongerBlockingStatus(t *testing.T) {
	for _, status := range allLifecycleStatuses {
		s := string(status)
		t.Run(s, func(t *testing.T) {
			assert.Equal(t, IsCascadeTerminalStatus(s), IsNoLongerBlockingStatus(s),
				"no-longer-blocking and cascade share the 6-status set today")
		})
	}
	t.Run("shipped_and_abandoned_stop_blocking", func(t *testing.T) {
		assert.True(t, IsNoLongerBlockingStatus("shipped"))
		assert.True(t, IsNoLongerBlockingStatus("abandoned"))
	})
}

// TestCascadeTerminalStatuses_ReturnsGoldenSet asserts the accessor exposes exactly
// the golden 6-status set (order-independent).
func TestCascadeTerminalStatuses_ReturnsGoldenSet(t *testing.T) {
	assert.ElementsMatch(t, goldenCascadeStatuses, CascadeTerminalStatuses())
}

// TestCascadeTerminalStatuses_Immutable asserts the taxonomy cannot be mutated
// externally: the accessor returns a fresh COPY each call, so mutating the returned
// slice must not affect subsequent calls.
func TestCascadeTerminalStatuses_Immutable(t *testing.T) {
	first := CascadeTerminalStatuses()
	require.NotEmpty(t, first)
	for i := range first {
		first[i] = "MUTATED"
	}
	second := CascadeTerminalStatuses()
	assert.ElementsMatch(t, goldenCascadeStatuses, second,
		"mutating the returned slice must not corrupt the backing taxonomy")
	assert.NotEqual(t, first, second, "accessor must return a fresh copy, not a shared slice")
}

// TestIsReleasableStatus pins the releasable (release-progression) predicate to the
// 4-status set and asserts it stays behaviorally identical to isTerminalReleaseStatus.
func TestIsReleasableStatus(t *testing.T) {
	want := map[models.ArtifactStatus]bool{
		models.StatusDone: true, models.StatusAccepted: true,
		models.StatusRejected: true, models.StatusArchived: true,
		models.StatusShipped: false, models.StatusAbandoned: false,
		models.StatusQueued: false, models.StatusActive: false,
		models.StatusBlocked: false, models.StatusReview: false,
	}
	for _, status := range allLifecycleStatuses {
		t.Run(string(status), func(t *testing.T) {
			assert.Equal(t, want[status], IsReleasableStatus(status))
			assert.Equal(t, isTerminalReleaseStatus(status), IsReleasableStatus(status),
				"isTerminalReleaseStatus must delegate to IsReleasableStatus")
		})
	}
}

// TestIsGateTargetStatus_HonorsConfiguredSet pins that gate-target is the ONLY
// parameterized predicate: it reads the workspace-configured terminal set rather
// than a static "done". An empty/nil set falls back to the ["done"] default.
func TestIsGateTargetStatus_HonorsConfiguredSet(t *testing.T) {
	t.Run("default_when_empty", func(t *testing.T) {
		assert.True(t, IsGateTargetStatus("done", nil))
		assert.True(t, IsGateTargetStatus("done", []string{}))
		assert.False(t, IsGateTargetStatus("rejected", nil))
	})
	t.Run("configured_done_rejected_is_honored", func(t *testing.T) {
		cfg := []string{"done", "rejected"}
		assert.True(t, IsGateTargetStatus("done", cfg))
		assert.True(t, IsGateTargetStatus("rejected", cfg))
		assert.False(t, IsGateTargetStatus("accepted", cfg))
	})
	t.Run("configured_done_archived_is_honored", func(t *testing.T) {
		cfg := []string{"done", "archived"}
		assert.True(t, IsGateTargetStatus("archived", cfg))
		assert.False(t, IsGateTargetStatus("rejected", cfg))
	})
	t.Run("unknown_status_is_false", func(t *testing.T) {
		assert.False(t, IsGateTargetStatus("garbage", []string{"done", "rejected"}))
	})
}
