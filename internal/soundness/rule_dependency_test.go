package soundness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findingsFor(t *testing.T, rule string, units ...Unit) []Finding {
	t.Helper()
	return rulesOf(runPlan(t, Plan{Units: units}, DefaultConfig()))[rule]
}

func TestU160002_DependencyConsistency_FlagsDangling(t *testing.T) {
	a := soundUnit("1-T", "A")
	a.Dependencies = []string{"9-T"}
	got := findingsFor(t, "dependency-consistency", a)
	require.Len(t, got, 1)
	assert.Equal(t, "1-T", got[0].UnitID)
	assert.Contains(t, got[0].Message, "9-T")
}

func TestU160002_DependencyConsistency_FlagsSelfAndCycle(t *testing.T) {
	self := soundUnit("1-T", "A")
	self.Dependencies = []string{"1-T"}
	assert.NotEmpty(t, findingsFor(t, "dependency-consistency", self))

	a, b, c := soundUnit("1-T", "A"), soundUnit("2-T", "B"), soundUnit("3-T", "C")
	a.Dependencies, b.Dependencies, c.Dependencies = []string{"2-T"}, []string{"3-T"}, []string{"1-T"}
	got := findingsFor(t, "dependency-consistency", a, b, c)
	require.Len(t, got, 1, "one finding per cycle")
	assert.Contains(t, got[0].Message, "cycle")
}

func TestU160002_DependencyConsistency_CleanGraphPasses(t *testing.T) {
	a, b, c := soundUnit("1-T", "A"), soundUnit("2-T", "B"), soundUnit("3-T", "C")
	b.Dependencies = []string{"1-T"}
	c.Dependencies = []string{"1-T", "2-T"}
	assert.Empty(t, findingsFor(t, "dependency-consistency", a, b, c))
}
