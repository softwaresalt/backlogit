package core

import (
	"context"
	"errors"
)

// StashProvenanceCorrectionOutcome describes the outcome of a stash provenance correction.
type StashProvenanceCorrectionOutcome string

const (
	// StashProvenanceCorrected indicates the correction was recorded successfully.
	StashProvenanceCorrected StashProvenanceCorrectionOutcome = "corrected"
	// StashProvenanceNoOp indicates the correction already exists with the same canonical delivery.
	StashProvenanceNoOp StashProvenanceCorrectionOutcome = "no_op"
)

// ProvenanceCorrection is a single correction record written to provenance_corrections.jsonl.
type ProvenanceCorrection struct {
	StashID                     string `json:"stash_id"`
	HistoricalArtifactID        string `json:"historical_artifact_id"`
	CanonicalDeliveryArtifactID string `json:"canonical_delivery_artifact_id"`
	Reason                      string `json:"reason"`
	Actor                       string `json:"actor"`
	CorrectedAt                 string `json:"corrected_at"`
	EventType                   string `json:"event_type"`
}

// StashProvenanceCorrectionRequest is the input to CorrectStashProvenance.
type StashProvenanceCorrectionRequest struct {
	// StashID is the stash entry ID to correct. Required.
	StashID string
	// CanonicalDeliveryArtifactID is the artifact ID of the actual delivery. Required.
	CanonicalDeliveryArtifactID string
	// Reason is a human-readable explanation. Required.
	Reason string
	// Actor is the operator or agent performing the correction. Required.
	Actor string
}

// StashProvenanceCorrectionResult is the outcome of CorrectStashProvenance.
type StashProvenanceCorrectionResult struct {
	// Outcome is corrected or no_op.
	Outcome StashProvenanceCorrectionOutcome
	// StashID is the stash entry ID.
	StashID string
	// HistoricalArtifactID is the harvested_artifact_id from the stash archive.
	HistoricalArtifactID string
	// CanonicalDeliveryArtifactID is the confirmed actual delivery artifact ID.
	CanonicalDeliveryArtifactID string
	// Message is a human-readable summary.
	Message string
}

// CorrectStashProvenance records a provenance correction for a stash entry, noting
// that the canonical actual delivery artifact differs from the historically
// auto-harvested artifact. It preserves the original harvested_artifact_id and
// appends an append-only correction record to provenance_corrections.jsonl.
// Conflicting corrections (same stash ID, different canonical delivery) are rejected.
func CorrectStashProvenance(_ context.Context, _ *Workspace, _ StashProvenanceCorrectionRequest) (*StashProvenanceCorrectionResult, error) {
	return nil, errors.New("backlogit: CorrectStashProvenance not implemented")
}