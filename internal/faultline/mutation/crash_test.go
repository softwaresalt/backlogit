package mutation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline/mutation"
)

// TestU3_PartialWrite_VerifySuccessFails verifies that a partial-write crash
// scenario — where only a subset of declared representations was written —
// causes VerifySuccess to return Passed=false, with the unwritten kinds
// listed in MissingReps in sorted order.
func TestU3_PartialWrite_VerifySuccessFails(t *testing.T) {
	op := uniqueOp("U3PartialWriteSuccess")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite, mutation.EventsJSONL},
	}))

	allKinds := []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite, mutation.EventsJSONL}
	updatedKinds := []mutation.RepresentationKind{mutation.Frontmatter}
	snap := partialWriteSnap(op, allKinds, updatedKinds)

	result, err := mutation.VerifySuccess(snap)
	require.NoError(t, err)
	assert.False(t, result.Passed, "partial write: VerifySuccess must return Passed=false")
	// Sorted alphabetically: "events_jsonl" < "sqlite"
	want := []mutation.RepresentationKind{mutation.EventsJSONL, mutation.SQLite}
	assert.Equal(t, want, result.MissingReps, "unwritten reps must appear in MissingReps")
}

// TestU3_PartialWrite_VerifyFailureFails verifies that a partial-write crash
// scenario causes VerifyFailure to return Passed=false, because the
// representations that were written show unexpected drift.
func TestU3_PartialWrite_VerifyFailureFails(t *testing.T) {
	op := uniqueOp("U3PartialWriteFailure")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite, mutation.EventsJSONL},
	}))

	allKinds := []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite, mutation.EventsJSONL}
	updatedKinds := []mutation.RepresentationKind{mutation.Frontmatter}
	snap := partialWriteSnap(op, allKinds, updatedKinds)

	result, err := mutation.VerifyFailure(snap)
	require.NoError(t, err)
	assert.False(t, result.Passed, "partial write: VerifyFailure must return Passed=false (Frontmatter drifted)")
	want := []mutation.RepresentationKind{mutation.Frontmatter}
	assert.Equal(t, want, result.DriftedReps, "written rep must appear in DriftedReps")
}

// TestU3_StaleIndex_VerifySucceeds verifies that a stale-index scenario —
// where the Before snapshot holds outdated bytes — does not cause a false
// positive: VerifySuccess correctly detects that all representations changed
// between the stale Before and the fresh After.
func TestU3_StaleIndex_VerifySucceeds(t *testing.T) {
	op := uniqueOp("U3StaleIndex")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}))

	kinds := []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite}
	snap := staleIndexSnap(op, kinds)

	result, err := mutation.VerifySuccess(snap)
	require.NoError(t, err)
	assert.True(t, result.Passed, "stale-index: VerifySuccess must pass (all reps differ)")
	assert.Empty(t, result.MissingReps, "no missing reps expected with stale Before")
}

// TestU3_OldIndex_VerifySucceeds verifies that an old-index scenario —
// where Before is 2+ generations stale — does not cause a false positive:
// VerifySuccess correctly detects old→new as a genuine change.
func TestU3_OldIndex_VerifySucceeds(t *testing.T) {
	op := uniqueOp("U3OldIndex")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}))

	kinds := []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite}
	snap := oldIndexSnap(op, kinds)

	result, err := mutation.VerifySuccess(snap)
	require.NoError(t, err)
	assert.True(t, result.Passed, "old-index: VerifySuccess must pass (all reps differ)")
	assert.Empty(t, result.MissingReps, "no missing reps expected with ancient Before")
}

// TestU3_IndeterminateAtomic_CommittedPasses verifies the "all reps committed"
// sub-case of an indeterminate atomic write: VerifySuccess passes because
// every representation shows a Before→After change.
func TestU3_IndeterminateAtomic_CommittedPasses(t *testing.T) {
	op := uniqueOp("U3IndeterminateCommitted")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}))

	kinds := []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite}
	committed, _ := indeterminateAtomicSnaps(op, kinds)

	result, err := mutation.VerifySuccess(committed)
	require.NoError(t, err)
	assert.True(t, result.Passed, "indeterminate-committed: VerifySuccess must pass")
	assert.Empty(t, result.MissingReps, "no missing reps: all reps were committed")
}

// TestU3_IndeterminateAtomic_RolledBackPasses verifies the "all reps rolled
// back" sub-case of an indeterminate atomic write: VerifyFailure passes
// because no representation changed.
func TestU3_IndeterminateAtomic_RolledBackPasses(t *testing.T) {
	op := uniqueOp("U3IndeterminateRolledBack")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}))

	kinds := []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite}
	_, rolledBack := indeterminateAtomicSnaps(op, kinds)

	result, err := mutation.VerifyFailure(rolledBack)
	require.NoError(t, err)
	assert.True(t, result.Passed, "indeterminate-rolled-back: VerifyFailure must pass")
	assert.Empty(t, result.DriftedReps, "no drifted reps: all reps were rolled back")
}
