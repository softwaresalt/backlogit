package events

import (
	"fmt"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// CheckConformingTopLevelNamespace evaluates whether data's top-level JSON
// keys are all members of the read-boundary legal key set: CheckpointV1's
// modeled top-level keys (checkpointV1TopLevelKeys) plus the reserved
// disposition* keys (checkpointV1ReservedKeys), which remain legal to READ —
// an already-abandoned checkpoint must stay readable — even though they are
// excluded from the create-time legal set. Unlike checkClosedSchemaNamespace
// (the create-boundary gate), this performs no reserved-status-value check:
// status: "abandoned" is a legal read-boundary value. This unit does not yet
// recurse into a nested "progress" object — that is U2b's delta.
//
// On success (every top-level key legal) it returns nil. On rejection it
// returns *backlogiterrors.CheckpointNonConformingError directly, naming
// every offending key path — never wrapped further, so errors.As recovers it
// without an extra unwrap hop (147-F / U2).
func CheckConformingTopLevelNamespace(data []byte) error {
	entries, err := decodeTopLevelEntries(data)
	if err != nil {
		return fmt.Errorf("checkpoint conformance decode: %w", err)
	}

	var unknown []string
	for _, e := range entries {
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
