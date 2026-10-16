package soundness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU160008_DuplicatedWork_FlagsSharedDeliverable(t *testing.T) {
	a, b := soundUnit("1-T", "Build parser"), soundUnit("2-T", "Write parser")
	a.Deliverables = []string{"internal/x/parser.go"}
	b.Deliverables = []string{"./internal/x/Parser.go"}
	got := findingsFor(t, "duplicated-work", a, b)
	require.Len(t, got, 1)
	assert.Equal(t, "2-T", got[0].UnitID)
}

func TestU160008_DuplicatedWork_RelatedOrDistinctPasses(t *testing.T) {
	a, b := soundUnit("1-T", "A"), soundUnit("2-T", "B")
	a.Deliverables = []string{"x.go"}
	b.Deliverables = []string{"x.go"}
	b.Dependencies = []string{"1-T"}
	assert.Empty(t, findingsFor(t, "duplicated-work", a, b), "extension of a dependency is intentional")

	b.Dependencies = nil
	b.Deliverables = []string{"y.go"}
	assert.Empty(t, findingsFor(t, "duplicated-work", a, b))
}
