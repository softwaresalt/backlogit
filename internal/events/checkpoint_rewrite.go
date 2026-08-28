package events

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/jsonutil"
)

// RewriteCheckpointFile is the only sanctioned in-place rewrite path for a
// stored checkpoint (147-F / U11, implemented by U13). QuarantineCheckpoint's
// verbatim move and CleanupCheckpoints' rename are explicitly excluded from
// this seam, since neither parses or re-marshals.
//
// Preconditions run in this exact order: filename validation and path
// containment (unchanged), ParseCheckpoint, ValidateCheckpoint,
// CheckConformingTopLevelNamespace, then mutate. Any precondition failure —
// or a mutate error — returns the raw verdict error (ErrCheckpointCorrupt,
// ErrCheckpointInvalid, or *CheckpointNonConformingError; a mutate error is
// returned as mutate returned it) before any marshal or write, and the file
// is left byte-unchanged. The seam never chooses a verb-facing sentinel
// (ErrCheckpointUseQuarantine, ErrCheckpointNonConforming); wrapping the
// verdict is the caller's job, because the wrap differs per verb and per
// gate-ordering rule (147-F / U12 contract, U14 caller migration).
func RewriteCheckpointFile(
	_ context.Context,
	checkpointDir, filename string,
	mutate func(*CheckpointV1) error,
) error {
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

	if err := ValidateCheckpoint(cp); err != nil {
		return err
	}

	if err := CheckConformingTopLevelNamespace(data); err != nil {
		return err
	}

	if err := mutate(cp); err != nil {
		return err
	}

	updated, err := jsonutil.MarshalReadable(cp)
	if err != nil {
		return fmt.Errorf("marshal rewritten checkpoint: %w", err)
	}

	// 147-F: route through atomicfile.WriteFileAtomic rather than the local
	// syncWriteFileAtomic helper (found during 130-S adversarial review).
	// syncWriteFileAtomic always writes with a hardcoded 0o644, silently
	// widening a more restrictive checkpoint's existing mode (e.g. 0600) on
	// every accepted rewrite, and its Windows path removes the destination
	// before os.Rename — a data-loss window if the rename then fails.
	// atomicfile.WriteFileAtomic preserves the destination's existing mode
	// and, on Windows, replaces via MoveFileEx(MOVEFILE_REPLACE_EXISTING),
	// which never removes the destination before the replacement commits.
	return atomicfile.WriteFileAtomic(path, updated)
}
