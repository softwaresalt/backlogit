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
	// mutation. A missing key is treated as nil (no content).
	Before map[RepresentationKind][]byte
	// After holds the serialised state of each representation following the
	// mutation. A missing key is treated as nil (no content).
	After map[RepresentationKind][]byte
}

// VerificationResult is the outcome of a postcondition verification pass.
type VerificationResult struct {
	// Op is the name of the mutating operation that was verified.
	Op string
	// Passed is true when the verification constraint was fully satisfied.
	Passed bool
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
// Returns a non-nil error when snap.Op is not registered.
func VerifySuccess(snap MutationSnapshot) (VerificationResult, error) {
	set, ok := Lookup(snap.Op)
	if !ok {
		return VerificationResult{}, fmt.Errorf("mutation: op %q not registered", snap.Op)
	}

	var missing []RepresentationKind
	for _, k := range set.Representations {
		if bytes.Equal(snap.Before[k], snap.After[k]) {
			missing = append(missing, k)
		}
	}

	sort.Slice(missing, func(i, j int) bool {
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
// Returns a non-nil error when snap.Op is not registered.
func VerifyFailure(snap MutationSnapshot) (VerificationResult, error) {
	set, ok := Lookup(snap.Op)
	if !ok {
		return VerificationResult{}, fmt.Errorf("mutation: op %q not registered", snap.Op)
	}

	var drifted []RepresentationKind
	for _, k := range set.Representations {
		if !bytes.Equal(snap.Before[k], snap.After[k]) {
			drifted = append(drifted, k)
		}
	}

	sort.Slice(drifted, func(i, j int) bool {
		return string(drifted[i]) < string(drifted[j])
	})

	return VerificationResult{
		Op:          snap.Op,
		Passed:      len(drifted) == 0,
		DriftedReps: drifted,
	}, nil
}
