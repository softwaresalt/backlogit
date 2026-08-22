package events

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
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
	Disposition         string    `json:"disposition,omitempty" validate:"omitempty,oneof=abandoned quarantined"`
	DispositionReason   string    `json:"disposition_reason,omitempty"`
	DispositionOperator string    `json:"disposition_operator,omitempty"`
	DispositionAt       time.Time `json:"disposition_at,omitempty"`
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

// Keys returns the sorted set of context key names that MarshalJSON actually
// emits to disk for this CheckpointContext. It is a value receiver so it
// behaves identically for addressable and non-addressable values, matching
// MarshalJSON's receiver.
//
// Deprecated: this is a declaration-only prelude stub (146.001-T / U0a). It
// always returns an empty, non-nil slice and a nil error. The real
// implementation lands in 146.006-T (U2), which replaces only this method's
// body — the signature is pinned here and does not change.
func (c CheckpointContext) Keys() ([]string, error) {
	return make([]string, 0), nil
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
	RemediationCommand string `json:"remediation_command,omitempty"`
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
