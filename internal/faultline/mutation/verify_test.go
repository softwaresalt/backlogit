package mutation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline/mutation"
)

// TestU2_VerifySuccess_AllChanged verifies the happy path: when all declared
// representations differ between Before and After, VerifySuccess returns
// Passed=true with an empty MissingReps slice.
func TestU2_VerifySuccess_AllChanged(t *testing.T) {
	op := uniqueOp("U2SuccessAll")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}))

	snap := mutation.MutationSnapshot{
		Op: op,
		Before: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("before-fm"),
			mutation.SQLite:      []byte("before-sql"),
		},
		After: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("after-fm"),
			mutation.SQLite:      []byte("after-sql"),
		},
	}

	result, err := mutation.VerifySuccess(snap)
	require.NoError(t, err)
	assert.Equal(t, op, result.Op)
	assert.True(t, result.Passed, "all reps changed: Passed must be true")
	assert.Empty(t, result.MissingReps, "no missing reps expected")
}

// TestU2_VerifySuccess_OneMissing verifies that when one declared
// representation is unchanged, VerifySuccess returns Passed=false with that
// kind in MissingReps.
func TestU2_VerifySuccess_OneMissing(t *testing.T) {
	op := uniqueOp("U2SuccessOneMissing")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}))

	snap := mutation.MutationSnapshot{
		Op: op,
		Before: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("before-fm"),
			mutation.SQLite:      []byte("same-sql"),
		},
		After: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("after-fm"),
			mutation.SQLite:      []byte("same-sql"), // unchanged — missing update
		},
	}

	result, err := mutation.VerifySuccess(snap)
	require.NoError(t, err)
	assert.False(t, result.Passed, "one rep unchanged: Passed must be false")
	assert.Equal(t, []mutation.RepresentationKind{mutation.SQLite}, result.MissingReps)
}

// TestU2_VerifySuccess_AllMissing verifies that when no declared
// representations changed, VerifySuccess returns Passed=false with all
// declared kinds in MissingReps in sorted (by kind string value) order.
func TestU2_VerifySuccess_AllMissing(t *testing.T) {
	op := uniqueOp("U2SuccessAllMissing")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite, mutation.EventsJSONL},
	}))

	snap := mutation.MutationSnapshot{
		Op: op,
		Before: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("same"),
			mutation.SQLite:      []byte("same"),
			mutation.EventsJSONL: []byte("same"),
		},
		After: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("same"),
			mutation.SQLite:      []byte("same"),
			mutation.EventsJSONL: []byte("same"),
		},
	}

	result, err := mutation.VerifySuccess(snap)
	require.NoError(t, err)
	assert.False(t, result.Passed, "no reps changed: Passed must be false")
	// Sorted alphabetically: "events_jsonl" < "frontmatter" < "sqlite"
	want := []mutation.RepresentationKind{mutation.EventsJSONL, mutation.Frontmatter, mutation.SQLite}
	assert.Equal(t, want, result.MissingReps)
}

// TestU2_VerifyFailure_AllUnchanged verifies the happy path: when no declared
// representations changed between Before and After, VerifyFailure returns
// Passed=true with an empty DriftedReps slice.
func TestU2_VerifyFailure_AllUnchanged(t *testing.T) {
	op := uniqueOp("U2FailureAllUnchanged")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}))

	snap := mutation.MutationSnapshot{
		Op: op,
		Before: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("same-fm"),
			mutation.SQLite:      []byte("same-sql"),
		},
		After: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("same-fm"),
			mutation.SQLite:      []byte("same-sql"),
		},
	}

	result, err := mutation.VerifyFailure(snap)
	require.NoError(t, err)
	assert.Equal(t, op, result.Op)
	assert.True(t, result.Passed, "no reps changed: Passed must be true")
	assert.Empty(t, result.DriftedReps, "no drifted reps expected")
}

// TestU2_VerifyFailure_OneDrifted verifies that when one declared
// representation changed unexpectedly, VerifyFailure returns Passed=false
// with that kind in DriftedReps.
func TestU2_VerifyFailure_OneDrifted(t *testing.T) {
	op := uniqueOp("U2FailureOneDrifted")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite},
	}))

	snap := mutation.MutationSnapshot{
		Op: op,
		Before: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("same-fm"),
			mutation.SQLite:      []byte("before-sql"),
		},
		After: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("same-fm"),
			mutation.SQLite:      []byte("after-sql"), // changed — unexpected drift
		},
	}

	result, err := mutation.VerifyFailure(snap)
	require.NoError(t, err)
	assert.False(t, result.Passed, "one rep drifted: Passed must be false")
	assert.Equal(t, []mutation.RepresentationKind{mutation.SQLite}, result.DriftedReps)
}

// TestU2_VerifyFailure_AllDrifted verifies that when all declared
// representations changed unexpectedly, VerifyFailure returns Passed=false
// with all kinds in DriftedReps in sorted (by kind string value) order.
func TestU2_VerifyFailure_AllDrifted(t *testing.T) {
	op := uniqueOp("U2FailureAllDrifted")
	require.NoError(t, mutation.Register(mutation.RepresentationSet{
		Op:              op,
		Representations: []mutation.RepresentationKind{mutation.Frontmatter, mutation.SQLite, mutation.EventsJSONL},
	}))

	snap := mutation.MutationSnapshot{
		Op: op,
		Before: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("before-fm"),
			mutation.SQLite:      []byte("before-sql"),
			mutation.EventsJSONL: []byte("before-ej"),
		},
		After: map[mutation.RepresentationKind][]byte{
			mutation.Frontmatter: []byte("after-fm"),
			mutation.SQLite:      []byte("after-sql"),
			mutation.EventsJSONL: []byte("after-ej"),
		},
	}

	result, err := mutation.VerifyFailure(snap)
	require.NoError(t, err)
	assert.False(t, result.Passed, "all reps drifted: Passed must be false")
	// Sorted alphabetically: "events_jsonl" < "frontmatter" < "sqlite"
	want := []mutation.RepresentationKind{mutation.EventsJSONL, mutation.Frontmatter, mutation.SQLite}
	assert.Equal(t, want, result.DriftedReps)
}

// TestU2_UnregisteredOp verifies that both VerifySuccess and VerifyFailure
// return a non-nil error when the op name is not in the registry.
func TestU2_UnregisteredOp(t *testing.T) {
	snap := mutation.MutationSnapshot{
		Op:     "definitely-not-a-registered-op-xyz-u2",
		Before: nil,
		After:  nil,
	}

	t.Run("VerifySuccess", func(t *testing.T) {
		_, err := mutation.VerifySuccess(snap)
		require.Error(t, err, "VerifySuccess with unregistered op must return error")
	})

	t.Run("VerifyFailure", func(t *testing.T) {
		_, err := mutation.VerifyFailure(snap)
		require.Error(t, err, "VerifyFailure with unregistered op must return error")
	})
}

// TestU2_NilSliceEquality verifies that nil Before/After map entries are
// treated as empty bytes: nil==nil counts as no change (VerifyFailure passes),
// while nil!=non-nil counts as a change (VerifySuccess passes).
func TestU2_NilSliceEquality(t *testing.T) {
	t.Run("nil_nil_passes_failure", func(t *testing.T) {
		op := uniqueOp("U2NilNil")
		require.NoError(t, mutation.Register(mutation.RepresentationSet{
			Op:              op,
			Representations: []mutation.RepresentationKind{mutation.Frontmatter},
		}))

		// Both Before and After are nil maps: all lookups return nil bytes.
		snap := mutation.MutationSnapshot{
			Op:     op,
			Before: nil,
			After:  nil,
		}

		result, err := mutation.VerifyFailure(snap)
		require.NoError(t, err)
		assert.True(t, result.Passed, "nil==nil: no drift, VerifyFailure must pass")
		assert.Empty(t, result.DriftedReps)
	})

	t.Run("nil_vs_nonnil_passes_success", func(t *testing.T) {
		op := uniqueOp("U2NilNonNil")
		require.NoError(t, mutation.Register(mutation.RepresentationSet{
			Op:              op,
			Representations: []mutation.RepresentationKind{mutation.Frontmatter},
		}))

		// Before has no entry (nil lookup), After has content: counts as changed.
		snap := mutation.MutationSnapshot{
			Op:     op,
			Before: nil,
			After: map[mutation.RepresentationKind][]byte{
				mutation.Frontmatter: []byte("new-content"),
			},
		}

		result, err := mutation.VerifySuccess(snap)
		require.NoError(t, err)
		assert.True(t, result.Passed, "nil!=non-nil: changed, VerifySuccess must pass")
		assert.Empty(t, result.MissingReps)
	})
}
