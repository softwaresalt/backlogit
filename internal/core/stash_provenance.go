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

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
	"github.com/softwaresalt/backlogit/internal/stash"
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
	Forced                      bool   `json:"forced,omitempty"`
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
	// Force repoints provenance onto a canonical delivery artifact that does not
	// yet carry source_stash_id, stamping the full source_stash_* snapshot from the
	// archived stash entry and superseding any prior conflicting correction.
	Force bool
}

// StashProvenanceCorrectionResult is the outcome of CorrectStashProvenance.
type StashProvenanceCorrectionResult struct {
	// Outcome is corrected or no_op.
	Outcome StashProvenanceCorrectionOutcome `json:"outcome"`
	// StashID is the stash entry ID.
	StashID string `json:"stash_id"`
	// HistoricalArtifactID is the harvested_artifact_id from the stash archive.
	HistoricalArtifactID string `json:"historical_artifact_id"`
	// CanonicalDeliveryArtifactID is the confirmed actual delivery artifact ID.
	CanonicalDeliveryArtifactID string `json:"canonical_delivery_artifact_id"`
	// Forced reports whether the correction was applied with Force.
	Forced bool `json:"forced,omitempty"`
	// Message is a human-readable summary.
	Message string `json:"message"`
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

	// 7a: Containment check — resolve the archive directory through any symlinks
	// before creating the lock file, so a planted symlink cannot redirect the lock
	// (and subsequent archive writes) outside the workspace (thread iO).
	archiveDir := filepath.Join(storageRoot, "archive")
	realArchiveDir, evalErr := filepath.EvalSymlinks(archiveDir)
	if evalErr != nil {
		if os.IsNotExist(evalErr) {
			// Archive directory doesn't exist yet — check the storage root is real.
			realStorageRoot, storageRootErr := filepath.EvalSymlinks(storageRoot)
			if storageRootErr != nil {
				return nil, fmt.Errorf("resolve workspace storage root: %w", storageRootErr)
			}
			realArchiveDir = filepath.Join(realStorageRoot, "archive")
		} else {
			return nil, fmt.Errorf("resolve archive directory: %w", evalErr)
		}
	} else {
		// Verify the resolved archive dir is still under the storage root.
		realStorageRoot, storageRootErr := filepath.EvalSymlinks(storageRoot)
		if storageRootErr != nil {
			return nil, fmt.Errorf("resolve workspace storage root: %w", storageRootErr)
		}
		if !strings.HasPrefix(realArchiveDir, realStorageRoot) {
			return nil, fmt.Errorf("archive directory %q resolves outside workspace storage root %q: %w",
				archiveDir, storageRoot, blerrors.ErrValidation)
		}
	}

	// Canonical lock order: the active stash lock is outermost (matching
	// HarvestStashEntry, which holds it while CreateArtifact takes the other
	// locks), then lifecycle global, then C (item log), then B (artifact
	// mutation). C is taken before B so the stash_provenance_corrected event
	// stays causally ordered with the mutation it records and no B-then-C edge
	// exists (artifact-mutation-lock-order.md).
	// The canonical lock is the active stash's, shared with HarvestStashEntry and
	// ArchiveStashEntry (which append to this archive while holding it).
	unlock, err := lockActiveStash(ws)
	if err != nil {
		return nil, fmt.Errorf("acquire stash lock: %w", err)
	}
	defer func() { _ = unlock() }()

	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("lock shipment lifecycle for stash provenance correction: %w", err)
	}
	defer func() { _ = globalUnlock() }()
	ctx = lockedCtx

	itemLogCtx, unlockItemLog, err := events.LockItemLogCrossProcess(ctx,
		WorkspaceLocksRoot(ws.RootPath), WorkspaceLogsRoot(ws.RootPath), req.CanonicalDeliveryArtifactID)
	if err != nil {
		return nil, fmt.Errorf("lock item log %s: %w", req.CanonicalDeliveryArtifactID, err)
	}
	defer unlockItemLog()
	ctx = itemLogCtx

	artifactCtx, unlockArtifacts, err := lockArtifactMutations(ctx, ws, []string{req.CanonicalDeliveryArtifactID})
	if err != nil {
		return nil, fmt.Errorf("lock artifact %s: %w", req.CanonicalDeliveryArtifactID, err)
	}
	defer func() { _ = unlockArtifacts() }()
	ctx = artifactCtx

	// 7: Locate the stash archive (stash lock already held, see above).
	stashArchivePath := filepath.Join(realArchiveDir, "stash.jsonl")

	// ---- Read-only preflight (no writes below this line until the audit step) ----

	// 8: Read the stash archive and find the entry.
	entry, err := readStashArchiveEntry(stashArchivePath, req.StashID)
	if err != nil {
		return nil, err
	}
	historicalArtifactID := entry.HarvestedArtifactID
	// Reject corrections for stash entries that have no harvested_artifact_id.
	// A manually archived stash entry with no harvest link cannot establish the
	// historical artifact baseline that this correction operation is designed to
	// override. A forced operator repoint may target an unharvested entry.
	if historicalArtifactID == "" && !req.Force {
		return nil, fmt.Errorf("stash entry %s has no harvested_artifact_id; provenance correction requires a prior harvest: %w",
			req.StashID, blerrors.ErrValidation)
	}

	// 9: Locate the canonical delivery artifact on the filesystem.
	artifactPath, err := FindArtifactPath(ctx, ws, req.CanonicalDeliveryArtifactID)
	if err != nil {
		if errors.Is(err, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("canonical delivery artifact %s: %w", req.CanonicalDeliveryArtifactID, blerrors.ErrNotFound)
		}
		return nil, fmt.Errorf("find canonical delivery artifact: %w", err)
	}
	if err := verifyProvenanceTargetContained(storageRoot, artifactPath); err != nil {
		return nil, err
	}

	// 10: Parse the artifact and verify source_stash_id (non-forced only).
	target, _, parseErr := parseFile(artifactPath)
	if parseErr != nil {
		return nil, fmt.Errorf("parse artifact %s frontmatter: %w", req.CanonicalDeliveryArtifactID, parseErr)
	}
	sourceStashID, _ := target.CustomFields["source_stash_id"].(string)
	if !req.Force && sourceStashID != req.StashID {
		return nil, fmt.Errorf("artifact %s source_stash_id does not match stash ID %s: %w", req.CanonicalDeliveryArtifactID, req.StashID, blerrors.ErrValidation)
	}
	if req.Force {
		if ws.DB == nil {
			return nil, fmt.Errorf("forced stash provenance correction requires the SQLite index: %w", blerrors.ErrValidation)
		}
		if sourceStashID != "" && sourceStashID != req.StashID {
			var otherCarriers int
			if qErr := ws.DB.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM items WHERE json_extract(custom_fields, '$.source_stash_id') = ? AND id != ?`,
				sourceStashID, req.CanonicalDeliveryArtifactID).Scan(&otherCarriers); qErr != nil {
				return nil, fmt.Errorf("check provenance carriers for stash %s: %w", sourceStashID, qErr)
			}
			if otherCarriers == 0 {
				return nil, fmt.Errorf("artifact %s is the sole provenance carrier for stash %s; refusing to overwrite with stash %s: %w",
					req.CanonicalDeliveryArtifactID, sourceStashID, req.StashID, blerrors.ErrValidation)
			}
		}
	}

	// 11: Check for an existing correction for this stash ID (latest wins).
	correctionsPath := filepath.Join(realArchiveDir, "provenance_corrections.jsonl")
	existing, found, err := readLatestProvenanceCorrection(correctionsPath, req.StashID)
	if err != nil {
		return nil, fmt.Errorf("read provenance corrections: %w", err)
	}
	resumeForced := false
	if found && existing.CanonicalDeliveryArtifactID != "" {
		if existing.CanonicalDeliveryArtifactID == req.CanonicalDeliveryArtifactID && (!req.Force || existing.Forced) {
			if existing.Forced && ws.DB != nil && !forcedSnapshotApplied(ctx, ws, target, entry, req) {
				// The ledger entry exists but the forced mutation never completed
				// (crash between ledger append and stamp): resume it below.
				resumeForced = true
			} else {
				// Idempotent — same correction already recorded.
				return &StashProvenanceCorrectionResult{
					Outcome:                     StashProvenanceNoOp,
					StashID:                     req.StashID,
					HistoricalArtifactID:        historicalArtifactID,
					CanonicalDeliveryArtifactID: req.CanonicalDeliveryArtifactID,
					Forced:                      existing.Forced,
					Message:                     "stash provenance correction already recorded",
				}, nil
			}
		}
		if !req.Force && !resumeForced && existing.CanonicalDeliveryArtifactID != req.CanonicalDeliveryArtifactID {
			return nil, fmt.Errorf("conflicting provenance correction already exists for stash %s (existing canonical: %s, requested: %s): %w",
				req.StashID, existing.CanonicalDeliveryArtifactID, req.CanonicalDeliveryArtifactID, blerrors.ErrValidation)
		}
	}

	if resumeForced {
		original := cloneArtifact(target)
		artifactSnap, snapErr := snapshotFile(artifactPath)
		if snapErr != nil {
			return nil, fmt.Errorf("snapshot canonical artifact: %w", snapErr)
		}
		if mutErr := stampForcedProvenance(ctx, ws, target, entry, req); mutErr != nil {
			rollbackErrs := []error{restoreSnapshot(artifactSnap)}
			if upErr := bldb.UpsertItem(ctx, ws.DB, original); upErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("restore item index: %w", upErr))
			}
			if rbErr := errors.Join(rollbackErrs...); rbErr != nil {
				return nil, fmt.Errorf("resume forced provenance: %w; rollback: %v", mutErr, rbErr)
			}
			return nil, fmt.Errorf("resume forced provenance: %w", mutErr)
		}
		return &StashProvenanceCorrectionResult{
			Outcome:                     StashProvenanceCorrected,
			StashID:                     req.StashID,
			HistoricalArtifactID:        historicalArtifactID,
			CanonicalDeliveryArtifactID: req.CanonicalDeliveryArtifactID,
			Forced:                      true,
			Message:                     "incomplete forced stash provenance correction resumed",
		}, nil
	}

	// ---- Audit-before-mutation ----
	correctionsSnap, err := snapshotFile(correctionsPath)
	if err != nil {
		return nil, fmt.Errorf("snapshot provenance corrections: %w", err)
	}
	artifactSnap, err := snapshotFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("snapshot canonical artifact: %w", err)
	}
	original := cloneArtifact(target)

	// C1 ordering: append the durable item-log event BEFORE writing to
	// provenance_corrections.jsonl so an event failure leaves no ledger entry.
	eventDelta := map[string]any{
		"stash_id":                       req.StashID,
		"historical_artifact_id":         historicalArtifactID,
		"canonical_delivery_artifact_id": req.CanonicalDeliveryArtifactID,
		"reason":                         req.Reason,
		"actor":                          req.Actor,
	}
	if req.Force {
		eventDelta["forced"] = true
	}
	if eventErr := appendItemEventErr(ctx, ws, req.CanonicalDeliveryArtifactID, "stash_provenance_corrected", eventDelta); eventErr != nil {
		return nil, fmt.Errorf("append stash provenance event: %w", eventErr)
	}

	// C5: durable append of the correction record.
	correction := ProvenanceCorrection{
		StashID:                     req.StashID,
		HistoricalArtifactID:        historicalArtifactID,
		CanonicalDeliveryArtifactID: req.CanonicalDeliveryArtifactID,
		Reason:                      req.Reason,
		Actor:                       req.Actor,
		CorrectedAt:                 time.Now().UTC().Format(time.RFC3339),
		EventType:                   "stash_provenance_corrected",
		Forced:                      req.Force,
	}
	if appendErr := appendToProvenanceCorrections(correctionsPath, correction); appendErr != nil {
		return nil, fmt.Errorf("append provenance correction record: %w", appendErr)
	}

	// ---- Mutation (forced only), with byte-snapshot rollback ----
	if req.Force {
		if mutErr := stampForcedProvenance(ctx, ws, target, entry, req); mutErr != nil {
			rollbackErrs := []error{restoreSnapshot(correctionsSnap), restoreSnapshot(artifactSnap)}
			if upErr := bldb.UpsertItem(ctx, ws.DB, original); upErr != nil {
				rollbackErrs = append(rollbackErrs, fmt.Errorf("restore item index: %w", upErr))
			}
			if rbErr := errors.Join(rollbackErrs...); rbErr != nil {
				return nil, fmt.Errorf("apply forced provenance: %w; rollback: %v", mutErr, rbErr)
			}
			return nil, fmt.Errorf("apply forced provenance: %w", mutErr)
		}
	}

	return &StashProvenanceCorrectionResult{
		Outcome:                     StashProvenanceCorrected,
		StashID:                     req.StashID,
		HistoricalArtifactID:        historicalArtifactID,
		CanonicalDeliveryArtifactID: req.CanonicalDeliveryArtifactID,
		Forced:                      req.Force,
		Message:                     "stash provenance correction recorded",
	}, nil
}

// verifyProvenanceTargetContained rejects a canonical artifact path that is a
// symlink or resolves outside the workspace storage root.
func verifyProvenanceTargetContained(storageRoot, artifactPath string) error {
	if info, err := os.Lstat(artifactPath); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("canonical delivery artifact %q is a symlink; refusing to write through it: %w", artifactPath, blerrors.ErrValidation)
	}
	realRoot, err := filepath.EvalSymlinks(storageRoot)
	if err != nil {
		return fmt.Errorf("resolve workspace storage root: %w", err)
	}
	realArtifact, err := filepath.EvalSymlinks(artifactPath)
	if err != nil {
		return fmt.Errorf("resolve canonical delivery artifact path: %w", err)
	}
	if rel, relErr := filepath.Rel(realRoot, realArtifact); relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("canonical delivery artifact %q resolves outside workspace storage root: %w", artifactPath, blerrors.ErrValidation)
	}
	return nil
}

// forcedSnapshotApplied reports whether target and the SQLite index already
// carry the forced source_stash_* snapshot for req.
func forcedSnapshotApplied(ctx context.Context, ws *Workspace, target *models.Artifact, entry ArchivedStashEntry, req StashProvenanceCorrectionRequest) bool {
	priority := entry.Priority
	if priority == "" {
		priority = stash.DefaultPriority
	}
	cf := target.CustomFields
	if s, _ := cf["source_stash_id"].(string); s != req.StashID {
		return false
	}
	if s, _ := cf["source_stash_priority"].(string); s != priority {
		return false
	}
	if s, _ := cf["source_stash_kind"].(string); s != entry.Kind {
		return false
	}
	if s, _ := cf["source_stash_text"].(string); s != entry.Text {
		return false
	}
	var itemID string
	err := ws.DB.QueryRowContext(ctx, `SELECT item_id FROM stash_links WHERE stash_id = ?`, req.StashID).Scan(&itemID)
	return err == nil && itemID == req.CanonicalDeliveryArtifactID
}

// stampForcedProvenance writes the complete source_stash_* snapshot from the
// archived stash entry onto target and refreshes the SQLite items,
// stash_entries, and stash_links rows.
func stampForcedProvenance(ctx context.Context, ws *Workspace, target *models.Artifact, entry ArchivedStashEntry, req StashProvenanceCorrectionRequest) error {
	if target.CustomFields == nil {
		target.CustomFields = map[string]any{}
	}
	priority := entry.Priority
	if priority == "" {
		priority = stash.DefaultPriority
	}
	target.CustomFields["source_stash_id"] = req.StashID
	target.CustomFields["source_stash_priority"] = priority
	target.CustomFields["source_stash_kind"] = entry.Kind
	target.CustomFields["source_stash_text"] = entry.Text
	target.CustomFields["source_stash_path"] = stash.JSONLFileName
	if entry.DeliberationID != "" {
		target.CustomFields["source_deliberation_id"] = entry.DeliberationID
	} else {
		delete(target.CustomFields, "source_deliberation_id")
	}
	if err := persistArtifact(ctx, ws, target, false); err != nil {
		return fmt.Errorf("persist canonical artifact: %w", err)
	}
	now := time.Now().UTC()
	return bldb.UpsertAndLinkStashEntry(ctx, ws.DB, req.StashID, req.CanonicalDeliveryArtifactID, priority, entry.Kind, entry.Text,
		entry.DeliberationID, "harvested", stash.JSONLFileName, now)
}

// isProvenanceIDPathSafe reports whether id contains no path-traversal characters.
func isProvenanceIDPathSafe(id string) bool {
	return !strings.Contains(id, "..") &&
		!strings.Contains(id, "/") &&
		!strings.Contains(id, `\`)
}

// readStashArchiveEntry opens the stash JSONL archive at archivePath and returns
// the entry whose ID matches stashID. Returns a wrapped ErrNotFound when the
// archive is absent or the entry is not present.
// readStashArchiveEntry rejects symlinks at the file leaf via os.Lstat to
// prevent path traversal through a planted archive/stash.jsonl symlink.
func readStashArchiveEntry(archivePath, stashID string) (ArchivedStashEntry, error) {
	if linfo, lstatErr := os.Lstat(archivePath); lstatErr == nil && linfo.Mode()&os.ModeSymlink != 0 {
		return ArchivedStashEntry{}, fmt.Errorf("stash archive %q is a symlink; refusing to read through it", archivePath)
	}
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
// Path containment is verified via symlink-resolving filepath.EvalSymlinks
// before any write so a planted symlink cannot redirect writes outside the
// workspace (Copilot review cycle 2).
func appendToProvenanceCorrections(correctionsPath string, correction ProvenanceCorrection) error {
	// Verify the parent directory is not a symlink pointing outside the workspace.
	// We resolve the parent rather than the file itself because the file may not
	// exist yet on first write.
	parentDir := filepath.Dir(correctionsPath)
	if mkErr := os.MkdirAll(parentDir, 0o755); mkErr != nil {
		return fmt.Errorf("create corrections dir: %w", mkErr)
	}
	realParent, evalErr := filepath.EvalSymlinks(parentDir)
	if evalErr != nil {
		return fmt.Errorf("resolve corrections dir: %w", evalErr)
	}
	// Rebase correctionsPath on the resolved parent so the open uses a real path.
	resolvedPath := filepath.Join(realParent, filepath.Base(correctionsPath))

	// Additional check: if the corrections file already exists, verify it is not
	// itself a symlink (a planted leaf symlink would be followed by os.OpenFile,
	// redirecting writes outside the workspace) (thread ib).
	if info, statErr := os.Lstat(resolvedPath); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("provenance corrections file %q is a symlink; refusing to write through it", resolvedPath)
		}
	}

	correctionBytes, err := json.Marshal(correction)
	if err != nil {
		return fmt.Errorf("marshal provenance correction: %w", err)
	}
	line := append(correctionBytes, '\n')
	f, err := os.OpenFile(resolvedPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
	// Sync parent dir for directory entry durability on first write.
	if dirFile, dirErr := os.Open(realParent); dirErr == nil {
		_ = dirFile.Sync()
		_ = dirFile.Close()
	}
	return nil
}

// readLatestProvenanceCorrection reads correctionsPath (provenance_corrections.jsonl)
// and returns the LATEST correction recorded for stashID (a forced correction
// supersedes earlier ones). A missing file is not an error. It rejects symlinks
// at the file leaf and fails closed on unparseable lines.
func readLatestProvenanceCorrection(correctionsPath, stashID string) (ProvenanceCorrection, bool, error) {
	var latest ProvenanceCorrection
	if linfo, lstatErr := os.Lstat(correctionsPath); lstatErr == nil && linfo.Mode()&os.ModeSymlink != 0 {
		return latest, false, fmt.Errorf("provenance corrections file %q is a symlink; refusing to read through it", correctionsPath)
	}
	f, err := os.Open(correctionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return latest, false, nil
		}
		return latest, false, fmt.Errorf("open provenance corrections: %w", err)
	}
	defer func() { _ = f.Close() }()

	found := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw := scanner.Bytes()
		if len(raw) == 0 {
			continue
		}
		var c ProvenanceCorrection
		if jsonErr := json.Unmarshal(raw, &c); jsonErr != nil {
			// C4: fail closed on unparseable lines so a torn line cannot bypass
			// conflict detection.
			return ProvenanceCorrection{}, false, fmt.Errorf("parse provenance corrections line %q: %w", string(raw), jsonErr)
		}
		if c.StashID == stashID {
			latest = c
			found = true
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return ProvenanceCorrection{}, false, fmt.Errorf("scan provenance corrections: %w", scanErr)
	}
	return latest, found, nil
}
