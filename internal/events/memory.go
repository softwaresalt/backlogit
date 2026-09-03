package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// maxCheckpointStateDumpSize is the fail-closed maximum allowed size for a
// checkpoint state_dump. Checkpoints are git-tracked; an unbounded write is
// a permanent, irreversible exposure of potentially large or sensitive data
// (153.003-T / S1 U3, 6CE00B88 decision).
const maxCheckpointStateDumpSize = 65536 // 64 KiB

// checkpointSecretPrefixes is the heuristic set of known secret-material
// prefixes scanned at the checkpoint-context write boundary. Any match
// causes a fail-closed rejection before writing. The list covers the most
// common accidental-exposure patterns; a full key-allowlist is deferred
// (recorded open follow-up, YAGNI).
//
// Patterns are anchored to a JSON-string-start boundary (a literal '"'
// character) to reduce false positives on common substrings: "sk-" alone
// would match "task-", "risk-", "desk-" etc., which are pervasive in this
// task-management tool's domain vocabulary. Anchoring to `"sk-` matches
// only a JSON string VALUE or KEY that begins with sk-, not words that
// merely contain those characters.
//
// "-----BEGIN" and longer patterns that are unlikely to appear as word
// substrings remain unanchored.
var checkpointSecretPrefixes = []string{
	`"ghp_`, `"gho_`, `"ghs_`, `"ghu_`,         // GitHub OAuth / server / user tokens (anchored)
	`"github_pat_`,                               // GitHub fine-grained PAT (anchored)
	`"ghr_`,                                      // GitHub refresh token (anchored)
	`"AKIA`,  // AWS access key ID (anchored)
	`"sk-`,   // OpenAI / generic secret-key prefix (anchored — avoids "task-", "risk-")
	`"AIza`,  // Google API key (anchored)
	`"SG.`,   // SendGrid API key (anchored)
	`"xoxb-`, `"xoxp-`, `"xoxe-`, `"xoxa-`, // Slack token variants (anchored, specific)
	// "eyJ" (JWT prefix) is intentionally ABSENT from the raw scan: a 3-char JSON
	// string value starting with "eyJ" (e.g. {"note":"eyJ"}) would be a false positive.
	// JWT detection is handled exclusively by containsJWTPrefix in the decoded pass,
	// which requires a word boundary AND a minimum payload length (Copilot re-review
	// PRRT_kwDORzozKM6fBih6).
	`-----BEGIN`, // PEM-encoded private key or certificate (unanchored — distinct pattern)
}

// checkStateDumpSize returns ErrCheckpointStateDumpTooLarge when data exceeds
// maxCheckpointStateDumpSize. The error message MUST NOT include payload bytes
// (Constitution III: checkpoint context may contain sensitive data).
func checkStateDumpSize(data []byte) error {
	if len(data) > maxCheckpointStateDumpSize {
		return fmt.Errorf("%w: dump is %d bytes, limit is %d",
			backlogiterrors.ErrCheckpointStateDumpTooLarge, len(data), maxCheckpointStateDumpSize)
	}
	return nil
}

// checkStateDumpSecrets returns ErrCheckpointStateDumpSecretDetected when
// data contains a heuristically detected secret pattern. It does NOT include
// the matched pattern or payload bytes in the error message (Constitution III).
//
// Two passes: (1) raw-byte scan against JSON-anchored prefixes; (2) decoded
// JSON string scan using Contains/word-boundary logic. The decoded pass closes
// the Unicode-escape bypass (e.g. \u0067hp_ → ghp_) and catches embedded
// secrets like "Bearer ghp_abc" (adversarial-review Copilot re-review
// remediation, 153.003-T).
func checkStateDumpSecrets(data []byte) error {
	// Pass 1: raw byte scan (fast; catches most common cases).
	s := string(data)
	for _, prefix := range checkpointSecretPrefixes {
		if strings.Contains(s, prefix) {
			return fmt.Errorf("%w: detected pattern indicates possible secret material; redact before creating a checkpoint",
				backlogiterrors.ErrCheckpointStateDumpSecretDetected)
		}
	}
	// Pass 2: decoded JSON string values (closes Unicode-escape bypass).
	if err := checkDecodedJSONStringsForSecrets(data); err != nil {
		return err
	}
	return nil
}

// checkDecodedJSONStringsForSecrets scans all JSON string tokens in data in
// SOURCE ORDER using a token stream, returning ErrCheckpointStateDumpSecretDetected
// if any string token (key or value) contains a known secret pattern after
// JSON decoding (e.g. \u0067hp_secret → ghp_secret).
//
// Token-stream scanning is required — json.Unmarshal into a map collapses
// duplicate keys to the last occurrence, so a legacy dump with a
// Unicode-escaped secret in an earlier duplicate entry and a safe value in
// the last occurrence would pass the map-based check while its original
// secret-bearing bytes are written verbatim (Copilot PRRT_kwDORzozKM6fCnxo).
//
// If data is not valid JSON, this pass is silently skipped (the caller
// already ran json.Valid). Error messages contain no payload bytes (Constitution III).
func checkDecodedJSONStringsForSecrets(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil // invalid token; fail open (raw scan already ran)
		}
		s, ok := tok.(string)
		if !ok {
			continue
		}
		// Each string token (key OR value) is scanned in source order,
		// so duplicate keys are all evaluated, including earlier ones that
		// map-based decoding would discard in favor of the last-wins entry.
		if decodedStringContainsSecret(s) {
			return fmt.Errorf("%w: detected pattern indicates possible secret material; redact before creating a checkpoint",
				backlogiterrors.ErrCheckpointStateDumpSecretDetected)
		}
	}
	return nil
}



// decodedStringContainsSecret reports whether s contains a secret-material
// pattern from a decoded JSON string value. ALL patterns use word-boundary
// detection to prevent false positives on ordinary words and filenames
// (e.g. "SLOVAKIA" contains "AKIA" but is not an AWS key; "MSG.txt" contains
// "SG." but is not a SendGrid API key — Copilot PRRT_kwDORzozKM6fDJgC).
func decodedStringContainsSecret(s string) bool {
	for _, prefix := range []string{
		"ghp_", "gho_", "ghs_", "ghu_",       // GitHub OAuth / server / user tokens
		"github_pat_",                          // GitHub fine-grained PAT
		"ghr_",                                 // GitHub refresh token
		"AKIA",  // AWS access key ID (word-boundary: "SLOVAKIA" must not match)
		"AIza",  // Google API key
		"SG.",   // SendGrid (word-boundary: "MSG.txt" must not match)
		"xoxb-", "xoxp-", "xoxe-", "xoxa-",   // Slack tokens
		"-----BEGIN",                           // PEM header
	} {
		if containsAtWordBoundary(s, prefix) {
			return true
		}
	}
	if containsJWTPrefix(s) {
		return true
	}
	return containsSecretSKPrefix(s)
}

// containsAtWordBoundary reports whether s contains prefix at a position that
// is either the start of the string or preceded by a non-word character
// ([a-zA-Z0-9_]). This prevents false positives where a short prefix appears
// as an internal substring of a normal word.
func containsAtWordBoundary(s, prefix string) bool {
	for i := 0; i+len(prefix) <= len(s); i++ {
		if !strings.HasPrefix(s[i:], prefix) {
			continue
		}
		if i == 0 {
			return true // start of string
		}
		prev := s[i-1]
		if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') ||
			(prev >= '0' && prev <= '9') || prev == '_' {
			continue // preceded by word char; not a token start
		}
		return true // word boundary: space, colon, comma, etc.
	}
	return false
}
// containsSecretSKPrefix reports whether s contains "sk-" preceded by a
// word boundary (start of string or a non-[a-zA-Z0-9_] character). Hyphen
// is intentionally NOT treated as a word character here: "openai-key-sk-proj-..."
// has a hyphen before "sk-" and that IS a secret token start, while "task-001"
// has 'a' before "sk-" (word char → rejected). This matches containsAtWordBoundary's
// convention (Copilot PRRT_kwDORzozKM6fDzYk).
func containsSecretSKPrefix(s string) bool {
	const prefix = "sk-"
	for i := 0; i+len(prefix) <= len(s); i++ {
		if !strings.HasPrefix(s[i:], prefix) {
			continue
		}
		if i == 0 {
			return true // at start of string
		}
		prev := s[i-1]
		// Reject if preceded by a letter, digit, or underscore — not by a hyphen.
		if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') ||
			(prev >= '0' && prev <= '9') || prev == '_' {
			continue
		}
		return true // word boundary: space, colon, hyphen, comma, etc.
	}
	return false
}

// containsJWTPrefix reports whether s contains "eyJ" with a word boundary
// (start of string or non-word char before it) AND at least 20 characters
// after the prefix (a plausible minimum JWT token length). This prevents
// false-positives on words like "honeyJar" that contain "eyJ" but are not JWTs.
func containsJWTPrefix(s string) bool {
	const prefix = "eyJ"
	const minPayloadLen = 20
	for i := 0; i+len(prefix) <= len(s); i++ {
		if !strings.HasPrefix(s[i:], prefix) {
			continue
		}
		// Word boundary: at start of string OR preceded by a non-word char.
		if i > 0 {
			prev := s[i-1]
			if (prev >= 'a' && prev <= 'z') || (prev >= 'A' && prev <= 'Z') ||
				(prev >= '0' && prev <= '9') || prev == '_' {
				continue
			}
		}
		// Minimum length check: must have at least minPayloadLen chars after prefix.
		if len(s)-(i+len(prefix)) < minPayloadLen {
			continue
		}
		return true
	}
	return false
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
	if err := checkStateDumpSize(data); err != nil {
		return CreateCheckpointResult{}, err
	}
	// 153.003-T (S1 U3 / Copilot suppressed finding): scan for secret patterns
	// BEFORE any V1 validation that echoes caller-supplied field names in its
	// error message. Without this early placement, checkClosedSchemaNamespace
	// could return CheckpointUnknownFieldError{Fields: ["ghp_<secret>"]} before
	// the scan runs, leaking the key name in the error message.
	if err := checkStateDumpSecrets(data); err != nil {
		return CreateCheckpointResult{}, err
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
		// 153.003-T (S1 U3 / adversarial-review F4 remediation): re-check the
		// size against the final re-marshaled bytes, not the raw input. V1 auto-
		// population (created_at, updated_at, status defaults) may grow the
		// buffer past the fail-closed limit. This second check runs against the
		// exact bytes that will be written to disk.
		if sizeErr := checkStateDumpSize(data); sizeErr != nil {
			return CreateCheckpointResult{}, sizeErr
		}
	}

	if err := checkStateDumpSecrets(data); err != nil {
		return CreateCheckpointResult{}, err
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
