package soundness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU160010_IndependentVerifiability_FlagsVagueAndMissingCriteria(t *testing.T) {
	vague := soundUnit("1-T", "A")
	vague.Acceptance = []string{"It works properly"}
	got := findingsFor(t, "independent-verifiability", vague)
	require.Len(t, got, 1)
	assert.Equal(t, "1-T", got[0].UnitID)

	none := soundUnit("2-T", "B")
	none.Acceptance = nil
	assert.Len(t, findingsFor(t, "independent-verifiability", none), 1)
}

func TestU160010_IndependentVerifiability_FlagsUndeclaredVerificationCoupling(t *testing.T) {
	a, b := soundUnit("1-T", "A"), soundUnit("2-T", "B")
	b.VerifiedWith = []string{"1-T"}
	got := findingsFor(t, "independent-verifiability", a, b)
	require.Len(t, got, 1)
	assert.Equal(t, "2-T", got[0].UnitID)
}

func TestU160010_IndependentVerifiability_CleanPasses(t *testing.T) {
	a, b := soundUnit("1-T", "A"), soundUnit("2-T", "B")
	b.Dependencies = []string{"1-T"}
	b.VerifiedWith = []string{"1-T"}
	b.Acceptance = []string{"B returns 3 for input 2", "go test ./internal/x exits 0"}
	assert.Empty(t, findingsFor(t, "independent-verifiability", a, b))
}

func TestCopilotR1_IndependentVerifiability_ObjectiveCriteriaWithKeywordsAreClean(t *testing.T) {
	for _, c := range []string{
		"Creates exactly one work item with status queued",
		"Reading /etc/config returns ENOENT when absent",
	} {
		u := soundUnit("1-T", "A")
		u.Acceptance = []string{c}
		assert.Empty(t, findingsFor(t, "independent-verifiability", u), c)
	}
	for _, c := range []string{"It works properly", "Handles errors appropriately", "Output is good", "Configs, etc."} {
		u := soundUnit("1-T", "A")
		u.Acceptance = []string{c}
		assert.Len(t, findingsFor(t, "independent-verifiability", u), 1, c)
	}
}
