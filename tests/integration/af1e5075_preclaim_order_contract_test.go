package integration_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// requireAF1E5075Before asserts that first occurs in text and that its first
// occurrence precedes the first occurrence of second.
func requireAF1E5075Before(t *testing.T, text, first, second string) {
	t.Helper()

	firstIdx := strings.Index(text, first)
	require.GreaterOrEqual(t, firstIdx, 0, "%q not found", first)
	secondIdx := strings.Index(text, second)
	require.GreaterOrEqual(t, secondIdx, 0, "%q not found", second)
	require.Less(t, firstIdx, secondIdx, "%q must occur before %q", first, second)
}

// TestAF1E5075PreclaimOrder pins where the hold-label and condition (b) checks
// sit relative to the claim path (U3).
func TestAF1E5075PreclaimOrder(t *testing.T) {
	sections := loadAF1E5075Sections(t)

	t.Run("Orchestrator", func(t *testing.T) {
		text := sections.orchestrator
		requireAF1E5075Before(t, text, "hard claim refusal label", "TOPOLOGY_GATE: pre_claim")
		requireAF1E5075Before(t, text, "CONDITION_B_UNSATISFIED", "TOPOLOGY_GATE: pre_claim")
	})

	t.Run("Ship", func(t *testing.T) {
		t.Run("MainPath", func(t *testing.T) {
			text := sections.ship
			requireAF1E5075Before(t, text, "hard claim refusal label", "**Branch Creation Gate")
			requireAF1E5075Before(t, text, "CONDITION_B_UNSATISFIED", "**Branch Creation Gate")

			item4 := strings.Index(text, "4. If the shipment is still in `queued` status")
			require.GreaterOrEqual(t, item4, 0, "main path item 4 not found")
			requireAF1E5075Before(t, text[item4:],
				"repeat the hold-label and condition (b) checks", "backlogit_claim_shipment")

			neverRetried := strings.Index(text, "never retried")
			require.GreaterOrEqual(t, neverRetried, 0, "never retried not found")
			require.Contains(t, text[neverRetried:], "CONDITION_B_UNSATISFIED")
			requireAF1E5075Before(t, text, "never retried", "CLAIM_NOT_OBSERVED")
			// The retry statement must sit in the terminal-halt wording that
			// precedes the step 4a retry carve-out.
			requireAF1E5075Before(t, text[item4:], "never retried", "CLAIM_NOT_OBSERVED")
		})

		t.Run("Fallback", func(t *testing.T) {
			text := sections.shipFallback
			requireAF1E5075Before(t, text,
				"repeat the hold-label and condition (b) checks", "backlogit_claim_shipment")
			requireAF1E5075Before(t, text, "never retried", "backlogit_claim_shipment")
			requireAF1E5075Before(t, text, "shipment_predecessor_not_shipped", "backlogit_claim_shipment")
		})
	})
}
