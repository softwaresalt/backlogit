package mutation

import (
	"bytes"
	"fmt"
	"sort"
)

// MutationSnapshot captures the state of representations before and after a
// mutation, keyed by RepresentationKind. Nil entries in Before or After are
// treated as empty byte slices during comparison.
type MutationSnapshot struct {
	// Op is the name of the mutating operation being verified.
	Op string
	// Before holds the serialised state of each representation prior to the
	// mutation. A missing key is treated as nil (no content) when the
	// corresponding key is present in After; if both Before and After lack the
	// key, the result carries IncompleteSnapshot=true.
	Before map[RepresentationKind][]byte
	// After holds the serialised state of each representation following the
	// mutation. A missing key is treated as nil (no content) when the
	// corresponding key is present in Before; if both Before and After lack the
	// key, the result carries IncompleteSnapshot=true.
	After map[RepresentationKind][]byte
}

// VerificationResult is the outcome of a postcondition verification pass.
type VerificationResult struct {
	// Op is the name of the mutating operation that was verified.
	Op string
	// Passed is true when the verification constraint was fully satisfied.
	Passed bool
	// IncompleteSnapshot is true when the snapshot did not contain an entry
	// (even a nil one) for one or more declared representation kinds in both
	// Before and After. When true, Passed is always false and neither
	// MissingReps nor DriftedReps is populated — the snapshot does not carry
	// enough information to make a meaningful assertion.
	IncompleteSnapshot bool
	// DriftedReps lists declared representation kinds that changed
	// unexpectedly on a failure path (populated by VerifyFailure).
	// Entries are sorted by kind string value for stable output.
	DriftedReps []RepresentationKind
	// MissingReps lists declared representation kinds that were expected to
	// change but did not on a success path (populated by VerifySuccess).
	// Entries are sorted by kind string value for stable output.
	MissingReps []RepresentationKind
}

// VerifySuccess verifies that all declared representations for snap.Op differ
// between snap.Before and snap.After, indicating that a mutation fully
// committed. A representation kind is considered missing (not updated) when
// its Before and After bytes are equal. The result's Passed field is true only
// when every declared kind was changed. MissingReps contains the kinds that
// were not updated, sorted by kind string value.
//
// If any declared representation is absent from both Before and After, the
// result carries IncompleteSnapshot=true and Passed=false; MissingReps is not
// populated.
//
// Returns a non-nil error when snap.Op is not registered.
func VerifySuccess(snap MutationSnapshot) (VerificationResult, error) {
	set, ok := Lookup(snap.Op)
	if !ok {
		return VerificationResult{}, fmt.Errorf("mutation.VerifySuccess: op %q: %w", snap.Op, ErrOpNotRegistered)
	}

	// Completeness guard: bytes.Equal(nil, nil) == true, so a snapshot whose
	// Before and After both lack a declared key would incorrectly report that
	// representation as changed (Passed: true with no real observation). Require
	// each declared representation to be present in at least one direction.
	for _, k := range set.Representations {
		_, hasB := snap.Before[k]
		_, hasA := snap.After[k]
		if !hasB && !hasA {
			return VerificationResult{Op: snap.Op, IncompleteSnapshot: true}, nil
		}
	}

	missing := make([]RepresentationKind, 0, len(set.Representations))
	for _, k := range set.Representations {
		if bytes.Equal(snap.Before[k], snap.After[k]) {
			missing = append(missing, k)
		}
	}

	sort.SliceStable(missing, func(i, j int) bool {
		return string(missing[i]) < string(missing[j])
	})

	return VerificationResult{
		Op:          snap.Op,
		Passed:      len(missing) == 0,
		MissingReps: missing,
	}, nil
}

// VerifyFailure verifies that all declared representations for snap.Op are
// identical between snap.Before and snap.After, indicating that a failed
// mutation was fully rolled back or never applied. A representation kind is
// considered drifted (unexpectedly changed) when its Before and After bytes
// differ. The result's Passed field is true only when every declared kind is
// unchanged. DriftedReps contains the kinds that changed, sorted by kind
// string value.
//
// If any declared representation is absent from both Before and After, the
// result carries IncompleteSnapshot=true and Passed=false; DriftedReps is not
// populated.
//
// Returns a non-nil error when snap.Op is not registered.
func VerifyFailure(snap MutationSnapshot) (VerificationResult, error) {
	set, ok := Lookup(snap.Op)
	if !ok {
		return VerificationResult{}, fmt.Errorf("mutation.VerifyFailure: op %q: %w", snap.Op, ErrOpNotRegistered)
	}

	// Completeness guard: bytes.Equal(nil, nil) == true, so a snapshot whose
	// Before and After both lack a declared key would incorrectly claim rollback
	// was verified (Passed: true) when no state was actually observed. Require
	// each declared representation to be present in at least one direction.
	for _, k := range set.Representations {
		_, hasB := snap.Before[k]
		_, hasA := snap.After[k]
		if !hasB && !hasA {
			return VerificationResult{Op: snap.Op, IncompleteSnapshot: true}, nil
		}
	}

	drifted := make([]RepresentationKind, 0, len(set.Representations))
	for _, k := range set.Representations {
		if !bytes.Equal(snap.Before[k], snap.After[k]) {
			drifted = append(drifted, k)
		}
	}

	sort.SliceStable(drifted, func(i, j int) bool {
		return string(drifted[i]) < string(drifted[j])
	})

	return VerificationResult{
		Op:          snap.Op,
		Passed:      len(drifted) == 0,
		DriftedReps: drifted,
	}, nil
}
