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
	Status        string              `json:"status" validate:"required,oneof=active resolved"`
	CreatedAt     time.Time           `json:"created_at" validate:"required"`
	UpdatedAt     time.Time           `json:"updated_at" validate:"required"`
	Context       CheckpointContext   `json:"context"`
	Progress      *CheckpointProgress `json:"progress,omitempty"`
	ResumeHint    string              `json:"resume_hint,omitempty"`
}

// CheckpointContext holds shipment/feature/branch context for the checkpoint.
type CheckpointContext struct {
	ShipmentID string   `json:"shipment_id,omitempty"`
	FeatureID  string   `json:"feature_id,omitempty"`
	TaskIDs    []string `json:"task_ids,omitempty"`
	Branch     string   `json:"branch,omitempty"`
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
	Quarantined bool `json:"quarantined,omitempty"`
}

// CleanupResult reports the outcome of a checkpoint cleanup operation.
type CleanupResult struct {
	ArchivedCount int      `json:"archived_count"`
	ArchivedFiles []string `json:"archived_files"`
	SkippedCount  int      `json:"skipped_count"`
	Errors        []string `json:"errors,omitempty"`
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
