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

// checkpointV1ReservedKeys are CheckpointV1 top-level fields that exist in
// the schema but are administrative/disposition-owned: per
// docs/design-docs/checkpoint-administrative-disposition.md, they are
// "populated only by AbandonCheckpoint" (mirrored by QuarantineCheckpoint's
// sidecar). Copilot review on PR #373 flagged that admitting them at the
// CreateCheckpoint boundary lets a caller forge disposition:"abandoned"
// directly; a later legitimate AbandonCheckpoint call on that same file then
// silently no-ops via its idempotent-already-abandoned short-circuit,
// producing NO audit append — exactly the class of evidence loss this
// shipment exists to close. These four keys are therefore excluded from the
// create-time legal set and treated as unknown fields if supplied at create,
// even though they remain legal (and required) for AbandonCheckpoint's own
// direct rewrite of the file, which never calls checkClosedSchemaNamespace.
var checkpointV1ReservedKeys = map[string]struct{}{
	"disposition":          {},
	"disposition_reason":   {},
	"disposition_operator": {},
	"disposition_at":       {},
}

// checkpointReservedStatusValues are Status values that must never be
// supplied directly at create, even though "status" itself remains a legal
// top-level key: they represent a governed disposition outcome that must
// only be reached via its own audited operation. "abandoned" is the only
// such value today. Copilot review on PR #373 found that excluding the four
// disposition* KEYS alone was insufficient: a caller could still submit
// status:"abandoned" with no disposition fields at all. That dump passes
// both checkClosedSchemaNamespace (status is a legal key) and
// ValidateCheckpoint (abandoned is a legal Status enum value), persisting a
// checkpoint that LOOKS abandoned but was never audited — and it can never
// be repaired through the governed operation afterward, because
// AbandonCheckpoint refuses any non-"active" status
// (ErrCheckpointNotActive) before it would ever reach its own
// already-abandoned idempotent check. "resolved" is NOT reserved: closing a
// checkpoint via resolve carries no audit-trail requirement, so a caller
// legitimately importing an already-closed session may supply it directly.
var checkpointReservedStatusValues = map[string]struct{}{
	"abandoned": {},
}

// checkpointV1TopLevelKeys is the case-insensitive set of CheckpointV1's
// modeled top-level JSON tag names MINUS checkpointV1ReservedKeys, derived
// once via reflection so the create-boundary closed-namespace check
// (146.011-T / U4) cannot silently desynchronize from the struct itself as
// fields are added or renamed.
var checkpointV1TopLevelKeys = deriveCreateableTopLevelKeys()

func deriveCreateableTopLevelKeys() map[string]struct{} {
	keys := modeledJSONTagKeys(reflect.TypeOf(CheckpointV1{}))
	for k := range checkpointV1ReservedKeys {
		delete(keys, k)
	}
	return keys
}

// checkpointProgressKeys is the case-insensitive set of CheckpointProgress's
// modeled JSON tag names, used to validate the nested progress object.
var checkpointProgressKeys = modeledJSONTagKeys(reflect.TypeOf(CheckpointProgress{}))

// topLevelEntry is one member of a decoded top-level JSON object in source
// order, preserving exact-duplicate keys.
type topLevelEntry struct {
	key   string
	value json.RawMessage
}

// decodeTopLevelEntries walks data as a JSON object token stream and returns
// EVERY top-level member in source order, including exact-case duplicate
// keys. This is deliberately NOT a map[string]json.RawMessage decode: Go's
// own json.Unmarshal into a map silently collapses exact-duplicate keys to a
// single last-value-wins entry before any caller-level code ever runs, so a
// dump carrying two identically-cased "progress" members (one with an
// unknown nested key, one clean) would have its dirty entry vanish before
// checkClosedSchemaNamespace's loop ever saw it — the same category of
// silent evidence loss the mixed-case alias handling was already written to
// prevent, just triggered by an exact duplicate instead of a case variant.
// Flagged by Copilot review on PR #373.
func decodeTopLevelEntries(data []byte) ([]topLevelEntry, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim.String() != "{" {
		return nil, fmt.Errorf("checkpoint strict decode: expected a JSON object, got %v", tok)
	}
	var entries []topLevelEntry
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("checkpoint strict decode: expected a string key, got %v", keyTok)
		}
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		entries = append(entries, topLevelEntry{key: key, value: v})
	}
	return entries, nil
}

// checkClosedSchemaNamespace enforces the closed CheckpointV1 top-level and
// nested-progress schema namespace at the CreateCheckpoint create boundary
// (146.011-T / U4). Callers MUST run this only after ParseCheckpoint (pass 1)
// has already succeeded on the same bytes, so a genuine shape error keeps its
// ErrCheckpointCorrupt classification instead of being misreported as an
// unknown-field rejection. Pass 2 (this function) walks the same original
// bytes as an ordered token stream (decodeTopLevelEntries, never a map decode
// — see its doc comment for why) and diffs every top-level key against
// checkpointV1TopLevelKeys (which already excludes the reserved
// disposition* fields); any key whose name case-insensitively matches
// "progress" is not itself flagged (progress is a legal top-level field) but
// is instead recursed into, diffing its own keys against
// checkpointProgressKeys. A "status" entry whose value is a reserved literal
// (checkpointReservedStatusValues) is ALSO flagged even though "status" is
// itself a legal key: the key is legal, but that specific value represents a
// governed disposition outcome that must only be reached via its own
// audited operation (see checkpointReservedStatusValues' doc comment).
//
// NESTED RECURSION IS DETERMINISTIC (PR #372 remediation): every raw
// top-level entry whose key case-insensitively matches "progress" is
// inspected — never a single map-iteration-selected entry — because a dump
// may legally carry both "progress" and "Progress" as two distinct entries
// (or even the same key twice), and choosing one by iterating an unordered
// map would select the winner in randomized map iteration order, so the
// same bytes would be accepted on one run and rejected on the next. The
// unknown nested paths found under ALL matching entries are unioned into one
// sorted, de-duplicated Fields slice using the "progress.<key>" path form. A
// matching entry whose raw value is JSON null is skipped (there are no
// nested keys to evaluate); the recursion is skipped entirely when no
// top-level key matches "progress" at all.
//
// On success (namespace closed) it returns nil. On rejection it returns the
// typed *backlogiterrors.CheckpointUnknownFieldError DIRECTLY — callers must
// not wrap it again, so errors.As continues to recover it without an extra
// unwrap hop.
func checkClosedSchemaNamespace(data []byte) error {
	entries, err := decodeTopLevelEntries(data)
	if err != nil {
		// Pass 1 (ParseCheckpoint) already succeeded on these same bytes, so
		// this decode is expected to succeed too; propagate rather than mask
		// if it somehow does not.
		return fmt.Errorf("checkpoint strict decode: %w", err)
	}

	var unknown []string
	for _, e := range entries {
		if strings.EqualFold(e.key, "progress") {
			unknown = append(unknown, unknownNestedProgressKeys(e.value)...)
			continue
		}
		if !isFoldKeyIn(e.key, checkpointV1TopLevelKeys) {
			unknown = append(unknown, e.key)
			continue
		}
		if strings.EqualFold(e.key, "status") && isReservedStatusValue(e.value) {
			unknown = append(unknown, e.key)
		}
	}

	if len(unknown) == 0 {
		return nil
	}
	return &backlogiterrors.CheckpointUnknownFieldError{Fields: dedupeSorted(unknown)}
}

// contextDuplicateCreateKeys walks the context object of data (a JSON
// checkpoint bytes buffer that has already passed ParseCheckpoint) and
// returns the sorted, de-duplicated set of context member names that appear
// more than once — either as byte-equal exact duplicates or as case-fold
// aliases of a modeled CheckpointContext field. Unmodeled context keys that
// differ only by case are NOT rejected here (the context namespace is open),
// matching the read-boundary rule in duplicateNestedMemberKeys for "context".
// Returns nil when no duplicate-class key pairs are found.
// contextDuplicateCreateKeys walks ALL top-level entries that case-insensitively
// match "context" (never just the first one) and returns the sorted,
// de-duplicated set of context member names that appear more than once across
// the union of those entries. This mirrors the all-alias recursion used by
// the "progress" path in checkClosedSchemaNamespace (FINDING-2 of the 148-F
// adversarial review): a payload with "context" (clean) + "Context" (dirty)
// would pass if only the first entry were scanned, because encoding/json uses
// last-wins semantics and populates cp.Context from the last fold-match.
func contextDuplicateCreateKeys(data []byte) []string {
	entries, err := decodeTopLevelEntries(data)
	if err != nil {
		return nil
	}
	// Collect nested members from ALL top-level entries whose keys
	// case-insensitively match "context", then union them for pairwise check.
	var allCtxEntries []topLevelEntry
	for _, e := range entries {
		if !strings.EqualFold(e.key, "context") {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(e.value), []byte("null")) {
			continue
		}
		sub, subErr := decodeTopLevelEntries(e.value)
		if subErr != nil {
			continue
		}
		allCtxEntries = append(allCtxEntries, sub...)
	}
	if len(allCtxEntries) == 0 {
		return nil
	}
	// Pairwise check across the union of all context entries.
	var offenders []string
	reported := map[string]struct{}{}
	for i, a := range allCtxEntries {
		for _, b := range allCtxEntries[i+1:] {
			exact := a.key == b.key
			fold := !exact && strings.EqualFold(a.key, b.key)
			if !exact && !fold {
				continue
			}
			// For the open context namespace: only reject fold variants when at
			// least one side aliases a modeled field (matching the read-boundary
			// rule in duplicateNestedMemberKeys).
			if fold && !isModeledContextKey(a.key) && !isModeledContextKey(b.key) {
				continue
			}
			lower := strings.ToLower(a.key)
			if _, already := reported[lower]; already {
				continue
			}
			reported[lower] = struct{}{}
			offenders = append(offenders, a.key)
		}
	}
	return offenders
}

// isReservedStatusValue reports whether raw is a JSON string equal to one of
// checkpointReservedStatusValues. A value that fails to decode as a plain
// JSON string (e.g. a number, object, or malformed literal) is treated as
// not reserved: ValidateCheckpoint's own oneof enum check downstream is
// responsible for rejecting a malformed Status shape, this check is only
// responsible for the specific reserved literal.
func isReservedStatusValue(raw json.RawMessage) bool {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return false
	}
	_, reserved := checkpointReservedStatusValues[s]
	return reserved
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
		if !isFoldKeyIn(k, checkpointProgressKeys) {
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
