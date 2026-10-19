package mutation

import "fmt"

// Op name constants for the mutating operations declared in this package.
// Callers should use these constants when invoking Lookup, VerifySuccess,
// or VerifyFailure to avoid typos and to maintain a stable API reference.
const (
	// OpCreateItem is the op name for the CreateItem mutation.
	OpCreateItem = "CreateItem"
	// OpArchiveItem is the op name for the ArchiveItem mutation.
	OpArchiveItem = "ArchiveItem"
)

// init registers the representation sets for existing mutating operations in
// internal/core. These declarations describe the existing behavior of each
// operation and are the primary verification surface for S5 postcondition
// checks. No production behavior is changed.
func init() {
	// Per CY deliberation D5, a declaration lists EventsJSONL as unconditional
	// only when the core path makes the event write fatal. CreateItem writes no
	// item event, and ArchiveItem's archive event is best-effort (on a log
	// failure it warns and the archive still succeeds), so both mark EventsJSONL
	// conditional: a successful mutation need not change it, and a failed
	// mutation is still checked for event-log drift.
	mustRegister(RepresentationSet{
		Op:              OpCreateItem,
		Representations: []RepresentationKind{Frontmatter, SQLite, EventsJSONL},
		Conditional:     []RepresentationKind{EventsJSONL},
	})
	mustRegister(RepresentationSet{
		Op:              OpArchiveItem,
		Representations: []RepresentationKind{Frontmatter, SQLite, EventsJSONL, ArchiveFile},
		Conditional:     []RepresentationKind{EventsJSONL},
	})
}

// mustRegister panics if the set fails to register, so that a broken
// declaration is discovered immediately at package initialization rather than
// silently at runtime.
func mustRegister(set RepresentationSet) {
	if err := Register(set); err != nil {
		panic(fmt.Errorf("mutation.declarations: %w", err))
	}
}
