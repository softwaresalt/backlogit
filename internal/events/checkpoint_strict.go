package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// checkpointV1TopLevelKeys is the case-insensitive set of CheckpointV1's
// modeled top-level JSON tag names, derived once via reflection so the
// create-boundary closed-namespace check (146.011-T / U4) cannot silently
// desynchronize from the struct itself as fields are added or renamed.
var checkpointV1TopLevelKeys = modeledJSONTagKeys(reflect.TypeOf(CheckpointV1{}))

// checkpointProgressKeys is the case-insensitive set of CheckpointProgress's
// modeled JSON tag names, used to validate the nested progress object.
var checkpointProgressKeys = modeledJSONTagKeys(reflect.TypeOf(CheckpointProgress{}))

// checkClosedSchemaNamespace enforces the closed CheckpointV1 top-level and
// nested-progress schema namespace at the CreateCheckpoint create boundary
// (146.011-T / U4). Callers MUST run this only after ParseCheckpoint (pass 1)
// has already succeeded on the same bytes, so a genuine shape error keeps its
// ErrCheckpointCorrupt classification instead of being misreported as an
// unknown-field rejection. Pass 2 (this function) decodes the same original
// bytes into a map[string]json.RawMessage and diffs every top-level key
// against checkpointV1TopLevelKeys; any key whose name case-insensitively
// matches "progress" is not itself flagged (progress is a legal top-level
// field) but is instead recursed into, diffing its own keys against
// checkpointProgressKeys.
//
// NESTED RECURSION IS DETERMINISTIC (PR #372 remediation): every raw
// top-level entry whose key case-insensitively matches "progress" is
// inspected — never a single map-iteration-selected entry — because a dump
// may legally carry both "progress" and "Progress" as two distinct map
// entries, and choosing one by iterating the raw map would select the
// winner in randomized map iteration order, so the same bytes would be
// accepted on one run and rejected on the next. The unknown nested paths
// found under ALL matching entries are unioned into one sorted,
// de-duplicated Fields slice using the "progress.<key>" path form. A
// matching entry whose raw value is JSON null is skipped (there are no
// nested keys to evaluate); the recursion is skipped entirely when no
// top-level key matches "progress" at all.
//
// On success (namespace closed) it returns nil. On rejection it returns the
// typed *backlogiterrors.CheckpointUnknownFieldError DIRECTLY — callers must
// not wrap it again, so errors.As continues to recover it without an extra
// unwrap hop.
func checkClosedSchemaNamespace(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// Pass 1 (ParseCheckpoint) already succeeded on these same bytes, so
		// this decode is expected to succeed too; propagate rather than mask
		// if it somehow does not.
		return fmt.Errorf("checkpoint strict decode: %w", err)
	}

	var unknown []string
	for k, v := range raw {
		if strings.EqualFold(k, "progress") {
			nested := unknownNestedProgressKeys(v)
			unknown = append(unknown, nested...)
			continue
		}
		if _, ok := checkpointV1TopLevelKeys[strings.ToLower(k)]; !ok {
			unknown = append(unknown, k)
		}
	}

	if len(unknown) == 0 {
		return nil
	}
	return &backlogiterrors.CheckpointUnknownFieldError{Fields: dedupeSorted(unknown)}
}

// unknownNestedProgressKeys returns the "progress.<key>" paths for every key
// in raw that does not case-insensitively match a CheckpointProgress modeled
// field. raw is a single top-level entry's value whose key case-insensitively
// matched "progress". A JSON null value is skipped (no nested keys to
// evaluate). A value that is present but does not decode as a JSON object
// (and is not null) is also skipped: pass 1 (ParseCheckpoint) already
// succeeded, so whichever "progress"-matching entry Go's own field matching
// actually assigned decoded as a valid *CheckpointProgress; a sibling
// differently-cased entry with a non-object, non-null shape carries no
// nested keys this closed-namespace check is responsible for evaluating.
func unknownNestedProgressKeys(raw json.RawMessage) []string {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(raw, &nested); err != nil {
		return nil
	}
	var unknown []string
	for k := range nested {
		if _, ok := checkpointProgressKeys[strings.ToLower(k)]; !ok {
			unknown = append(unknown, "progress."+k)
		}
	}
	return unknown
}

// dedupeSorted returns a sorted, de-duplicated copy of fields.
func dedupeSorted(fields []string) []string {
	sort.Strings(fields)
	out := make([]string, 0, len(fields))
	for i, f := range fields {
		if i > 0 && f == fields[i-1] {
			continue
		}
		out = append(out, f)
	}
	return out
}
