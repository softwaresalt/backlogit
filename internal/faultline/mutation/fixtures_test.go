package mutation_test

import (
	"fmt"

	"github.com/softwaresalt/backlogit/internal/faultline/mutation"
)

// partialWriteSnap creates a MutationSnapshot simulating a partial write for
// the given op. All allKinds are set to "before-<kind>" in Before. In After,
// only the updatedKinds are written to "after-<kind>"; the remaining allKinds
// are copied from Before (unchanged), modelling a crash after a subset write.
func partialWriteSnap(op string, allKinds []mutation.RepresentationKind, updatedKinds []mutation.RepresentationKind) mutation.MutationSnapshot {
	before := make(map[mutation.RepresentationKind][]byte, len(allKinds))
	after := make(map[mutation.RepresentationKind][]byte, len(allKinds))

	for _, k := range allKinds {
		before[k] = []byte(fmt.Sprintf("before-%s", k))
	}

	// Build a lookup set for the kinds that were actually written.
	written := make(map[mutation.RepresentationKind]struct{}, len(updatedKinds))
	for _, k := range updatedKinds {
		written[k] = struct{}{}
	}

	for _, k := range allKinds {
		if _, ok := written[k]; ok {
			after[k] = []byte(fmt.Sprintf("after-%s", k))
		} else {
			// Not written: After retains the Before bytes (partial write).
			after[k] = before[k]
		}
	}

	return mutation.MutationSnapshot{
		Op:     op,
		Before: before,
		After:  after,
	}
}

// staleIndexSnap creates a MutationSnapshot simulating a stale Before
// snapshot: Before contains "stale-v1-<kind>" bytes and After contains
// "fresh-v2-<kind>" bytes. The stale marker distinguishes Before from After
// to exercise the verifier with an outdated baseline.
func staleIndexSnap(op string, kinds []mutation.RepresentationKind) mutation.MutationSnapshot {
	before := make(map[mutation.RepresentationKind][]byte, len(kinds))
	after := make(map[mutation.RepresentationKind][]byte, len(kinds))
	for _, k := range kinds {
		before[k] = []byte(fmt.Sprintf("stale-v1-%s", k))
		after[k] = []byte(fmt.Sprintf("fresh-v2-%s", k))
	}
	return mutation.MutationSnapshot{
		Op:     op,
		Before: before,
		After:  after,
	}
}

// oldIndexSnap creates a MutationSnapshot where Before represents a very old
// (2+ generations stale) snapshot with "ancient-v0-<kind>" bytes and After
// is the current freshly written state with "current-v10-<kind>" bytes.
func oldIndexSnap(op string, kinds []mutation.RepresentationKind) mutation.MutationSnapshot {
	before := make(map[mutation.RepresentationKind][]byte, len(kinds))
	after := make(map[mutation.RepresentationKind][]byte, len(kinds))
	for _, k := range kinds {
		before[k] = []byte(fmt.Sprintf("ancient-v0-%s", k))
		after[k] = []byte(fmt.Sprintf("current-v10-%s", k))
	}
	return mutation.MutationSnapshot{
		Op:     op,
		Before: before,
		After:  after,
	}
}

// indeterminateAtomicSnaps creates MutationSnapshots for the two possible
// outcomes of an indeterminate atomic write:
//
//   - committed: Before is "before-<kind>", After is "after-<kind>" for every
//     kind — all representations changed, so VerifySuccess returns Passed=true.
//   - rolledBack: Before and After both carry "before-<kind>" bytes — no
//     representation changed, so VerifyFailure returns Passed=true.
func indeterminateAtomicSnaps(op string, kinds []mutation.RepresentationKind) (committed mutation.MutationSnapshot, rolledBack mutation.MutationSnapshot) {
	beforeC := make(map[mutation.RepresentationKind][]byte, len(kinds))
	afterC := make(map[mutation.RepresentationKind][]byte, len(kinds))
	beforeR := make(map[mutation.RepresentationKind][]byte, len(kinds))
	afterR := make(map[mutation.RepresentationKind][]byte, len(kinds))

	for _, k := range kinds {
		beforeC[k] = []byte(fmt.Sprintf("before-%s", k))
		afterC[k] = []byte(fmt.Sprintf("after-%s", k))
		// Rolled-back case: both Before and After carry the same content.
		beforeR[k] = []byte(fmt.Sprintf("before-%s", k))
		afterR[k] = []byte(fmt.Sprintf("before-%s", k))
	}

	committed = mutation.MutationSnapshot{
		Op:     op,
		Before: beforeC,
		After:  afterC,
	}
	rolledBack = mutation.MutationSnapshot{
		Op:     op,
		Before: beforeR,
		After:  afterR,
	}
	return committed, rolledBack
}
