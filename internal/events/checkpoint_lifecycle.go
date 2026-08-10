package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
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
// RemediationCommand an operator or agent can run to quarantine the file via
// QuarantineCheckpoint. Prior to 136-F, ListCheckpoints physically quarantined
// unparseable files as a side effect of listing; that side effect has been
// removed so listing can never mutate workspace state.
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
				Filename:            filename,
				ValidationErr:       parseErr.Error(),
				NeedsQuarantine:     true,
				RemediationCommand:  fmt.Sprintf("backlogit checkpoint quarantine %s --reason <reason>", filename),
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
			summary.RemediationCommand = fmt.Sprintf("backlogit checkpoint quarantine %s --reason <reason>", filename)
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

	data, err := os.ReadFile(path)
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

// ResolveCheckpoint marks a checkpoint as resolved (idempotent).
func ResolveCheckpoint(_ context.Context, checkpointDir, filename string) error {
	if err := validateCheckpointFilename(filename); err != nil {
		return err
	}

	path := filepath.Join(checkpointDir, filename)
	if err := ensurePathContained(checkpointDir, path); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", backlogiterrors.ErrCheckpointNotFound, filename)
		}
		return fmt.Errorf("read checkpoint %s: %w", filename, err)
	}

	cp, err := ParseCheckpoint(data)
	if err != nil {
		return err
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

	cp.Status = "resolved"
	cp.UpdatedAt = time.Now().UTC()

	updated, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("marshal resolved checkpoint: %w", err)
	}

	return syncWriteFileAtomic(path, updated, 0o644)
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
		if runtime.GOOS == "windows" {
			_ = os.Remove(dst)
		}
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
	return nil
}

