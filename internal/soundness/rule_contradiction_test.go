package soundness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU160007_Contradiction_FlagsUnitVsDecision(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Directives = []Directive{{Subject: "Squash merge", Stance: StanceRequire}}
	p := Plan{Units: []Unit{u}, Decisions: []Decision{{ID: "D1", Directives: []Directive{{Subject: "squash merge", Stance: StanceForbid}}}}}
	got := rulesOf(runPlan(t, p, DefaultConfig()))["contradiction"]
	require.Len(t, got, 1)
	assert.Equal(t, "1-T", got[0].UnitID)
	assert.Contains(t, got[0].Message, "D1")
}

func TestU160007_Contradiction_FlagsSiblings(t *testing.T) {
	a, b := soundUnit("1.1-T", "A"), soundUnit("1.2-T", "B")
	a.ParentID, b.ParentID = "1-F", "1-F"
	a.Directives = []Directive{{Subject: "use cgo", Stance: StanceRequire}}
	b.Directives = []Directive{{Subject: "use cgo", Stance: StanceForbid}}
	assert.Len(t, findingsFor(t, "contradiction", a, b), 1)
}

func TestU160007_Contradiction_ConsistentPasses(t *testing.T) {
	a, b, c := soundUnit("1.1-T", "A"), soundUnit("1.2-T", "B"), soundUnit("2.1-T", "C")
	a.ParentID, b.ParentID, c.ParentID = "1-F", "1-F", "2-F"
	a.Directives = []Directive{{Subject: "use cgo", Stance: StanceRequire}}
	b.Directives = []Directive{{Subject: "use cgo", Stance: StanceRequire}}
	c.Directives = []Directive{{Subject: "use cgo", Stance: StanceForbid}} // different parent: not a sibling
	p := Plan{Units: []Unit{a, b, c}, Decisions: []Decision{{ID: "D1", Directives: []Directive{{Subject: "use cgo", Stance: StanceRequire}}}}}
	got := rulesOf(runPlan(t, p, DefaultConfig()))["contradiction"]
	require.Len(t, got, 1, "only c contradicts decision D1")
	assert.Equal(t, "2.1-T", got[0].UnitID)
}

func TestU160007_Contradiction_UnknownStanceFailsClosed(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Directives = []Directive{{Subject: "x", Stance: "maybe"}}
	assert.Len(t, findingsFor(t, "contradiction", u), 1)
}
