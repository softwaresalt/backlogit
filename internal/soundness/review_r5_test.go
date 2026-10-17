package soundness

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewR5_Dependency_CycleFindingsAreCapped(t *testing.T) {
	// n self-contained two-cycles: far more than the cap.
	var units []Unit
	for i := 0; i < 20; i++ {
		a, b := soundUnit(fmt.Sprintf("%da-T", i), "A"), soundUnit(fmt.Sprintf("%db-T", i), "B")
		a.Dependencies, b.Dependencies = []string{b.ID}, []string{a.ID}
		units = append(units, a, b)
	}
	got := findingsFor(t, "dependency-consistency", units...)
	require.Len(t, got, maxCycleFindings+1)
	assert.Contains(t, got[len(got)-1].Message, "additional cycles omitted")
	assert.Contains(t, got[len(got)-1].Message, "15")
}

func TestReviewR5_Dependency_LongCyclePathIsTruncated(t *testing.T) {
	var units []Unit
	const n = 3000 // also exercises the iterative walk (no recursion depth limit)
	for i := 0; i < n; i++ {
		u := soundUnit(fmt.Sprintf("%d-T", i), "U")
		u.Dependencies = []string{fmt.Sprintf("%d-T", (i+1)%n)}
		units = append(units, u)
	}
	got := findingsFor(t, "dependency-consistency", units...)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "…")
	assert.LessOrEqual(t, strings.Count(got[0].Message, "->"), maxCyclePathIDs)
}

func TestReviewR5_Dependency_RepeatedDanglingAndSelfReportOnce(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Dependencies = []string{"9-T", "9-T", "1-T", "1-T"}
	assert.Len(t, findingsFor(t, "dependency-consistency", u), 2)
}

func TestReviewR5_ConfinedArtifact_RejectsDotAndDriveRelative(t *testing.T) {
	for _, p := range []string{".", "./", "c:foo", "C:foo.go", `d:\x`, "c:"} {
		t.Run(p, func(t *testing.T) {
			_, err := confinedArtifact(p)
			assert.Error(t, err)
		})
	}
	for _, p := range []string{"a.go", "dir/c:x"} {
		_, err := confinedArtifact(p)
		assert.NoError(t, err, p)
	}
}

func TestReviewR5_ConfinedArtifactIn_OnlyOutsideRootSaysEscapes(t *testing.T) {
	key, err := confinedArtifactIn(t.TempDir(), "new/out.go")
	require.NoError(t, err)
	assert.Equal(t, "new/out.go", key)
}
