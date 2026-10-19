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
	assert.Nil(t, result.DriftedReps, "DriftedReps must be nil on success path")
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
	assert.Nil(t, result.DriftedReps, "DriftedReps must be nil on success path")
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
	assert.Nil(t, result.DriftedReps, "DriftedReps must be nil on success path")
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
	assert.Nil(t, result.MissingReps, "MissingReps must be nil on failure path")
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
	assert.Nil(t, result.MissingReps, "MissingReps must be nil on failure path")
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
	assert.Nil(t, result.MissingReps, "MissingReps must be nil on failure path")
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
		assert.ErrorIs(t, err, mutation.ErrOpNotRegistered)
	})

	t.Run("VerifyFailure", func(t *testing.T) {
		_, err := mutation.VerifyFailure(snap)
		require.Error(t, err, "VerifyFailure with unregistered op must return error")
		assert.ErrorIs(t, err, mutation.ErrOpNotRegistered)
	})
}

// TestU2_NilSliceEquality verifies nil Before/After snapshot semantics. A
// snapshot whose Before and After both lack a declared key (nil maps or absent
// entries) is unobservable and returns IncompleteSnapshot=true. A snapshot
// where one side carries a non-nil value for a key is observable and proceeds
// to the normal byte comparison.
func TestU2_NilSliceEquality(t *testing.T) {
	t.Run("nil_nil_incomplete_failure", func(t *testing.T) {
		op := uniqueOp("U2NilNilF")
		require.NoError(t, mutation.Register(mutation.RepresentationSet{
			Op:              op,
			Representations: []mutation.RepresentationKind{mutation.Frontmatter},
		}))

		// Both Before and After are nil maps: all lookups return nil bytes and
		// key-presence checks return false. The completeness guard fires.
		snap := mutation.MutationSnapshot{
			Op:     op,
			Before: nil,
			After:  nil,
		}

		result, err := mutation.VerifyFailure(snap)
		require.NoError(t, err)
		assert.False(t, result.Passed, "nil/nil snapshot is unobservable: Passed must be false")
		assert.True(t, result.IncompleteSnapshot, "nil/nil snapshot must set IncompleteSnapshot")
		assert.Empty(t, result.DriftedReps)
		assert.Nil(t, result.MissingReps)
	})

	t.Run("nil_nil_incomplete_success", func(t *testing.T) {
		op := uniqueOp("U2NilNilS")
		require.NoError(t, mutation.Register(mutation.RepresentationSet{
			Op:              op,
			Representations: []mutation.RepresentationKind{mutation.Frontmatter},
		}))

		snap := mutation.MutationSnapshot{
			Op:     op,
			Before: nil,
			After:  nil,
		}

		result, err := mutation.VerifySuccess(snap)
		require.NoError(t, err)
		assert.False(t, result.Passed, "nil/nil snapshot is unobservable: Passed must be false")
		assert.True(t, result.IncompleteSnapshot, "nil/nil snapshot must set IncompleteSnapshot")
		assert.Empty(t, result.MissingReps)
		assert.Nil(t, result.DriftedReps)
	})

	t.Run("nil_vs_nonnil_passes_success", func(t *testing.T) {
		op := uniqueOp("U2NilNonNil")
		require.NoError(t, mutation.Register(mutation.RepresentationSet{
			Op:              op,
			Representations: []mutation.RepresentationKind{mutation.Frontmatter},
		}))

		// Before has no entry (nil lookup), After has content: After key IS
		// present (hasA=true) so the completeness guard passes; nil!=non-nil
		// counts as a change.
		snap := mutation.MutationSnapshot{
			Op:     op,
			Before: nil,
			After: map[mutation.RepresentationKind][]byte{
				mutation.Frontmatter: []byte("new-content"),
			},
		}

		result, err := mutation.VerifySuccess(snap)
		require.NoError(t, err)
		assert.False(t, result.IncompleteSnapshot, "one side present: not incomplete")
		assert.True(t, result.Passed, "nil!=non-nil: changed, VerifySuccess must pass")
		assert.Empty(t, result.MissingReps)
	})
}

// 200.010-T (CY-B B10): declaration accuracy for conditional EventsJSONL.
//
// Per deliberation D5, a declaration lists EventsJSONL as required only when the
// core path makes the event write fatal. CreateItem writes no item event, and
// ArchiveItem's event append is best-effort: on a log failure it warns and the
// archive still succeeds. For those ops a successful mutation may leave
// EventsJSONL unchanged, so EventsJSONL must be conditional (optional on
// success) while still being checked for drift on a failed mutation.

// TestU10_VerifySuccess_ConditionalEventsJSONLMayBeUnchanged verifies that
// VerifySuccess accepts a successful CreateItem with no item event and a
// successful ArchiveItem whose event append degraded to a warning.
func TestU10_VerifySuccess_ConditionalEventsJSONLMayBeUnchanged(t *testing.T) {
	t.Run("create_item_without_item_event", func(t *testing.T) {
		result, err := mutation.VerifySuccess(mutation.MutationSnapshot{
			Op: mutation.OpCreateItem,
			Before: map[mutation.RepresentationKind][]byte{
				mutation.Frontmatter: nil,
				mutation.SQLite:      nil,
				mutation.EventsJSONL: nil,
			},
			After: map[mutation.RepresentationKind][]byte{
				mutation.Frontmatter: []byte("created-fm"),
				mutation.SQLite:      []byte("created-row"),
				mutation.EventsJSONL: nil, // CreateItem wrote no item event.
			},
		})
		require.NoError(t, err)
		assert.True(t, result.Passed, "a successful CreateItem with no item event must verify; missing=%v", result.MissingReps)
		assert.Empty(t, result.MissingReps)
	})

	t.Run("archive_item_event_append_degraded_to_warning", func(t *testing.T) {
		result, err := mutation.VerifySuccess(mutation.MutationSnapshot{
			Op: mutation.OpArchiveItem,
			Before: map[mutation.RepresentationKind][]byte{
				mutation.Frontmatter: []byte("active-fm"),
				mutation.SQLite:      []byte("active-row"),
				mutation.EventsJSONL: []byte("prior-events"),
				mutation.ArchiveFile: nil,
			},
			After: map[mutation.RepresentationKind][]byte{
				mutation.Frontmatter: []byte("archived-fm"),
				mutation.SQLite:      []byte("archived-row"),
				mutation.EventsJSONL: []byte("prior-events"), // append failed: warn and skip.
				mutation.ArchiveFile: []byte("archived-copy"),
			},
		})
		require.NoError(t, err)
		assert.True(t, result.Passed, "a successful ArchiveItem whose event append degraded must verify; missing=%v", result.MissingReps)
		assert.Empty(t, result.MissingReps)
	})

	t.Run("required_representation_still_must_change", func(t *testing.T) {
		result, err := mutation.VerifySuccess(mutation.MutationSnapshot{
			Op: mutation.OpArchiveItem,
			Before: map[mutation.RepresentationKind][]byte{
				mutation.Frontmatter: []byte("active-fm"),
				mutation.SQLite:      []byte("active-row"),
				mutation.EventsJSONL: []byte("prior-events"),
				mutation.ArchiveFile: nil,
			},
			After: map[mutation.RepresentationKind][]byte{
				mutation.Frontmatter: []byte("archived-fm"),
				mutation.SQLite:      []byte("active-row"), // unchanged: required update missing.
				mutation.EventsJSONL: []byte("prior-events"),
				mutation.ArchiveFile: []byte("archived-copy"),
			},
		})
		require.NoError(t, err)
		assert.False(t, result.Passed)
		assert.Equal(t, []mutation.RepresentationKind{mutation.SQLite}, result.MissingReps,
			"only the required representation is missing; conditional EventsJSONL is never reported missing")
	})
}

// TestU10_VerifyFailure_ConditionalEventsJSONLStillReportsDrift verifies that a
// conditional EventsJSONL representation is still checked on a failed mutation:
// an event appended by a mutation that reported failure is drift.
func TestU10_VerifyFailure_ConditionalEventsJSONLStillReportsDrift(t *testing.T) {
	for _, op := range []string{mutation.OpCreateItem, mutation.OpArchiveItem} {
		t.Run(op, func(t *testing.T) {
			set, ok := mutation.Lookup(op)
			require.True(t, ok)
			before := make(map[mutation.RepresentationKind][]byte, len(set.Representations))
			after := make(map[mutation.RepresentationKind][]byte, len(set.Representations))
			for _, kind := range set.Representations {
				before[kind] = []byte("unchanged")
				after[kind] = []byte("unchanged")
			}
			after[mutation.EventsJSONL] = []byte("unchanged\nleaked-event")

			result, err := mutation.VerifyFailure(mutation.MutationSnapshot{Op: op, Before: before, After: after})
			require.NoError(t, err)
			assert.False(t, result.Passed, "an event-log change on a failed mutation must be reported")
			assert.Equal(t, []mutation.RepresentationKind{mutation.EventsJSONL}, result.DriftedReps)
		})
	}
}
