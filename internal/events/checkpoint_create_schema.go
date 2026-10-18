package events

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/jsonutil"
)

// createSchemaClass is the create-time classification of a dump's top-level
// schema_version member.
type createSchemaClass int

const (
	// createSchemaV1 is exactly one schema_version member whose raw token is
	// the integer literal 1 (matched case-insensitively on the key).
	createSchemaV1 createSchemaClass = iota
	// createSchemaLegacy is an absent schema_version member, or exactly one
	// member whose raw token is the integer literal 0.
	createSchemaLegacy
)

// classifyCreateSchema classifies data by streaming its top-level members
// (never a struct/map decode, which would collapse duplicate keys last-wins).
// The schema_version key is matched with strings.EqualFold, mirroring
// encoding/json and checkClosedSchemaNamespace. Anything that is not a V1 or
// legacy-eligible shape (non-object, duplicate/fold-duplicate member,
// wrong-typed, non-integral, out-of-range, negative or future value) returns
// an error satisfying ErrCheckpointSchemaUnsupported.
func classifyCreateSchema(data []byte) (createSchemaClass, error) {
	entries, err := decodeTopLevelEntries(data)
	if err != nil {
		return 0, fmt.Errorf("%w: state_dump is not a JSON object", backlogiterrors.ErrCheckpointSchemaUnsupported)
	}
	var raws []json.RawMessage
	for _, e := range entries {
		if strings.EqualFold(e.key, "schema_version") {
			raws = append(raws, e.value)
		}
	}
	switch len(raws) {
	case 0:
		return createSchemaLegacy, nil
	case 1:
		switch string(bytes.TrimSpace(raws[0])) {
		case "1":
			return createSchemaV1, nil
		case "0":
			return createSchemaLegacy, nil
		}
		return 0, fmt.Errorf("%w: schema_version must be the integer 1", backlogiterrors.ErrCheckpointSchemaUnsupported)
	default:
		return 0, fmt.Errorf("%w: schema_version is duplicated", backlogiterrors.ErrCheckpointSchemaUnsupported)
	}
}

// coerceSchemaVersionToV1 rewrites data's top-level object with every
// schema_version member removed and a single "schema_version":1 member
// prepended. All other members (including duplicates) are preserved so the
// downstream V1 pipeline sees exactly what the caller supplied.
func coerceSchemaVersionToV1(data []byte) ([]byte, error) {
	entries, err := decodeTopLevelEntries(data)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString(`{"schema_version":1`)
	for _, e := range entries {
		if strings.EqualFold(e.key, "schema_version") {
			continue
		}
		k, err := json.Marshal(e.key)
		if err != nil {
			return nil, err
		}
		buf.WriteByte(',')
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(e.value)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// buildCanonicalV1 runs the complete V1 pre-write pipeline over data (which
// must carry schema_version 1): parse, closed namespace, duplicate context
// keys, create-time defaults, full validation, Context.Keys(), canonical
// marshal, post-marshal size check, and a second secret scan. It returns the
// exact bytes to write and the persisted context key names. It performs no
// I/O, so it doubles as the legacy-upgrade dry-run.
func buildCanonicalV1(data []byte) ([]byte, []string, error) {
	cp, err := ParseCheckpoint(data)
	if err != nil {
		// Preserve the ErrCheckpointCorrupt sentinel from ParseCheckpoint.
		return nil, nil, fmt.Errorf("parse v1 checkpoint: %w", err)
	}
	// Pass 2 (146.011-T / U4): closed top-level and nested-progress namespace.
	// The typed error is returned directly so errors.As recovers it in one hop.
	if err := checkClosedSchemaNamespace(data); err != nil {
		return nil, nil, err
	}
	// 148-F / U2: duplicate or case-fold-aliased context member names.
	if dupKeys := contextDuplicateCreateKeys(data); len(dupKeys) > 0 {
		return nil, nil, &backlogiterrors.CheckpointDuplicateContextKeyError{Keys: dedupeSorted(dupKeys)}
	}
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now().UTC()
	}
	if cp.UpdatedAt.IsZero() {
		cp.UpdatedAt = time.Now().UTC()
	}
	if cp.Status == "" {
		cp.Status = "active"
	}
	if err := ValidateCheckpoint(cp); err != nil {
		return nil, nil, err
	}

	// Call ordering is pinned (146.015-T / U6): Keys() runs before any write,
	// so a serialization failure prevents a file whose keys cannot be reported.
	keys, keysErr := cp.Context.Keys()
	if keysErr != nil {
		return nil, nil, fmt.Errorf("compute checkpoint context keys: %w", keysErr)
	}

	out, marshalErr := jsonutil.MarshalReadable(cp)
	if marshalErr != nil {
		return nil, nil, fmt.Errorf("marshal v1 checkpoint: %w", marshalErr)
	}
	// 153.003-T: re-check size against the exact bytes that will be written.
	if sizeErr := checkStateDumpSize(out); sizeErr != nil {
		return nil, nil, sizeErr
	}
	if err := checkStateDumpSecrets(out); err != nil {
		return nil, nil, err
	}
	return out, keys, nil
}

// nullRejectedFields are the create-time defaultable members. json.Unmarshal
// maps a present JSON null to the Go zero value, which is indistinguishable
// from an absent member and would be silently defaulted; a present-null member
// is invalid, and only an absent one is defaulted.
var nullRejectedFields = [...]string{"status", "created_at", "updated_at"}

// rejectPresentNullDefaultables scans the ordered top-level members (every
// occurrence, including duplicates) and reports an error naming the
// first defaultable member whose value is JSON null. Member names match
// case-insensitively, mirroring encoding/json decoding. Non-object input is
// left to the downstream pipeline to classify.
func rejectPresentNullDefaultables(data []byte) error {
	// Ordered member scan (not a map decode, which collapses duplicate keys
	// last-wins and would hide an earlier null).
	entries, err := decodeTopLevelEntries(data)
	if err != nil {
		return nil
	}
	for _, field := range nullRejectedFields {
		for _, e := range entries {
			if strings.EqualFold(e.key, field) && bytes.Equal(bytes.TrimSpace(e.value), []byte("null")) {
				return fmt.Errorf("checkpoint field %q is present but null; omit it to use the default", field)
			}
		}
	}
	return nil
}

// legacyDryRun upgrades a legacy-eligible dump to schema_version 1 and runs
// the full pipeline. Size and secret failures keep their native errors so the
// caller can remediate; every other failure is structural (opt-in cannot
// help) and is reported as ErrCheckpointSchemaUnsupported with the cause
// preserved via a second %w.
func legacyDryRun(data []byte) ([]byte, []string, error) {
	if err := rejectPresentNullDefaultables(data); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointSchemaUnsupported, err)
	}
	upgraded, err := coerceSchemaVersionToV1(data)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointSchemaUnsupported, err)
	}
	out, keys, err := buildCanonicalV1(upgraded)
	if err != nil {
		if errors.Is(err, backlogiterrors.ErrCheckpointStateDumpTooLarge) ||
			errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected) {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointSchemaUnsupported, err)
	}
	return out, keys, nil
}
