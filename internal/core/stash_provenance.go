package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
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
func CorrectStashProvenance(ctx context.Context, ws *Workspace, req StashProvenanceCorrectionRequest) (*StashProvenanceCorrectionResult, error) {
	// 1–4: Validate required fields.
	if req.StashID == "" {
		return nil, blerrors.NewValidationError("stash_id", req.StashID, "must not be empty", nil)
	}
	if req.CanonicalDeliveryArtifactID == "" {
		return nil, blerrors.NewValidationError("canonical_delivery_artifact_id", req.CanonicalDeliveryArtifactID, "must not be empty", nil)
	}
	if req.Reason == "" {
		return nil, blerrors.NewValidationError("reason", req.Reason, "must not be empty", nil)
	}
	if req.Actor == "" {
		return nil, blerrors.NewValidationError("actor", req.Actor, "must not be empty", nil)
	}

	// 5: Validate path-safety (no "..", "/", "\").
	if !isProvenanceIDPathSafe(req.StashID) {
		return nil, blerrors.NewValidationError("stash_id", req.StashID, "must not contain path traversal characters", nil)
	}
	if !isProvenanceIDPathSafe(req.CanonicalDeliveryArtifactID) {
		return nil, blerrors.NewValidationError("canonical_delivery_artifact_id", req.CanonicalDeliveryArtifactID, "must not contain path traversal characters", nil)
	}

	// 6: Resolve storage root.
	storageRoot := workspaceStorageRoot(ws)

	// 7: Lock the stash archive file.
	stashArchivePath := filepath.Join(storageRoot, "archive", "stash.jsonl")
	unlock, err := lockStashFile(stashArchivePath)
	if err != nil {
		return nil, fmt.Errorf("acquire stash lock: %w", err)
	}
	defer func() { _ = unlock() }()

	// 8: Read the stash archive and find the entry.
	entry, err := readStashArchiveEntry(stashArchivePath, req.StashID)
	if err != nil {
		return nil, err
	}
	historicalArtifactID := entry.HarvestedArtifactID

	// 9: Locate the canonical delivery artifact on the filesystem.
	artifactPath, err := FindArtifactPath(ctx, ws, req.CanonicalDeliveryArtifactID)
	if err != nil {
		if errors.Is(err, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("canonical delivery artifact %s: %w", req.CanonicalDeliveryArtifactID, blerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("find canonical delivery artifact: %w", err)
	}

	// 10: Parse the artifact frontmatter and verify source_stash_id.
	rawContent, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("read artifact %s: %w", req.CanonicalDeliveryArtifactID, err)
	}
	fm, _, parseErr := models.ParseFrontmatter(string(rawContent))
	if parseErr != nil {
		return nil, fmt.Errorf("parse artifact %s frontmatter: %w", req.CanonicalDeliveryArtifactID, parseErr)
	}
	sourceStashID := extractSourceStashID(fm)
	if sourceStashID != req.StashID {
		return nil, fmt.Errorf("artifact %s source_stash_id does not match stash ID %s: %w", req.CanonicalDeliveryArtifactID, req.StashID, blerrors.ErrValidation)
	}

	// 11: Check for an existing correction for this stash ID.
	correctionsPath := filepath.Join(storageRoot, "archive", "provenance_corrections.jsonl")
	existingCanonical, err := readProvenanceCorrections(correctionsPath, req.StashID)
	if err != nil {
		return nil, fmt.Errorf("read provenance corrections: %w", err)
	}
	if existingCanonical != "" {
		if existingCanonical == req.CanonicalDeliveryArtifactID {
			// Idempotent — same correction already recorded.
			return &StashProvenanceCorrectionResult{
				Outcome:                     StashProvenanceNoOp,
				StashID:                     req.StashID,
				HistoricalArtifactID:        historicalArtifactID,
				CanonicalDeliveryArtifactID: req.CanonicalDeliveryArtifactID,
				Message:                     "stash provenance correction already recorded",
			}, nil
		}
		return nil, fmt.Errorf("conflicting provenance correction already exists for stash %s (existing canonical: %s, requested: %s): %w",
			req.StashID, existingCanonical, req.CanonicalDeliveryArtifactID, blerrors.ErrValidation)
	}

	// C1 ordering: append the durable item-log event BEFORE writing to
	// provenance_corrections.jsonl. If the event append fails, the correction
	// record is not yet persisted, so the operation is safe to retry — the retry
	// will re-check readProvenanceCorrections, find no existing correction, and
	// re-attempt both the event and the persistence. If the persistence write
	// fails after the event succeeds, the next retry will find no correction and
	// re-emit a duplicate event (benign for audit logs) before persisting.
	// This ordering prevents the irrecoverable case: correction persisted + event
	// lost, which would cause retries to short-circuit as NoOp without the event.

	// 13 (reordered): Append a durable event to the canonical artifact's item log.
	if eventErr := appendItemEventErr(ctx, ws, req.CanonicalDeliveryArtifactID, "stash_provenance_corrected", map[string]any{
		"stash_id":                       req.StashID,
		"historical_artifact_id":         historicalArtifactID,
		"canonical_delivery_artifact_id": req.CanonicalDeliveryArtifactID,
		"reason":                         req.Reason,
		"actor":                          req.Actor,
	}); eventErr != nil {
		return nil, fmt.Errorf("append stash provenance event: %w", eventErr)
	}

	// 12 (reordered): Append the correction record to provenance_corrections.jsonl.
	// C5: Use durable append (fsync file + ensure archive dir exists) so the record
	// survives crash-after-write within the OS page-cache flush window.
	correction := ProvenanceCorrection{
		StashID:                     req.StashID,
		HistoricalArtifactID:        historicalArtifactID,
		CanonicalDeliveryArtifactID: req.CanonicalDeliveryArtifactID,
		Reason:                      req.Reason,
		Actor:                       req.Actor,
		CorrectedAt:                 time.Now().UTC().Format(time.RFC3339),
		EventType:                   "stash_provenance_corrected",
	}
	if appendErr := appendToProvenanceCorrections(correctionsPath, correction); appendErr != nil {
		return nil, fmt.Errorf("append provenance correction record: %w", appendErr)
	}

	// 14: Unlock is handled by defer.

	// 15: Return the successful result.
	return &StashProvenanceCorrectionResult{
		Outcome:                     StashProvenanceCorrected,
		StashID:                     req.StashID,
		HistoricalArtifactID:        historicalArtifactID,
		CanonicalDeliveryArtifactID: req.CanonicalDeliveryArtifactID,
		Message:                     "stash provenance correction recorded",
	}, nil
}

// isProvenanceIDPathSafe reports whether id contains no path-traversal characters.
func isProvenanceIDPathSafe(id string) bool {
	return !strings.Contains(id, "..") &&
		!strings.Contains(id, "/") &&
		!strings.Contains(id, `\`)
}

// extractSourceStashID safely reads custom_fields.source_stash_id from parsed
// frontmatter, returning "" when the field is absent or not a string.
func extractSourceStashID(fm map[string]any) string {
	if fm == nil {
		return ""
	}
	cfRaw, ok := fm["custom_fields"]
	if !ok {
		return ""
	}
	cf, ok := cfRaw.(map[string]any)
	if !ok {
		return ""
	}
	v, _ := cf["source_stash_id"].(string)
	return v
}

// readStashArchiveEntry opens the stash JSONL archive at archivePath and returns
// the entry whose ID matches stashID. Returns a wrapped ErrNotFound when the
// archive is absent or the entry is not present.
func readStashArchiveEntry(archivePath, stashID string) (ArchivedStashEntry, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		if os.IsNotExist(err) {
			return ArchivedStashEntry{}, fmt.Errorf("stash entry %s not found in archive: %w", stashID, blerrors.ErrNotFound)
		}
		return ArchivedStashEntry{}, fmt.Errorf("open stash archive: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var entry ArchivedStashEntry
		if jsonErr := json.Unmarshal(raw, &entry); jsonErr != nil {
			continue
		}
		if entry.ID == stashID {
			return entry, nil
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return ArchivedStashEntry{}, fmt.Errorf("scan stash archive: %w", scanErr)
	}
	return ArchivedStashEntry{}, fmt.Errorf("stash entry %s not found in archive: %w", stashID, blerrors.ErrNotFound)
}

// appendToProvenanceCorrections durably appends a correction record to
// correctionsPath. It fsyncs the file after write to satisfy the
// "durable append-only correction record" contract (C5 remediation).
func appendToProvenanceCorrections(correctionsPath string, correction ProvenanceCorrection) error {
	correctionBytes, err := json.Marshal(correction)
	if err != nil {
		return fmt.Errorf("marshal provenance correction: %w", err)
	}
	line := append(correctionBytes, '\n')
	f, err := os.OpenFile(correctionsPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open provenance corrections file: %w", err)
	}
	if _, writeErr := f.Write(line); writeErr != nil {
		_ = f.Close()
		return fmt.Errorf("write provenance correction: %w", writeErr)
	}
	if syncErr := f.Sync(); syncErr != nil {
		_ = f.Close()
		return fmt.Errorf("fsync provenance corrections file: %w", syncErr)
	}
	if closeErr := f.Close(); closeErr != nil {
		return fmt.Errorf("close provenance corrections file: %w", closeErr)
	}
	return nil
}

// readProvenanceCorrections reads correctionsPath (provenance_corrections.jsonl)
// and returns the canonical_delivery_artifact_id recorded for stashID, or ""
// when no correction exists. A missing file is not an error.
func readProvenanceCorrections(correctionsPath, stashID string) (string, error) {
	f, err := os.Open(correctionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("open provenance corrections: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var c ProvenanceCorrection
		if jsonErr := json.Unmarshal(raw, &c); jsonErr != nil {
			// C4: Fail closed on unparseable lines. A torn/corrupt line that is
			// silently skipped would bypass conflict detection and allow a conflicting
			// canonical delivery to be appended. Return the line number with context
			// so the operator can inspect and repair the corrections file.
			return "", fmt.Errorf("parse provenance corrections line %q: %w", string(raw), jsonErr)
		}
		if c.StashID == stashID {
			return c.CanonicalDeliveryArtifactID, nil
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return "", fmt.Errorf("scan provenance corrections: %w", scanErr)
	}
	return "", nil
}
