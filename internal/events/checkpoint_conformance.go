package events

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// checkpointV1AllTopLevelKeys is the case-insensitive set of every modeled
// CheckpointV1 top-level JSON tag name, derived once via reflection with
// no reserved-key subtraction. It is the single authoritative source the
// read boundary consults (147-F / U2d): unlike the create boundary, which
// must subtract checkpointV1ReservedKeys, the read boundary admits the
// full modeled set plus the reserved keys, and deriving both from one
// reflection call keeps the hand-written checkpointV1ReservedKeys literal
// from silently drifting out of the modeled field set.
var checkpointV1AllTopLevelKeys = modeledJSONTagKeys(reflect.TypeOf(CheckpointV1{}))

// CheckConformingTopLevelNamespace evaluates whether data's top-level JSON
// keys are all members of the read-boundary legal key set: every modeled
// CheckpointV1 top-level key (checkpointV1AllTopLevelKeys), which already
// includes the reserved disposition* keys — legal to READ even though they
// are excluded from the create-time legal set, because an already-abandoned
// checkpoint must stay readable. Unlike checkClosedSchemaNamespace (the
// create-boundary gate), this performs no reserved-status-value check:
// status: "abandoned" is a legal read-boundary value.
//
// The predicate is round-trip safety, not merely "no unknown keys" (147-F /
// U2c): any two top-level entries whose keys are strings.EqualFold-equal —
// including exact duplicates — make the document non-conforming, reported
// as "duplicate:<lowercased key>", because a rewrite would silently drop one
// member's bytes. The same rule recurses one level into a case-insensitive
// "progress" match (147-F / U2b, U2e) and into a case-insensitive "context"
// match (147-F / U2g, widened to any fold-variant spelling by U2h — the
// same routing rule encoding/json itself applies when decoding into
// CheckpointV1.Context), reporting nested offenders as
// "duplicate:progress.<key>" and "duplicate:context.<key>" respectively; a
// non-object progress or context value is not a conformance failure.
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

	// Pass 1: detect exact or Unicode-fold-equal top-level key pairs via
	// pairwise comparison, mirroring duplicateNestedMemberKeys below.
	// strings.ToLower is NOT a safe canonicalization key for this check:
	// strings.ToLower and strings.EqualFold disagree for some Unicode input
	// (e.g. "status" vs "\u017ftatus", long s U+017F, are EqualFold-equal but
	// not ToLower-equal), so a ToLower-keyed map could miss a duplicate pair
	// this function's own contract requires it to catch — and because
	// isFoldKeyIn (below) independently recognizes each EqualFold-equal
	// spelling as a legitimate occurrence of the same modeled field, the
	// miss would silently accept a round-trip-unsafe document as conforming.
	isDuplicate := make([]bool, len(entries))
	var duplicateNames []string
	reportedDuplicate := map[string]struct{}{}
	for i, a := range entries {
		for j := i + 1; j < len(entries); j++ {
			b := entries[j]
			if a.key != b.key && !strings.EqualFold(a.key, b.key) {
				continue
			}
			isDuplicate[i] = true
			isDuplicate[j] = true
			lower := strings.ToLower(a.key)
			if _, already := reportedDuplicate[lower]; !already {
				reportedDuplicate[lower] = struct{}{}
				duplicateNames = append(duplicateNames, "duplicate:"+lower)
			}
		}
	}

	unknown := append([]string(nil), duplicateNames...)
	for i, e := range entries {
		if isDuplicate[i] {
			continue
		}

		if strings.EqualFold(e.key, "progress") {
			unknown = append(unknown, unknownNestedProgressKeys(e.value)...)
			unknown = append(unknown, duplicateNestedMemberKeys(e.value, "progress")...)
			continue
		}
		if strings.EqualFold(e.key, "context") {
			unknown = append(unknown, duplicateNestedMemberKeys(e.value, "context")...)
			continue
		}
		if isFoldKeyIn(e.key, checkpointV1AllTopLevelKeys) {
			continue
		}
		unknown = append(unknown, e.key)
	}

	if len(unknown) == 0 {
		return nil
	}
	return &backlogiterrors.CheckpointNonConformingError{Fields: dedupeSorted(unknown)}
}

// duplicateNestedMemberKeys walks raw — the value of a nested object member
// named prefix ("progress" or "context") — as an ordered token stream and
// reports round-trip-unsafe member pairs as "duplicate:<prefix>.<key>".
//
// Exact duplicate decoded member names are always refused: an
// escape-equivalent spelling like \u0066oo decodes to the same string as
// foo, so a raw-byte comparison after decoding still catches it. Fold
// variants (case differs, not byte-equal) are refused only when refuseFold
// permits: for "progress", every fold variant is refused because
// CheckpointProgress has no open extension namespace; for "context", a fold
// variant is refused only when one of the pair aliases a modeled
// CheckpointContext field (147-F / U2e, U2g).
func duplicateNestedMemberKeys(raw []byte, prefix string) []string {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	entries, err := decodeTopLevelEntries(raw)
	if err != nil {
		return nil
	}

	var offenders []string
	reported := map[string]struct{}{}
	for i, a := range entries {
		for _, b := range entries[i+1:] {
			exact := a.key == b.key
			fold := !exact && strings.EqualFold(a.key, b.key)
			if !exact && !fold {
				continue
			}
			if fold && prefix == "context" && !isModeledContextKey(a.key) && !isModeledContextKey(b.key) {
				continue // distinct unmodeled fold variant: preserved, not an offender
			}
			lower := strings.ToLower(a.key)
			if _, already := reported[lower]; already {
				continue
			}
			reported[lower] = struct{}{}
			offenders = append(offenders, "duplicate:"+prefix+"."+lower)
		}
	}
	return offenders
}
