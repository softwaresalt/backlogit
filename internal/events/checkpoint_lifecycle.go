package events

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
)

// ListCheckpoints returns checkpoint summaries from checkpointDir, applying optional filter.
//
// ListCheckpoints is read-only (136-F/U9): it NEVER moves, deletes, or rewrites
// any checkpoint file, and it never fails due to a disposition- or audit-related
// error. Files that fail to parse are surfaced with NeedsQuarantine=true and a
// structured RemediationIntent describing how to quarantine the file via
// QuarantineCheckpoint. Prior to 136-F, ListCheckpoints physically quarantined
// unparseable files as a side effect of listing; that side effect has been
// removed so listing can never mutate workspace state.
//
// Any summary with NeedsQuarantine: true bypasses every optional filter
// (Agent, Status, ShipmentID, FeatureID, MaxAge) and is always returned
// (147-F / U6d): a quarantine candidate's own fields are untrusted data, so
// filtering on them could silently hide the remediation surface from
// exactly the query an agent runs at session start. A conforming,
// schema-valid document that simply does not match a filter is still
// dropped as usual.
func ListCheckpoints(_ context.Context, checkpointDir string, filter CheckpointFilter) ([]CheckpointSummary, error) {
	pattern := filepath.Join(checkpointDir, "checkpoint-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob checkpoints: %w", err)
	}

	var summaries []CheckpointSummary
	now := time.Now().UTC()

	for _, path := range matches {
		filename := filepath.Base(path)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			slog.Warn("checkpoint read failed", "file", filename, "error", readErr)
			continue
		}

		cp, parseErr := ParseCheckpoint(data)
		if parseErr != nil {
			// Read-only: surface the parse failure as a quarantine candidate
			// instead of physically moving the file. QuarantineCheckpoint
			// (136-F/U7) performs the actual move when invoked explicitly.
			summaries = append(summaries, CheckpointSummary{
				Filename:        filename,
				ValidationErr:   parseErr.Error(),
				NeedsQuarantine: true,
				RemediationIntent: &RemediationIntent{
					Verb:             "quarantine",
					TargetFilename:   filename,
					RequiresApproval: true,
					ApprovalClass:    "A4c",
					Reason:           "unparseable",
				},
			})
			continue
		}

		valErr := ValidateCheckpoint(cp)
		summary := CheckpointSummary{
			Filename:   filename,
			Agent:      cp.Agent,
			SessionID:  cp.SessionID,
			Phase:      cp.Phase,
			Status:     cp.Status,
			CreatedAt:  cp.CreatedAt,
			ShipmentID: cp.Context.ShipmentID,
			FeatureID:  cp.Context.FeatureID,
			ResumeHint: cp.ResumeHint,
		}
		if valErr != nil {
			summary.ValidationErr = valErr.Error()
			summary.NeedsQuarantine = true
			summary.RemediationIntent = &RemediationIntent{
				Verb:             "quarantine",
				TargetFilename:   filename,
				RequiresApproval: true,
				ApprovalClass:    "A4c",
				Reason:           "schema_invalid",
			}
		}

		// Conformance check (147-F / U6): runs regardless of valErr, so a
		// document can fail both validation and conformance and the operator
		// sees both reasons. Publishes structured RemediationIntent, never a
		// command string — internal/events has no knowledge of the caller's
		// working directory (cycle-17 gate finding H1). The parse-failure
		// and schema-invalid branches above already publish only structured
		// remediation metadata; this unit adds no executable command string.
		// When a document is both schema-invalid and non-conforming, this
		// branch runs after the validity branch and overwrites Reason with
		// "non_conforming", matching the ValidationErr append order that
		// already reports both reasons (147-F / U6e precedence rule).
		if confErr := CheckConformingTopLevelNamespace(data); confErr != nil {
			summary.NeedsQuarantine = true
			if summary.ValidationErr != "" {
				summary.ValidationErr += "; " + confErr.Error()
			} else {
				summary.ValidationErr = confErr.Error()
			}
			summary.RemediationIntent = &RemediationIntent{
				Verb:             "quarantine",
				TargetFilename:   filename,
				RequiresApproval: true,
				ApprovalClass:    "A4c",
				Reason:           "non_conforming",
			}
		}

		// A quarantine candidate bypasses every optional filter (147-F /
		// U6d): its own fields are untrusted data, and hiding the
		// remediation surface behind a filter it may not even legitimately
		// match would be worse than an unfiltered false positive.
		if summary.NeedsQuarantine {
			summaries = append(summaries, summary)
			continue
		}

		// Apply filters.
		if filter.Agent != "" && cp.Agent != filter.Agent {
			continue
		}
		if filter.Status != "" && cp.Status != filter.Status {
			continue
		}
		if filter.ShipmentID != "" && cp.Context.ShipmentID != filter.ShipmentID {
			continue
		}
		if filter.FeatureID != "" && cp.Context.FeatureID != filter.FeatureID {
			continue
		}
		if filter.MaxAge > 0 && now.Sub(cp.CreatedAt) > filter.MaxAge {
			continue
		}

		summaries = append(summaries, summary)
	}

	// Sort by created_at descending (most recent first).
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].CreatedAt.After(summaries[j].CreatedAt)
	})

	return summaries, nil
}

// GetCheckpoint reads and validates a specific checkpoint file.
// Returns ErrCheckpointNotFound if the file doesn't exist,
// ErrCheckpointInvalid if validation fails.
func GetCheckpoint(_ context.Context, checkpointDir, filename string) (*CheckpointV1, error) {
	if err := validateCheckpointFilename(filename); err != nil {
		return nil, err
	}

	path := filepath.Join(checkpointDir, filename)
	if err := ensurePathContained(checkpointDir, path); err != nil {
		return nil, err
	}

	data, err := readCheckpointFileNoFollow(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", backlogiterrors.ErrCheckpointNotFound, filename)
		}
		return nil, fmt.Errorf("read checkpoint %s: %w", filename, err)
	}

	cp, err := ParseCheckpoint(data)
	if err != nil {
		return nil, err
	}

	if err := ValidateCheckpoint(cp); err != nil {
		return nil, err
	}

	return cp, nil
}

// CheckpointReadResult is the structured read result for a checkpoint,
// carrying conformance and remediation metadata alongside the parsed
// document (147-F / U15). This declaration adds no conformance evaluation,
// no intent population, and no offender projection of its own — every field
// beyond Checkpoint and Valid starts at its zero value here; U6b's
// production delta populates them from a live read.
type CheckpointReadResult struct {
	Checkpoint          *CheckpointV1
	Valid               bool
	Conforming          bool
	NeedsQuarantine     bool
	RemediationIntent   *RemediationIntent
	NonConformingFields backlogiterrors.BoundedFieldPathSet
}

// GetCheckpointResult reads and validates a specific checkpoint file,
// returning a CheckpointReadResult. It reads the file's bytes exactly once
// and derives parsing, validation, and conformance all from that same
// snapshot (147-F, found during 130-S adversarial review) — an earlier
// version called GetCheckpoint (which performs its own internal read) and
// then re-read the same path a second time to run the conformance check.
// If the file changed between those two reads, the returned Checkpoint and
// conformance metadata could describe different byte sequences; if the
// second read failed, the result was misreported as conforming rather than
// surfacing the read error. On error, the error is returned unwrapped — a
// read is not a rewrite, so ErrCheckpointInvalid still resolves via
// errors.Is and QuarantineIsRemedy(err) is false; there is nothing to
// refuse (147-F / U15).
//
// On success, Conforming, NeedsQuarantine, RemediationIntent, and
// NonConformingFields are populated by running
// CheckConformingTopLevelNamespace against the same bytes that produced
// Checkpoint (147-F / U6b). NonConformingFields is recovered via errors.As
// from the conformance verdict and produced by
// CheckpointNonConformingError.BoundedFieldPaths — never re-derived or
// re-capped here — so `checkpoint get` stays an atomic, bounded, per-file
// offender source with machine-checkable truncation metadata. valid
// retains its existing (schema-valid) meaning; conforming is reported as a
// distinct field so no existing consumer's contract silently changes.
func GetCheckpointResult(_ context.Context, checkpointDir, filename string) (*CheckpointReadResult, error) {
	if err := validateCheckpointFilename(filename); err != nil {
		return nil, err
	}

	path := filepath.Join(checkpointDir, filename)
	if err := ensurePathContained(checkpointDir, path); err != nil {
		return nil, err
	}

	data, err := readCheckpointFileNoFollow(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", backlogiterrors.ErrCheckpointNotFound, filename)
		}
		return nil, fmt.Errorf("read checkpoint %s: %w", filename, err)
	}

	cp, err := ParseCheckpoint(data)
	if err != nil {
		return nil, err
	}
	if err := ValidateCheckpoint(cp); err != nil {
		return nil, err
	}

	result := &CheckpointReadResult{
		Checkpoint: cp,
		Valid:      true,
		Conforming: true,
		// NonConformingFields.Paths defaults to a non-nil empty slice
		// (never left nil) so a conforming document still marshals
		// "paths": [] rather than "paths": null on every JSON-projecting
		// surface (docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md).
		NonConformingFields: backlogiterrors.BoundedFieldPathSet{Paths: []string{}},
	}

	if confErr := CheckConformingTopLevelNamespace(data); confErr != nil {
		result.Conforming = false
		result.NeedsQuarantine = true
		var typed *backlogiterrors.CheckpointNonConformingError
		if errors.As(confErr, &typed) {
			result.NonConformingFields = typed.BoundedFieldPaths()
		}
		result.RemediationIntent = &RemediationIntent{
			Verb:             "quarantine",
			TargetFilename:   filename,
			RequiresApproval: true,
			ApprovalClass:    "A4c",
			Reason:           "non_conforming",
		}
	}

	return result, nil
}

// ResolveCheckpoint marks a checkpoint as resolved (idempotent).
func ResolveCheckpoint(ctx context.Context, checkpointDir, filename string) error {
	if err := validateCheckpointFilename(filename); err != nil {
		return err
	}

	path := filepath.Join(checkpointDir, filename)
	if err := ensurePathContained(checkpointDir, path); err != nil {
		return err
	}

	data, err := readCheckpointFileNoFollow(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", backlogiterrors.ErrCheckpointNotFound, filename)
		}
		return fmt.Errorf("read checkpoint %s: %w", filename, err)
	}

	cp, err := ParseCheckpoint(data)
	if err != nil {
		// 147-F: multi-%w so both errors.Is(err, ErrCheckpointUseQuarantine)
		// and errors.Is(err, ErrCheckpointCorrupt) hold — matching the same
		// disposition-refusal shape the schema-invalid-but-parseable class
		// gets below, so the two "does not parse" / "parses but
		// schema-invalid" rows of the four-class contract stay identical in
		// remedy verb, and QuarantineIsRemedy(err) is true for both.
		return fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointUseQuarantine, err)
	}

	// Idempotent: already resolved.
	if cp.Status == "resolved" {
		return nil
	}

	// Refuse to resolve an administratively abandoned checkpoint. Abandon
	// (136-F) is a terminal, non-resumable disposition; silently flipping
	// Status to "resolved" here would erase that terminal state and let
	// resolve undo an abandon disposition.
	if cp.Disposition == DispositionAbandoned {
		return fmt.Errorf("%w: %s", backlogiterrors.ErrCheckpointCannotResolveAbandoned, filename)
	}

	// 147-F / U3: refuse a schema-invalid document rather than replacing it
	// with a fabricated skeleton. Multi-%w (not %v — Q2) so both
	// errors.Is(err, ErrCheckpointUseQuarantine) and errors.Is(err,
	// ErrCheckpointInvalid) hold: the caller learns both the remedy verb and
	// the underlying validation-class reason. This gate does not write; the
	// file is left byte-identical on refusal.
	if valErr := ValidateCheckpoint(cp); valErr != nil {
		return fmt.Errorf("%w: %w", backlogiterrors.ErrCheckpointUseQuarantine, valErr)
	}

	// 147-F / U14: the rewrite itself routes through the guarded seam, which
	// requires ParseCheckpoint, ValidateCheckpoint, and
	// CheckConformingTopLevelNamespace to all succeed before any marshal or
	// write. A valid-but-non-conforming or schema-invalid document is
	// therefore refused here with the seam's raw verdict error, rather than
	// rewritten with a fabricated skeleton. This introduces no new
	// verb-facing sentinel and changes no ordering: the idempotent and
	// abandoned-disposition checks above still run first, against the same
	// initial read. resolveCheckpointMutate additionally re-checks
	// disposition against the seam's own fresh read (see its doc comment).
	// NormalizeSeamMalformedVerdict wraps a between-read-race malformed
	// verdict the same way this function's own earlier read is normalized
	// above (found during 130-S adversarial review; see its doc comment).
	return backlogiterrors.NormalizeSeamMalformedVerdict(
		RewriteCheckpointFile(ctx, checkpointDir, filename, resolveCheckpointMutate(filename)))
}

// resolveCheckpointMutate returns the mutate callback ResolveCheckpoint
// passes to RewriteCheckpointFile. It is a named function (rather than an
// inline closure) so a test can invoke RewriteCheckpointFile with this exact
// production logic directly, against a checkpoint whose on-disk content
// reflects a state RewriteCheckpointFile's own independent read observes —
// which may differ from what ResolveCheckpoint's earlier classification read
// observed if a concurrent AbandonCheckpoint won the race in between (147-F,
// found during 130-S adversarial review). The disposition re-check below is
// what keeps that race from silently flipping status to "resolved" on an
// already-abandoned document.
func resolveCheckpointMutate(filename string) func(*CheckpointV1) error {
	return func(cp *CheckpointV1) error {
		if cp.Disposition == DispositionAbandoned {
			return fmt.Errorf("%w: %s", backlogiterrors.ErrCheckpointCannotResolveAbandoned, filename)
		}
		cp.Status = "resolved"
		cp.UpdatedAt = time.Now().UTC()
		return nil
	}
}

// CleanupCheckpoints archives resolved and stale checkpoints. retentionDays must be > 0.
func CleanupCheckpoints(_ context.Context, checkpointDir string, retentionDays int) (CleanupResult, error) {
	if retentionDays <= 0 {
		return CleanupResult{}, fmt.Errorf("retentionDays must be > 0, got %d", retentionDays)
	}

	archiveDir := filepath.Join(filepath.Dir(checkpointDir), "archive", "checkpoints")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return CleanupResult{}, fmt.Errorf("create archive dir: %w", err)
	}

	pattern := filepath.Join(checkpointDir, "checkpoint-*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return CleanupResult{}, fmt.Errorf("glob checkpoints: %w", err)
	}

	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	result := CleanupResult{}

	for _, path := range matches {
		filename := filepath.Base(path)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("read %s: %v", filename, readErr))
			result.SkippedCount++
			continue
		}

		cp, parseErr := ParseCheckpoint(data)
		if parseErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("parse %s: %v", filename, parseErr))
			result.SkippedCount++
			continue
		}

		// Validate before eligibility check so invalid files are skipped rather
		// than being silently archived (e.g., zero-time created_at would always
		// appear stale).
		if valErr := ValidateCheckpoint(cp); valErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("validate %s: %v", filename, valErr))
			result.SkippedCount++
			continue
		}

		// Archive if resolved OR stale (older than retention).
		eligible := cp.Status == "resolved" || cp.CreatedAt.Before(cutoff)
		if !eligible {
			result.SkippedCount++
			continue
		}

		dst := filepath.Join(archiveDir, filename)
		if mvErr := os.Rename(path, dst); mvErr != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("archive %s: %v", filename, mvErr))
			result.SkippedCount++
			continue
		}

		result.ArchivedCount++
		result.ArchivedFiles = append(result.ArchivedFiles, filename)
	}

	return result, nil
}

// validateCheckpointFilename rejects invalid checkpoint filenames and only allows
// files in the expected checkpoint-*.json namespace.
func validateCheckpointFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("%w: filename must not be empty", backlogiterrors.ErrCheckpointInvalid)
	}
	if strings.ContainsAny(filename, `/\`) {
		return fmt.Errorf("%w: filename must be a basename without path separators", backlogiterrors.ErrCheckpointInvalid)
	}
	if !strings.HasPrefix(filename, "checkpoint-") || !strings.HasSuffix(filename, ".json") {
		return fmt.Errorf("%w: filename must match checkpoint-*.json", backlogiterrors.ErrCheckpointInvalid)
	}
	checkpointID := strings.TrimSuffix(strings.TrimPrefix(filename, "checkpoint-"), ".json")
	if checkpointID == "" {
		return fmt.Errorf("%w: filename must include a checkpoint identifier", backlogiterrors.ErrCheckpointInvalid)
	}
	return nil
}

// ensurePathContained verifies that resolved path is under the expected dir.
func ensurePathContained(dir, path string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve dir: %w", err)
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	if !strings.HasPrefix(absPath, absDir+string(filepath.Separator)) && absPath != absDir {
		return fmt.Errorf("%w: path escapes checkpoint directory", backlogiterrors.ErrCheckpointInvalid)
	}

	// 153.001-T (302EFF07): reject symlinks at any component of the path
	// chain from dir (inclusive) to path (inclusive), preventing both the
	// leaf-symlink and intermediate-dir-symlink escape vectors.
	if err := rejectSymlinksInPathChain(absDir, absPath); err != nil {
		return err
	}

	return nil
}

// rejectSymlinksInPathChain checks every filesystem path component from base
// (inclusive) to path (inclusive) using os.Lstat. If any existing component
// is a symlink it returns a wrapped ErrCheckpointTargetUnsafe. A missing
// component (os.ErrNotExist) stops the walk without error — the caller's
// subsequent read will surface the absence. Permission, I/O, or transient
// errors are propagated fail-closed: an inaccessible component cannot be
// proven symlink-free, so the chain is rejected (adversarial-review Copilot
// finding #1 remediation, 153.001-T).
func rejectSymlinksInPathChain(base, path string) error {
	current := path
	for {
		info, lerr := os.Lstat(current)
		if lerr == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: path component is a symlink", backlogiterrors.ErrCheckpointTargetUnsafe)
		}
		if lerr != nil {
			if os.IsNotExist(lerr) {
				// Component not found; stop. Caller's read will surface absence.
				return nil
			}
			// Permission, I/O, or transient error — fail closed: an inaccessible
			// component cannot be verified as symlink-free.
			return fmt.Errorf("%w: cannot verify path component is not a symlink: %w",
				backlogiterrors.ErrCheckpointTargetUnsafe, lerr)
		}
		if current == base {
			break // all components including base checked
		}
		parent := filepath.Dir(current)
		if parent == current {
			break // filesystem root reached
		}
		current = parent
	}
	return nil
}
