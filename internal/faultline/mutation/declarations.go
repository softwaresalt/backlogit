package mutation

// init registers the representation sets for existing mutating operations in
// internal/core. These declarations describe the existing behavior of each
// operation and are the primary verification surface for S5 postcondition
// checks. No production behavior is changed.
func init() {
	mustRegister(RepresentationSet{
		Op:              "CreateItem",
		Representations: []RepresentationKind{Frontmatter, SQLite, EventsJSONL},
	})
	mustRegister(RepresentationSet{
		Op:              "ArchiveItem",
		Representations: []RepresentationKind{Frontmatter, SQLite, EventsJSONL, ArchiveFile},
	})
}

// mustRegister panics if the set fails to register, so that a broken
// declaration is discovered immediately at package initialization rather than
// silently at runtime.
func mustRegister(set RepresentationSet) {
	if err := Register(set); err != nil {
		panic("mutation.declarations: " + err.Error())
	}
}
