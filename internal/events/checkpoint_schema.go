package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/jsonutil"
)

// checkpointValidator is the package-level validator instance (per compound learning: cache at package level).
var checkpointValidator = validator.New()

// CheckpointV1 is the canonical schema for agent session checkpoints.
type CheckpointV1 struct {
	SchemaVersion int                 `json:"schema_version" validate:"eq=1"`
	Agent         string              `json:"agent" validate:"required,oneof=ship stage"`
	SessionID     string              `json:"session_id" validate:"required"`
	Phase         string              `json:"phase" validate:"required"`
	Status        string              `json:"status" validate:"required,oneof=active resolved abandoned"`
	CreatedAt     time.Time           `json:"created_at" validate:"required"`
	UpdatedAt     time.Time           `json:"updated_at" validate:"required"`
	Context       CheckpointContext   `json:"context"`
	Progress      *CheckpointProgress `json:"progress,omitempty"`
	ResumeHint    string              `json:"resume_hint,omitempty"`

	// Disposition, DispositionReason, DispositionOperator, and DispositionAt
	// (136-F) record an administrative disposition action taken against this
	// checkpoint via AbandonCheckpoint. Disposition validates against a closed
	// allowlist (see DispositionAbandoned, DispositionQuarantined) and fails
	// closed on any unrecognized value. These fields are populated only when
	// an abandon disposition has been applied; QuarantineCheckpoint never
	// rewrites the checkpoint bytes and instead records its disposition in a
	// sidecar (see CheckpointDispositionRecord).
	//
	// DispositionAt is *time.Time, not time.Time (Copilot review remediation
	// on PR #373): encoding/json's `omitempty` does NOT omit a zero-value
	// struct (time.Time is a struct), so a value-typed field with
	// `,omitempty` would still marshal a never-abandoned checkpoint's
	// DispositionAt as the literal zero time "0001-01-01T00:00:00Z" on every
	// ordinary create. Once 146.011-T made "disposition_at" a reserved,
	// create-rejected field, that spurious zero-time member would make an
	// otherwise-normal checkpoint's own written JSON fail if ever resubmitted
	// through CreateCheckpoint. A nil *time.Time correctly omits under
	// `,omitempty`.
	Disposition         string     `json:"disposition,omitempty" validate:"omitempty,oneof=abandoned quarantined"`
	DispositionReason   string     `json:"disposition_reason,omitempty"`
	DispositionOperator string     `json:"disposition_operator,omitempty"`
	DispositionAt       *time.Time `json:"disposition_at,omitempty"`
}

// CheckpointContext holds shipment/feature/branch context for the checkpoint.
// The context namespace is intentionally OPEN: any key not modeled by one of
// the four fields below is preserved in Extra rather than dropped, so a
// caller's unmodeled context keys survive the create round-trip on disk. This
// is distinct from the CheckpointV1 top level and the nested progress object,
// both of which enforce a CLOSED schema namespace at the create boundary.
type CheckpointContext struct {
	ShipmentID string   `json:"shipment_id,omitempty"`
	FeatureID  string   `json:"feature_id,omitempty"`
	TaskIDs    []string `json:"task_ids,omitempty"`
	Branch     string   `json:"branch,omitempty"`

	// Extra carries every context key that does not map to one of the four
	// modeled fields above. The json:"-" tag is load-bearing: it keeps Extra
	// invisible to encoding/json so a defined-type conversion to a
	// method-less shadow (used by the custom UnmarshalJSON/MarshalJSON pair)
	// cannot leak a literal "Extra" member into the emitted context object.
	Extra map[string]json.RawMessage `json:"-"`
}

// plainContext is a method-less shadow of CheckpointContext's modeled fields
// (146.006-T / U2). It carries no Extra field and no UnmarshalJSON/MarshalJSON
// methods, so json.Unmarshal against it exercises encoding/json's own
// deterministic, case-insensitive, source-order, last-wins field matching —
// exactly once, against the original bytes — and json.Marshal against it
// (via jsonutil.MarshalReadable, never json.Marshal, so HTML escaping stays
// disabled) renders only the four modeled fields in declaration order.
type plainContext struct {
	ShipmentID string   `json:"shipment_id,omitempty"`
	FeatureID  string   `json:"feature_id,omitempty"`
	TaskIDs    []string `json:"task_ids,omitempty"`
	Branch     string   `json:"branch,omitempty"`
}

// modeledContextKeys is the set of CheckpointContext's modeled JSON tag
// names (exact declared spelling), derived once via a package-level var
// initializer by reflecting over plainContext's json tags. No init() and no
// panic: an unparsable tag is simply skipped, since plainContext's tags are
// pinned by this same file. Both UnmarshalJSON (routing keys into Extra) and
// emit() (filtering Extra keys that collide with a modeled field) consult
// this set, through isFoldKeyIn, so a future modeled field cannot silently
// desynchronize the two paths.
var modeledContextKeys = deriveModeledContextKeys()

func deriveModeledContextKeys() map[string]struct{} {
	return modeledJSONTagKeys(reflect.TypeOf(plainContext{}))
}

// modeledJSONTagKeys derives the set of a struct type's modeled JSON tag
// names (exact declared spelling) via reflection. Shared by the context
// Extra carrier (146.006-T / U2) and the create-boundary closed-namespace
// check (146.011-T / U4) so both derivations stay pinned to the same rule:
// skip unexported fields, skip an absent or "-" tag, and strip any
// ",omitempty" (or other) tag option suffix. Keys are intentionally NOT
// lowercased here: membership checks against this set must go through
// isFoldKeyIn, which compares using Unicode simple case folding (the same
// algorithm encoding/json itself uses for field matching), not
// strings.ToLower -- see isFoldKeyIn's doc comment for why the two are not
// equivalent.
func modeledJSONTagKeys(typ reflect.Type) map[string]struct{} {
	set := make(map[string]struct{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			// unexported field
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" {
			continue
		}
		set[name] = struct{}{}
	}
	return set
}

// isFoldKeyIn reports whether key matches any member of known using Unicode
// simple case folding, via strings.EqualFold. This is the single shared
// matcher for every modeled-vs-unmodeled JSON key classification in this
// package: CheckpointContext's Extra routing and collision filtering
// (isModeledContextKey below) and checkpoint_strict.go's closed top-level
// and nested-progress namespace checks.
//
// strings.EqualFold, not strings.ToLower, is load-bearing (Copilot review
// remediation on PR #373, deferred past the review-fix circuit breaker as
// stash 6D03554D and now fixed under an operator-authorized exceptional
// cycle): encoding/json's own field-name matching uses Unicode simple case
// folding, the exact equivalence relation strings.EqualFold implements --
// NOT a per-rune lowercase mapping. The two disagree for runes such as
// U+017F LATIN SMALL LETTER LONG S ("ſ"), which simple-case-folds to "s"/"S"
// (so encoding/json treats "ſhipment_id" as a match for the "shipment_id"
// tag) but is left unchanged by unicode.ToLower (it is already its own
// lowercase form, so ToLower-based comparison never recognizes the fold).
// Using ToLower here let a fold-duplicate key survive into Extra as if it
// were genuinely unmodeled, get re-emitted after the modeled field, and flip
// which occurrence won on the next reparse. Matching this package's key
// classification to encoding/json's own algorithm closes that gap by
// construction: a fold-duplicate key is always recognized as modeled, so it
// is never routed into Extra in the first place. This intentionally performs
// a linear scan rather than a normalized-map lookup: the known-key sets here
// are small (at most a handful of struct fields), and a normalization step
// (e.g. lowercasing) would reintroduce exactly the same non-equivalence bug
// this function exists to close.
func isFoldKeyIn(key string, known map[string]struct{}) bool {
	for k := range known {
		if strings.EqualFold(key, k) {
			return true
		}
	}
	return false
}

// isModeledContextKey reports whether key matches one of CheckpointContext's
// modeled JSON tag names under Unicode simple case folding (see isFoldKeyIn).
func isModeledContextKey(key string) bool {
	return isFoldKeyIn(key, modeledContextKeys)
}

// UnmarshalJSON decodes b into c. The decode mechanism is pinned (146.006-T /
// U2, PR #372 remediation): it performs exactly two decodes of the SAME
// original bytes. Pass 1 decodes b into the method-less plainContext shadow,
// which is the ONLY thing that may set a modeled field and which inherits
// encoding/json's deterministic source-order, case-insensitive, last-wins
// semantics. Pass 2 decodes b into a map[string]json.RawMessage used ONLY to
// populate Extra, keeping every key that does NOT case-insensitively match a
// modeled tag. Routing modeled keys OUT of that raw map is forbidden: a
// context carrying both shipment_id and Shipment_ID lands as two distinct
// map entries, and selecting a winner by map iteration would make the same
// bytes parse to A on one run and B on the next — a nondeterministic
// violation. Extra is order-immune because it is an accumulate-all set
// difference, never a single-winner selection. A JSON null value is a no-op
// (the zero CheckpointContext), matching encoding/json's own convention for
// decoding null into a non-pointer struct field.
func (c *CheckpointContext) UnmarshalJSON(b []byte) error {
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		*c = CheckpointContext{}
		return nil
	}

	var shadow plainContext
	if err := json.Unmarshal(b, &shadow); err != nil {
		return err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	var extra map[string]json.RawMessage
	if len(raw) > 0 {
		extra = make(map[string]json.RawMessage, len(raw))
		for k, v := range raw {
			if isModeledContextKey(k) {
				continue
			}
			extra[k] = v
		}
		if len(extra) == 0 {
			extra = nil
		}
	}

	*c = CheckpointContext{
		ShipmentID: shadow.ShipmentID,
		FeatureID:  shadow.FeatureID,
		TaskIDs:    shadow.TaskIDs,
		Branch:     shadow.Branch,
		Extra:      extra,
	}
	return nil
}

// MarshalJSON renders c's modeled fields followed by its sorted Extra keys,
// flattened into a single JSON object. It is a VALUE receiver, because
// Context is a value field on CheckpointV1 and a pointer receiver would be
// silently skipped by encoding/json whenever the value is non-addressable
// (a bare function return value, or a value stored in a map[string]any).
func (c CheckpointContext) MarshalJSON() ([]byte, error) {
	_, body, err := c.emit()
	return body, err
}

// Keys returns the context key names that MarshalJSON actually emits to disk
// for this CheckpointContext: the modeled fields JSON actually renders
// (respecting omitempty elision), in declaration order, followed by the
// sorted Extra keys that survive the modeled-key collision filter. It is a
// value receiver so it behaves identically for addressable and
// non-addressable values, matching MarshalJSON's receiver.
func (c CheckpointContext) Keys() ([]string, error) {
	keys, _, err := c.emit()
	if keys == nil {
		keys = make([]string, 0)
	}
	return keys, err
}

// emit is the single unexported implementation both MarshalJSON and Keys()
// delegate to, so neither can silently swallow an error the other surfaces.
// It marshals the plainContext shadow through jsonutil.MarshalReadable (never
// json.Marshal, so HTML escaping stays disabled), reads back the keys it
// actually emitted (respecting omitempty elision) via a token-stream walk —
// never a map decode, which would not preserve declaration order — and
// splices in each surviving Extra member: the key re-encoded through the
// same escape-free encoder (a bare json.Marshal(key) would still emit
// \u0026 for a key containing "&", and splicing raw key text into a buffer
// would let a key containing a quote or newline inject a sibling member),
// and the value appended verbatim from its stored json.RawMessage.
func (c CheckpointContext) emit() ([]string, []byte, error) {
	shadow := plainContext{
		ShipmentID: c.ShipmentID,
		FeatureID:  c.FeatureID,
		TaskIDs:    c.TaskIDs,
		Branch:     c.Branch,
	}
	modeledBytes, err := jsonutil.MarshalReadable(shadow)
	if err != nil {
		return nil, nil, fmt.Errorf("checkpoint context: marshal modeled fields: %w", err)
	}
	modeledKeysPresent, err := objectMemberKeys(modeledBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("checkpoint context: read modeled keys: %w", err)
	}

	extraKeys := make([]string, 0, len(c.Extra))
	for k := range c.Extra {
		if isModeledContextKey(k) {
			continue
		}
		extraKeys = append(extraKeys, k)
	}
	sort.Strings(extraKeys)

	keys := make([]string, 0, len(modeledKeysPresent)+len(extraKeys))
	keys = append(keys, modeledKeysPresent...)
	keys = append(keys, extraKeys...)

	var buf bytes.Buffer
	// modeledBytes is a well-formed JSON object; strip its trailing '}' so
	// Extra members can be appended before the single closing brace.
	buf.Write(modeledBytes[:len(modeledBytes)-1])
	needComma := len(modeledKeysPresent) > 0
	for _, k := range extraKeys {
		if needComma {
			buf.WriteByte(',')
		}
		needComma = true
		keyBytes, err := jsonutil.MarshalReadable(k)
		if err != nil {
			return nil, nil, fmt.Errorf("checkpoint context: marshal extra key %q: %w", k, err)
		}
		buf.Write(keyBytes)
		buf.WriteByte(':')
		buf.Write(c.Extra[k])
	}
	buf.WriteByte('}')
	return keys, buf.Bytes(), nil
}

// objectMemberKeys returns the top-level key names of a JSON object, in the
// order they appear, using a token stream rather than a map decode (which
// does not preserve order).
func objectMemberKeys(raw []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim.String() != "{" {
		return nil, fmt.Errorf("checkpoint context: expected a JSON object, got %v", tok)
	}
	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("checkpoint context: expected a string key, got %v", keyTok)
		}
		keys = append(keys, key)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// CheckpointProgress tracks task completion state within a checkpoint.
type CheckpointProgress struct {
	TasksCompleted []string `json:"tasks_completed,omitempty"`
	TasksRemaining []string `json:"tasks_remaining,omitempty"`
	FilesModified  []string `json:"files_modified,omitempty"`
	Decisions      []string `json:"decisions,omitempty"`
}

// CheckpointFilter constrains which checkpoints are returned by ListCheckpoints.
type CheckpointFilter struct {
	Agent      string        `json:"agent,omitempty"`
	Status     string        `json:"status,omitempty"`
	ShipmentID string        `json:"shipment_id,omitempty"`
	FeatureID  string        `json:"feature_id,omitempty"`
	MaxAge     time.Duration `json:"max_age,omitempty"`
}

// CheckpointSummary is a lightweight view of a checkpoint for list results.
type CheckpointSummary struct {
	Filename      string    `json:"filename"`
	Agent         string    `json:"agent"`
	SessionID     string    `json:"session_id"`
	Phase         string    `json:"phase"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	ShipmentID    string    `json:"shipment_id,omitempty"`
	FeatureID     string    `json:"feature_id,omitempty"`
	ResumeHint    string    `json:"resume_hint,omitempty"`
	ValidationErr string    `json:"validation_error,omitempty"`
	// Quarantined is true when the file was physically moved to the quarantine
	// directory due to a parse failure. ValidationErr may also be set for
	// schema validation failures that do NOT quarantine the file.
	//
	// Deprecated: ListCheckpoints (136-F/U9) no longer performs the physical
	// quarantine move as a side effect of listing. This field is retained for
	// backward JSON-shape compatibility and stays false on the read-only list
	// path; use NeedsQuarantine and RemediationCommand instead to detect and
	// act on malformed checkpoints.
	Quarantined bool `json:"quarantined,omitempty"`
	// NeedsQuarantine is true when the checkpoint file failed to parse and is a
	// quarantine candidate. ListCheckpoints never moves the file itself
	// (136-F/U9); callers must run the remediation command to quarantine it.
	NeedsQuarantine bool `json:"needs_quarantine,omitempty"`
	// RemediationCommand is the exact CLI invocation an operator or agent can
	// run to quarantine this checkpoint when NeedsQuarantine is true.
	//
	// Deprecated: use RemediationIntent; RemediationCommand is an unbound
	// command string and will be removed.
	RemediationCommand string `json:"remediation_command,omitempty"`
	// RemediationIntent describes, as structured non-executable data, what an
	// operator must do to dispose of a checkpoint that cannot be safely
	// rewritten. It is populated on every quarantine-candidate branch
	// (parse-failure, schema-invalid, and non-conforming) and is nil
	// otherwise. It is not omitempty: a nil intent marshals as
	// "remediation_intent": null so the key is always present for callers to
	// check (147-F / U1d).
	RemediationIntent *RemediationIntent `json:"remediation_intent"`
}

// RemediationIntent describes what an operator must do to dispose of a
// checkpoint that cannot be safely rewritten. It carries no shell text and is
// not runnable: TargetFilename is a bare, already-validated filename, never a
// path, never shell-quoted, and never concatenated with a directory. Only the
// CLI boundary (147-F / U16) is permitted to render an executable command
// from this structured record, bound to the resolved workspace and the A4c
// approval / preimage / no-clobber contract.
type RemediationIntent struct {
	// Verb is the disposition verb the operator must invoke, e.g. "quarantine".
	Verb string `json:"verb"`
	// TargetFilename is the bare checkpoint filename, never a path.
	TargetFilename string `json:"target_filename"`
	// RequiresApproval is always true: every remediation action is gated by
	// Constitution Principle VII / A4c operator approval.
	RequiresApproval bool `json:"requires_approval"`
	// ApprovalClass names the approval class governing this remediation,
	// e.g. "A4c".
	ApprovalClass string `json:"approval_class"`
	// Reason names why remediation is required: "schema_invalid",
	// "non_conforming", or "unparseable".
	Reason string `json:"reason"`
}

// CleanupResult reports the outcome of a checkpoint cleanup operation.
type CleanupResult struct {
	ArchivedCount int      `json:"archived_count"`
	ArchivedFiles []string `json:"archived_files"`
	SkippedCount  int      `json:"skipped_count"`
	Errors        []string `json:"errors,omitempty"`
}

// CreateCheckpointResult reports the outcome of CreateCheckpoint: the written
// file path and the context key names persisted to disk. Neither field
// carries omitempty and ContextKeys is always initialized as a non-nil,
// possibly-empty slice, so a caller reading context_keys can never observe an
// absent key or a JSON null in place of an array.
type CreateCheckpointResult struct {
	Path        string   `json:"path"`
	ContextKeys []string `json:"context_keys"`
}

// ParseCheckpoint decodes JSON bytes into a CheckpointV1 struct.
func ParseCheckpoint(data []byte) (*CheckpointV1, error) {
	var cp CheckpointV1
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("%w: %v", backlogiterrors.ErrCheckpointCorrupt, err)
	}
	return &cp, nil
}

// ValidateCheckpoint validates a CheckpointV1 struct against its validator tags.
func ValidateCheckpoint(cp *CheckpointV1) error {
	if err := checkpointValidator.Struct(cp); err != nil {
		return fmt.Errorf("%w: %v", backlogiterrors.ErrCheckpointInvalid, err)
	}
	return nil
}
