package events

import (
	"fmt"
	"strings"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// CheckConformingTopLevelNamespace evaluates whether data's top-level JSON
// keys are all members of the read-boundary legal key set: CheckpointV1's
// modeled top-level keys (checkpointV1TopLevelKeys) plus the reserved
// disposition* keys (checkpointV1ReservedKeys), which remain legal to READ —
// an already-abandoned checkpoint must stay readable — even though they are
// excluded from the create-time legal set. Unlike checkClosedSchemaNamespace
// (the create-boundary gate), this performs no reserved-status-value check:
// status: "abandoned" is a legal read-boundary value.
//
// The predicate is round-trip safety, not merely "no unknown keys" (147-F /
// U2c): any two top-level entries whose keys are strings.EqualFold-equal —
// including exact duplicates — make the document non-conforming, reported
// as "duplicate:<lowercased key>", because a rewrite would silently drop one
// member's bytes. A case-insensitive match on "progress" that is not itself
// a duplicate recurses into the nested object via unknownNestedProgressKeys,
// reporting offenders in "progress.<key>" form (147-F / U2b); a non-object
// progress value is not a conformance failure.
//
// On success (no offenders found) it returns nil. On rejection it returns
// *backlogiterrors.CheckpointNonConformingError directly, naming every
// offending key path — never wrapped further, so errors.As recovers it
// without an extra unwrap hop (147-F / U2).
func CheckConformingTopLevelNamespace(data []byte) error {
	entries, err := decodeTopLevelEntries(data)
	if err != nil {
		return fmt.Errorf("checkpoint conformance decode: %w", err)
	}

	var unknown []string
	seen := map[string]struct{}{}
	for _, e := range entries {
		lower := strings.ToLower(e.key)
		if _, dup := seen[lower]; dup {
			unknown = append(unknown, "duplicate:"+lower)
			continue
		}
		seen[lower] = struct{}{}

		if strings.EqualFold(e.key, "progress") {
			unknown = append(unknown, unknownNestedProgressKeys(e.value)...)
			continue
		}
		if isFoldKeyIn(e.key, checkpointV1TopLevelKeys) || isFoldKeyIn(e.key, checkpointV1ReservedKeys) {
			continue
		}
		unknown = append(unknown, e.key)
	}

	if len(unknown) == 0 {
		return nil
	}
	return &backlogiterrors.CheckpointNonConformingError{Fields: dedupeSorted(unknown)}
}
