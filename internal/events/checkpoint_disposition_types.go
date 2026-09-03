package events

import (
	"encoding/json"
	"fmt"
	"time"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// Closed allowlist of checkpoint administrative disposition values (136-F).
// AbandonCheckpoint writes DispositionAbandoned; QuarantineCheckpoint writes
// DispositionQuarantined. Any other value fails closed via
// ValidateDispositionValue.
const (
	// DispositionAbandoned marks a checkpoint as administratively abandoned
	// (a terminal, non-resumable state distinct from the artifact-level
	// "abandoned" WIT status — see the naming boundary note in
	// docs/design-docs/checkpoint-administrative-disposition.md).
	DispositionAbandoned = "abandoned"
	// DispositionQuarantined marks a checkpoint as administratively
	// quarantined: its bytes were moved verbatim to the archive/checkpoints
	// directory without rewriting.
	DispositionQuarantined = "quarantined"
)

// checkpointDispositionSidecarSuffix is the fixed suffix appended to a
// checkpoint's basename to produce its sidecar record filename:
// "<checkpoint-filename>.disposition.json".
const checkpointDispositionSidecarSuffix = ".disposition.json"

// CheckpointDispositionRecord is the sidecar record written alongside (or, for
// quarantine, into the quarantine destination directory alongside) a
// checkpoint file to describe an administrative disposition action. It is
// written as "<checkpoint-filename>.disposition.json".
//
// 153.002-T (S1 U2): The quarantine sidecar is a CREATE-ONLY write (not an
// idempotent upsert). Re-running QuarantineCheckpoint when a sidecar already
// exists at the destination returns ErrCheckpointDestinationOccupied rather
// than silently overwriting the prior quarantine evidence. Do not assume the
// operation is safely retryable; check the returned error first.
type CheckpointDispositionRecord struct {
	// Filename is the basename of the checkpoint file the sidecar describes.
	Filename string `json:"filename"`
	// Disposition is the administrative action taken; validated against the
	// closed allowlist (DispositionAbandoned, DispositionQuarantined).
	Disposition string `json:"disposition"`
	// Reason is the caller-supplied justification for the disposition.
	Reason string `json:"reason"`
	// Operator is the resolved operator identity that performed the
	// disposition. Never defaulted to a fixed string such as "backlogit".
	Operator string `json:"operator"`
	// DispositionAt is the UTC timestamp the disposition was applied.
	DispositionAt time.Time `json:"disposition_at"`
}

// ValidateDispositionValue fails closed on any value outside the closed
// allowlist (DispositionAbandoned, DispositionQuarantined).
func ValidateDispositionValue(v string) error {
	switch v {
	case DispositionAbandoned, DispositionQuarantined:
		return nil
	default:
		return fmt.Errorf("%w: unrecognized disposition value %q", backlogiterrors.ErrCheckpointInvalid, v)
	}
}

// CheckpointDispositionSidecarPath returns the sidecar record path for a
// checkpoint file path (its directory is preserved; only the filename gains
// the ".disposition.json" suffix).
func CheckpointDispositionSidecarPath(checkpointPath string) string {
	return checkpointPath + checkpointDispositionSidecarSuffix
}

// MarshalDispositionRecord serializes a CheckpointDispositionRecord to
// indented JSON bytes suitable for an atomic sidecar write.
func MarshalDispositionRecord(rec CheckpointDispositionRecord) ([]byte, error) {
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal checkpoint disposition record: %w", err)
	}
	return data, nil
}
