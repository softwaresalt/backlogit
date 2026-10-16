package soundness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU160005_UniqueTitles_FlagsUnrelatedDuplicate(t *testing.T) {
	a, b := soundUnit("1-T", "Add Parser"), soundUnit("2-T", "  add   parser ")
	got := findingsFor(t, "unique-titles", a, b)
	require.Len(t, got, 1)
	assert.Equal(t, "2-T", got[0].UnitID)
	assert.Contains(t, got[0].Message, "1-T")
}

func TestU160005_UniqueTitles_IntentionallyRelatedPasses(t *testing.T) {
	for name, mut := range map[string]func(a, b *Unit){
		"dependency": func(_, b *Unit) { b.Dependencies = []string{"1-T"} },
		"related":    func(a, _ *Unit) { a.Related = []string{"2-T"} },
		"parent":     func(_, b *Unit) { b.ParentID = "1-T" },
	} {
		t.Run(name, func(t *testing.T) {
			a, b := soundUnit("1-T", "Add parser"), soundUnit("2-T", "Add parser")
			mut(&a, &b)
			assert.Empty(t, findingsFor(t, "unique-titles", a, b))
		})
	}
}

func TestU160005_UniqueTitles_DistinctTitlesPass(t *testing.T) {
	assert.Empty(t, findingsFor(t, "unique-titles", soundUnit("1-T", "Add parser"), soundUnit("2-T", "Add renderer")))
}
