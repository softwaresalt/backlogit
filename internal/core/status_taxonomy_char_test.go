package core

// F3 (106.002-T) characterization tests: capture the CURRENT status-taxonomy
// truth tables on a GREEN baseline BEFORE the refactor. These reference only the
// existing predicates (isTerminalReleaseStatus, isDescopeEligibleStatus,
// isRecognizedReleaseStatus) so they pin present behavior. The two truth tables
// diverge DELIBERATELY and MUST NOT be unified:
//
//   - 6-status cascade set {done, accepted, archived, shipped, abandoned, rejected}
//     governs the downward blocking cascade + queue no-longer-blocking resolution
//     (pinned behaviorally in blocking_cascade_char_test.go via CheckChildrenTerminal).
//   - 4-status release set {done, accepted, rejected, archived} (omits shipped and
//     abandoned) governs release relocation/lifecycle — pinned here.

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/models"
)

// allLifecycleStatuses is the full artifact lifecycle enum. Every predicate's
// characterization runs over this SAME universe so an accidental set change is
// impossible to miss.
var allLifecycleStatuses = []models.ArtifactStatus{
	models.StatusQueued, models.StatusActive, models.StatusBlocked,
	models.StatusReview, models.StatusDone, models.StatusAccepted,
	models.StatusRejected, models.StatusArchived, models.StatusShipped,
	models.StatusAbandoned,
}

// TestTaxonomyChar_IsTerminalReleaseStatus pins the 4-status release-progression
// truth table exactly as it is today: {done, accepted, rejected, archived} are
// releasable; shipped and abandoned are DELIBERATELY excluded. Unknown/malformed
// provenance fails closed (false).
func TestTaxonomyChar_IsTerminalReleaseStatus(t *testing.T) {
	want := map[models.ArtifactStatus]bool{
		models.StatusDone:      true,
		models.StatusAccepted:  true,
		models.StatusRejected:  true,
		models.StatusArchived:  true,
		models.StatusShipped:   false, // deliberate divergence from the 6-status cascade set
		models.StatusAbandoned: false, // deliberate divergence from the 6-status cascade set
		models.StatusQueued:    false,
		models.StatusActive:    false,
		models.StatusBlocked:   false,
		models.StatusReview:    false,
	}
	for _, status := range allLifecycleStatuses {
		t.Run(string(status), func(t *testing.T) {
			assert.Equal(t, want[status], isTerminalReleaseStatus(status))
		})
	}
	t.Run("unknown_fails_closed", func(t *testing.T) {
		assert.False(t, isTerminalReleaseStatus(models.ArtifactStatus("garbage")))
		assert.False(t, isTerminalReleaseStatus(models.ArtifactStatus("")))
	})
}

// TestTaxonomyChar_IsDescopeEligibleStatus pins descope-eligibility as it stands
// today: in-flight statuses (queued, active, blocked, review) and non-completion
// terminals (abandoned, rejected) are descope-eligible; completions (done,
// accepted, shipped) and the archived sink are NOT. This orthogonality with
// releasability is intentional (rejected is releasable AND descope-eligible;
// abandoned is neither releasable nor... it IS descope-eligible but not releasable).
func TestTaxonomyChar_IsDescopeEligibleStatus(t *testing.T) {
	want := map[models.ArtifactStatus]bool{
		models.StatusQueued:    true,
		models.StatusActive:    true,
		models.StatusBlocked:   true,
		models.StatusReview:    true,
		models.StatusAbandoned: true,
		models.StatusRejected:  true,
		models.StatusDone:      false,
		models.StatusAccepted:  false,
		models.StatusShipped:   false,
		models.StatusArchived:  false,
	}
	for _, status := range allLifecycleStatuses {
		t.Run(string(status), func(t *testing.T) {
			assert.Equal(t, want[status], isDescopeEligibleStatus(status))
		})
	}
	t.Run("unknown_fails_closed", func(t *testing.T) {
		assert.False(t, isDescopeEligibleStatus(models.ArtifactStatus("garbage")))
		assert.False(t, isDescopeEligibleStatus(models.ArtifactStatus("")))
	})
}

// TestTaxonomyChar_IsRecognizedReleaseStatus pins the recognized-status allowlist:
// every one of the ten known lifecycle statuses is recognized, and any unknown or
// empty value is rejected so safety-critical callers can fail closed rather than
// misclassify malformed provenance.
func TestTaxonomyChar_IsRecognizedReleaseStatus(t *testing.T) {
	for _, status := range allLifecycleStatuses {
		t.Run(string(status), func(t *testing.T) {
			assert.True(t, isRecognizedReleaseStatus(status),
				"every known lifecycle status must be recognized")
		})
	}
	t.Run("unknown_rejected", func(t *testing.T) {
		assert.False(t, isRecognizedReleaseStatus(models.ArtifactStatus("garbage")))
		assert.False(t, isRecognizedReleaseStatus(models.ArtifactStatus("")))
		assert.False(t, isRecognizedReleaseStatus(models.ArtifactStatus("Done")))
	})
}
