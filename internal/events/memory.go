package events

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/jsonutil"
)

// memoriesMu serializes concurrent SaveMemory calls on the same process.
var memoriesMu sync.Mutex

// SaveMemory persists a key-value pair to memories.json via atomic read-modify-write.
// A process-level mutex prevents lost updates from concurrent callers.
func SaveMemory(_ context.Context, memoriesPath string, key string, summary string) error {
	memoriesMu.Lock()
	defer memoriesMu.Unlock()

	memories := make(map[string]string)
	if data, err := os.ReadFile(memoriesPath); err == nil {
		_ = json.Unmarshal(data, &memories)
	}
	memories[key] = summary
	data, err := json.MarshalIndent(memories, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memories: %w", err)
	}
	tmp := memoriesPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write memories tmp: %w", err)
	}
	return os.Rename(tmp, memoriesPath)
}

// CreateCheckpoint writes a timestamped state dump to the checkpoints directory.
// If the state dump contains a V1 schema (schema_version=1), it is parsed and
// validated before writing. Missing created_at, updated_at, and status fields
// are auto-populated. Legacy (non-V1) dumps are written as-is with atomic writes.
//
// CreateCheckpointResult.ContextKeys (146.015-T / U6) reports the exact
// context key names persisted to disk. On the V1 path it comes from
// CheckpointContext.Keys(), whose error is PROPAGATED, never discarded; per
// the pinned call ordering, Keys() runs BEFORE the write, so a serialization
// failure deterministically prevents a file from landing on disk whose keys
// could not be reported. On the legacy (non-V1) path it comes from a
// strictly best-effort scan of the WRITTEN bytes' top-level context object,
// sorted with sort.Strings: any decode failure or non-object context yields
// an empty, non-nil array, and the create still succeeds regardless.
func CreateCheckpoint(_ context.Context, checkpointDir string, stateDump string) (CreateCheckpointResult, error) {
	if err := os.MkdirAll(checkpointDir, 0o755); err != nil {
		return CreateCheckpointResult{}, fmt.Errorf("create checkpoint dir: %w", err)
	}
	name := fmt.Sprintf("checkpoint-%s.json", time.Now().UTC().Format("20060102-150405"))
	path := filepath.Join(checkpointDir, name)

	data := []byte(stateDump)
	var contextKeys []string

	// 148-F / U1: Validate JSON syntactic correctness FIRST, before classification.
	// A truncated V1-shaped payload would otherwise fall through to the legacy
	// path and be written as corrupt bytes. No raw payload excerpt in error
	// (Constitution III: checkpoint context may contain sensitive data).
	if !json.Valid(data) {
		return CreateCheckpointResult{}, &backlogiterrors.CheckpointMalformedInputError{}
	}

	// Probe for V1 schema.
	var probe struct {
		SchemaVersion int `json:"schema_version"`
	}
	if json.Unmarshal(data, &probe) == nil && probe.SchemaVersion == 1 {
		cp, err := ParseCheckpoint(data)
		if err != nil {
			// Preserve the ErrCheckpointCorrupt sentinel from ParseCheckpoint.
			return CreateCheckpointResult{}, fmt.Errorf("parse v1 checkpoint: %w", err)
		}
		// Pass 2 (146.011-T / U4): only after pass 1 (ParseCheckpoint) has
		// already succeeded, enforce the closed CheckpointV1 top-level and
		// nested-progress schema namespace. The typed error is returned
		// directly, not wrapped again, so errors.As still recovers it in one
		// hop.
		if err := checkClosedSchemaNamespace(data); err != nil {
			return CreateCheckpointResult{}, err
		}
		// 148-F / U2: check for duplicate or case-fold-aliased context member
		// names at the create boundary. Returns typed
		// *CheckpointDuplicateContextKeyError so callers can recover the offending
		// key names via errors.As. Ordered after checkClosedSchemaNamespace so
		// the top-level and progress namespace is already clean.
		if dupKeys := contextDuplicateCreateKeys(data); len(dupKeys) > 0 {
			return CreateCheckpointResult{}, &backlogiterrors.CheckpointDuplicateContextKeyError{Keys: dedupeSorted(dupKeys)}
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
			return CreateCheckpointResult{}, err
		}

		// Call ordering is pinned (146.015-T / U6): Keys() MUST run before the
		// write below, so a serialization failure prevents the write instead
		// of leaving a file on disk whose keys cannot be reported.
		keys, keysErr := cp.Context.Keys()
		if keysErr != nil {
			return CreateCheckpointResult{}, fmt.Errorf("compute checkpoint context keys: %w", keysErr)
		}
		contextKeys = keys

		var marshalErr error
		data, marshalErr = jsonutil.MarshalReadable(cp)
		if marshalErr != nil {
			return CreateCheckpointResult{}, fmt.Errorf("marshal v1 checkpoint: %w", marshalErr)
		}
	}

	// 148-F / U3: route through the seam so tests can simulate ErrWriteIndeterminate
	// and ErrWriteNotApplied outcomes and callers can classify the error class.
	if err := syncWriteFileAtomicHook(path, data, 0o644); err != nil {
		return CreateCheckpointResult{}, fmt.Errorf("write checkpoint: %w", err)
	}

	if contextKeys == nil {
		// Legacy (non-V1) path: CheckpointContext.Keys() never ran above, so
		// scan the bytes that were actually written.
		contextKeys = legacyContextKeys(data)
	}
	return CreateCheckpointResult{Path: path, ContextKeys: contextKeys}, nil
}

// legacyContextKeys performs a strictly best-effort scan of a written
// checkpoint's top-level context object for a legacy (non-V1) dump. It never
// fails: an unparseable document, an absent context key, or a context value
// that is not a JSON object (null, a scalar, an array) all yield an empty,
// non-nil slice. When keys are found they are sorted with sort.Strings.
func legacyContextKeys(data []byte) []string {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return make([]string, 0)
	}
	ctxRaw, ok := doc["context"]
	if !ok {
		return make([]string, 0)
	}
	var ctx map[string]json.RawMessage
	if err := json.Unmarshal(ctxRaw, &ctx); err != nil {
		return make([]string, 0)
	}
	keys := make([]string, 0, len(ctx))
	for k := range ctx {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
