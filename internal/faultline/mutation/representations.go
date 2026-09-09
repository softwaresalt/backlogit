// Package mutation provides the S5 representation declaration model:
// a declarative surface where each mutating operation lists the
// representations it must update, and a registry binding op names to
// their declared representation sets.
package mutation

import (
	"errors"
	"fmt"
)

// RepresentationKind identifies a single representation layer that a
// mutating operation may affect.
type RepresentationKind string

const (
	// Frontmatter is the YAML front-matter in a work-item Markdown file.
	Frontmatter RepresentationKind = "frontmatter"
	// SQLite is the SQLite projection of the work-item record.
	SQLite RepresentationKind = "sqlite"
	// EventsJSONL is the append-only events journal in JSONL format.
	EventsJSONL RepresentationKind = "events_jsonl"
	// ArchiveFile is the archived copy of a work-item Markdown file.
	ArchiveFile RepresentationKind = "archive_file"
	// ShipmentRecord is the shipment metadata record for a work item.
	ShipmentRecord RepresentationKind = "shipment_record"
	// CommitLink is the commit-hash provenance link attached to a work item.
	CommitLink RepresentationKind = "commit_link"
)

// validKinds is the closed set of all defined RepresentationKind values used
// for schema validation.
var validKinds = map[RepresentationKind]struct{}{
	Frontmatter:    {},
	SQLite:         {},
	EventsJSONL:    {},
	ArchiveFile:    {},
	ShipmentRecord: {},
	CommitLink:     {},
}

// ErrEmptyOp is returned by Validate when Op is empty.
var ErrEmptyOp = errors.New("mutation: op name must not be empty")

// ErrEmptyRepresentations is returned by Validate when Representations is nil
// or empty.
var ErrEmptyRepresentations = errors.New("mutation: representations must not be empty")

// ErrUnknownKind is returned by Validate when a RepresentationKind value is
// not in the defined set.
var ErrUnknownKind = errors.New("mutation: unknown representation kind")

// ErrDuplicateKind is returned by Validate when the same RepresentationKind
// appears more than once in the Representations list.
var ErrDuplicateKind = errors.New("mutation: duplicate representation kind")

// RepresentationSet declares the representations that a single mutating
// operation is responsible for updating.
type RepresentationSet struct {
	// Op is the stable human-readable name of the mutating operation.
	Op string
	// Representations is the non-empty ordered list of representation layers
	// the operation must update. Each kind must appear at most once.
	Representations []RepresentationKind
}

// Validate checks that the RepresentationSet satisfies the schema invariants:
// Op must be non-empty, Representations must be non-nil and non-empty, every
// kind must be from the defined set, and no kind may appear more than once.
// Returns one of ErrEmptyOp, ErrEmptyRepresentations, a wrapped ErrUnknownKind,
// or a wrapped ErrDuplicateKind on validation failure.
func (r RepresentationSet) Validate() error {
	if r.Op == "" {
		return ErrEmptyOp
	}
	if len(r.Representations) == 0 {
		return ErrEmptyRepresentations
	}
	seen := make(map[RepresentationKind]struct{}, len(r.Representations))
	for _, k := range r.Representations {
		if _, ok := validKinds[k]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownKind, k)
		}
		if _, dup := seen[k]; dup {
			return fmt.Errorf("%w: %q", ErrDuplicateKind, k)
		}
		seen[k] = struct{}{}
	}
	return nil
}
