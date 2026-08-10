package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// dispositionVerbAbandon and dispositionVerbQuarantine are the audit "verb"
// values recorded by appendCheckpointDispositionAudit.
const (
	dispositionVerbAbandon    = "abandon"
	dispositionVerbQuarantine = "quarantine"
)

// AbandonCheckpoint administratively abandons a checkpoint: it requires a
// parseable, schema-valid target with status=active (or any non-abandoned
// status), stamps disposition-labeled fields in place, and transitions
// Status to "abandoned".
//
// AbandonCheckpoint refuses to operate on a malformed (unparseable or
// schema-invalid) target, returning ErrCheckpointUseQuarantine so the caller
// can retry with QuarantineCheckpoint instead — the two verbs are disjoint by
// design (see docs/design-docs/checkpoint-administrative-disposition.md).
//
// An already-abandoned checkpoint is treated as an idempotent no-op: the
// original disposition_reason, disposition_operator, and disposition_at
// fields are preserved and neither the audit log nor the checkpoint file is
// rewritten.
//
// reason and operator must both be non-empty; operator is never defaulted to
// a fixed identity such as "backlogit". ew must be a real, non-nil
// *events.EventWriter — callers must never pass nil (the MCP server passes
// its shared writer; the CLI constructs a per-invocation writer via
// NewWorkspaceEventWriter, mirroring AssociateCommit's established pattern).
func AbandonCheckpoint(ctx context.Context, ws *Workspace, ew *events.EventWriter, filename, reason, operator string) error {
	if reason == "" {
		return blerrors.ErrCheckpointReasonRequired
	}
	if operator == "" {
		return blerrors.ErrCheckpointOperatorRequired
	}

	target, err := ResolveDispositionTarget(ws, filename)
	if err != nil {
		return err
	}
	baseName := filepath.Base(target)

	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", blerrors.ErrCheckpointNotFound, baseName)
		}
		return fmt.Errorf("abandon checkpoint: read %s: %w", baseName, err)
	}

	cp, parseErr := events.ParseCheckpoint(data)
	if parseErr != nil {
		return fmt.Errorf("%w: %v", blerrors.ErrCheckpointUseQuarantine, parseErr)
	}
	if valErr := events.ValidateCheckpoint(cp); valErr != nil {
		return fmt.Errorf("%w: %v", blerrors.ErrCheckpointUseQuarantine, valErr)
	}

	// Idempotent no-op: already abandoned. Preserve the original disposition
	// fields rather than overwriting reason/operator/timestamp on a repeat call.
	if cp.Disposition == events.DispositionAbandoned {
		return nil
	}

	// The U6 contract requires an active checkpoint (the already-abandoned
	// case above is the sole idempotent exception). Any other status (e.g.
	// "resolved") is a state conflict, not a silent transition to
	// "abandoned" — refuse rather than rewrite a checkpoint that was never
	// active in the first place.
	if cp.Status != "active" {
		return fmt.Errorf("%w: status=%s", blerrors.ErrCheckpointNotActive, cp.Status)
	}

	// Audit append happens BEFORE any rewrite of the checkpoint file. A failed
	// audit append (either failure class) leaves the target file untouched.
	if auditErr := appendCheckpointDispositionAudit(ctx, ew, baseName, dispositionVerbAbandon, reason, operator); auditErr != nil {
		return auditErr
	}

	now := time.Now().UTC()
	cp.Status = "abandoned"
	cp.Disposition = events.DispositionAbandoned
	cp.DispositionReason = reason
	cp.DispositionOperator = operator
	cp.DispositionAt = now
	cp.UpdatedAt = now

	updated, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("abandon checkpoint: marshal %s: %w", baseName, err)
	}

	err = MutationEnvelope(ctx, []MutationStep{
		{
			Name: "rewrite-checkpoint",
			Apply: func(context.Context) error {
				// atomicfile.WriteFileAtomic replaces an existing destination
				// correctly on Windows (a plain os.Rename(tmp, path) fails
				// there when path already exists), unlike a hand-rolled
				// temp-then-rename helper.
				return atomicfile.WriteFileAtomic(target, updated)
			},
		},
	})
	if err != nil {
		return fmt.Errorf("abandon checkpoint %s: %w", baseName, err)
	}
	return nil
}

// QuarantineCheckpoint administratively quarantines a checkpoint: it
// classifies the target WITHOUT rewriting it (read bytes, attempt
// parse+validate in memory), and only a malformed (unparseable or
// schema-invalid) target proceeds. A parseable, schema-valid target is
// refused with ErrCheckpointUseAbandon so the caller can retry with
// AbandonCheckpoint instead.
//
// On success, the checkpoint's bytes are moved VERBATIM (byte-identical, no
// rewrite) to WorkspaceStorageRoot(ws.RootPath)/archive/checkpoints via an
// atomic link-then-remove sequence that cannot silently clobber a
// concurrently-created destination (ErrCheckpointDestinationOccupied). A
// disposition sidecar record is written as an idempotent upsert alongside the
// quarantined file. If the sidecar write fails after the move succeeds, the
// move is rolled back and diagnostics are logged — nothing is left
// half-quarantined.
//
// reason and operator must both be non-empty; operator is never defaulted to
// a fixed identity such as "backlogit". ew must be a real, non-nil
// *events.EventWriter.
func QuarantineCheckpoint(ctx context.Context, ws *Workspace, ew *events.EventWriter, filename, reason, operator string) error {
	if reason == "" {
		return blerrors.ErrCheckpointReasonRequired
	}
	if operator == "" {
		return blerrors.ErrCheckpointOperatorRequired
	}

	target, err := ResolveDispositionTarget(ws, filename)
	if err != nil {
		return err
	}
	baseName := filepath.Base(target)

	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", blerrors.ErrCheckpointNotFound, baseName)
		}
		return fmt.Errorf("quarantine checkpoint: read %s: %w", baseName, err)
	}

	// Classify WITHOUT rewriting: attempt parse+validate purely in memory.
	cp, parseErr := events.ParseCheckpoint(data)
	validTarget := parseErr == nil
	var valErr error
	if validTarget {
		valErr = events.ValidateCheckpoint(cp)
		validTarget = valErr == nil
	}
	if validTarget {
		return blerrors.ErrCheckpointUseAbandon
	}

	archiveDir := filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive")
	if err := rejectSymlinkedDir(archiveDir, "archive directory"); err != nil {
		return err
	}
	destDir := filepath.Join(archiveDir, "checkpoints")
	if err := rejectSymlinkedDir(destDir, "archive checkpoints directory"); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("quarantine checkpoint: create archive dir: %w", err)
	}
	destPath := filepath.Join(destDir, baseName)

	// Audit append happens BEFORE any move. A failed audit append (either
	// failure class) leaves the target file untouched.
	if auditErr := appendCheckpointDispositionAudit(ctx, ew, baseName, dispositionVerbQuarantine, reason, operator); auditErr != nil {
		return auditErr
	}

	rec := events.CheckpointDispositionRecord{
		Filename:      baseName,
		Disposition:   events.DispositionQuarantined,
		Reason:        reason,
		Operator:      operator,
		DispositionAt: time.Now().UTC(),
	}
	sidecarData, err := events.MarshalDispositionRecord(rec)
	if err != nil {
		return fmt.Errorf("quarantine checkpoint: marshal disposition sidecar: %w", err)
	}
	sidecarPath := events.CheckpointDispositionSidecarPath(destPath)

	err = MutationEnvelope(ctx, []MutationStep{
		{
			Name: "move-verbatim",
			Apply: func(context.Context) error {
				return moveNoReplace(target, destPath)
			},
			Compensate: func(context.Context) error {
				if restoreErr := moveNoReplace(destPath, target); restoreErr != nil {
					slog.Error("quarantine checkpoint: failed to restore after downstream failure",
						"file", baseName, "error", restoreErr)
					return fmt.Errorf("restore %s: %w", baseName, restoreErr)
				}
				return nil
			},
		},
		{
			Name: "write-disposition-sidecar",
			Apply: func(context.Context) error {
				return atomicfile.WriteFileAtomic(sidecarPath, sidecarData)
			},
		},
	})
	if err != nil {
		// MutationPartialError.Unwrap returns Cause, so errors.Is traverses
		// through to the underlying *os.LinkError / syscall errno raised by
		// moveNoReplace's os.Link when the destination already exists.
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%w: %s", blerrors.ErrCheckpointDestinationOccupied, baseName)
		}
		return fmt.Errorf("quarantine checkpoint %s: %w", baseName, err)
	}
	return nil
}

// moveNoReplace moves src to dst atomically without ever clobbering an
// existing file at dst, closing a TOCTOU race that a separate
// os.Stat-then-os.Rename check cannot: os.Link fails with EEXIST if dst
// already exists (created by a concurrent quarantine call, for example),
// and no bytes at dst are ever overwritten. Once the link is established the
// original is removed; if that removal fails the just-created link is
// unwound so no duplicate is left behind.
func moveNoReplace(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		_ = os.Remove(dst)
		return fmt.Errorf("remove source after link %s: %w", src, err)
	}
	return nil
}


