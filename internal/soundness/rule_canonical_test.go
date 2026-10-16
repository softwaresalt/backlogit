package soundness

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU160006_CanonicalProcedure_FlagsNonCanonical(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Procedure = "ad-hoc-hack"
	cfg := DefaultConfig()
	cfg.CanonicalProcedures = []string{"build-feature", "review"}
	got := rulesOf(runPlan(t, Plan{Units: []Unit{u}}, cfg))["canonical-procedure"]
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "ad-hoc-hack")
}

func TestU160006_CanonicalProcedure_CanonicalAndAbsentPass(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CanonicalProcedures = []string{"build-feature", "review"}
	ok, none := soundUnit("1-T", "A"), soundUnit("2-T", "B")
	ok.Procedure = "build-feature"
	assert.Empty(t, rulesOf(runPlan(t, Plan{Units: []Unit{ok, none}}, cfg))["canonical-procedure"])
}

func TestU160006_CanonicalProcedure_NoCanonicalSetFailsClosed(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Procedure = "build-feature"
	assert.Len(t, findingsFor(t, "canonical-procedure", u), 1)
}
