package events

import (
	"bytes"
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

	// 153.001-T (302EFF07 / adversarial-review F1 remediation): use the
	// no-follow read to close the TOCTOU race between ensurePathContained's
	// Lstat-based pre-check and the actual read. On Unix this is O_NOFOLLOW
	// (kernel-level protection); on Windows it is best-effort post-open Lstat.
	data, err := readCheckpointFileNoFollow(path)
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

	// 147-F: re-verify the on-disk bytes immediately before committing the
	// replacement (found during 130-S adversarial review). Also uses
	// readCheckpointFileNoFollow to maintain the no-follow invariant for
	// this second read (153.001-T / adversarial-review F1 remediation).
	current, err := readCheckpointFileNoFollow(path)
	if err != nil {
		return fmt.Errorf("re-read checkpoint %s before write: %w", filename, err)
	}
	if !bytes.Equal(data, current) {
		return fmt.Errorf("checkpoint content changed since classification; refusing rewrite: %w: %s",
			backlogiterrors.ErrCheckpointContentChanged, filename)
	}

	// 147-F: route through atomicfile.WriteFileAtomicWithOptions rather than
	// the local syncWriteFileAtomic helper (found during 130-S adversarial
	// review). syncWriteFileAtomic always writes with a hardcoded 0o644,
	// silently widening a more restrictive checkpoint's existing mode (e.g.
	// 0600) on every accepted rewrite, and its Windows path removes the
	// destination before os.Rename — a data-loss window if the rename then
	// fails. atomicfile.WriteFileAtomic* preserves the destination's
	// existing mode and, on Windows, replaces via
	// MoveFileEx(MOVEFILE_REPLACE_EXISTING), which never removes the
	// destination before the replacement commits.
	//
	// DurableWrites: true is required (not the plain WriteFileAtomic
	// fast path) to preserve the fsync-before-rename guarantee
	// syncWriteFileAtomic always provided — omitting it would have silently
	// regressed durability: a "successful" resolve/abandon could be lost
	// after a crash or power failure before the OS itself flushed the
	// rename to disk (also found during 130-S adversarial review). A
	// resulting ErrWriteIndeterminate (parent-dir fsync failed after the
	// rename already committed) or ErrWriteNotApplied (temp-file fsync
	// failed before the rename) propagates to the caller unwrapped by this
	// seam; AbandonCheckpoint's MutationEnvelope already classifies both
	// generically, and domainError classifies them for ResolveCheckpoint's
	// un-enveloped callers.
	return atomicfile.WriteFileAtomicWithOptions(path, updated, atomicfile.Options{DurableWrites: true})
}
