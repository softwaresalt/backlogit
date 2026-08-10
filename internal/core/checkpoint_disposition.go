package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

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
				return writeBytesAtomically(target, updated, 0o644)
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
// rewrite) to WorkspaceStorageRoot(ws.RootPath)/archive/checkpoints, guarded
// by a clobber-refuse check against an existing destination file
// (ErrCheckpointDestinationOccupied). A disposition sidecar record is written
// as an idempotent upsert alongside the quarantined file. If the sidecar
// write fails after the move succeeds, the move is rolled back (renamed
// back) and diagnostics are logged — nothing is left half-quarantined.
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

	destDir := filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive", "checkpoints")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("quarantine checkpoint: create archive dir: %w", err)
	}
	destPath := filepath.Join(destDir, baseName)

	// Clobber-refuse guard before rename: never overwrite an existing
	// quarantined file with a different one that happens to share a filename.
	if _, statErr := os.Stat(destPath); statErr == nil {
		return fmt.Errorf("%w: %s", blerrors.ErrCheckpointDestinationOccupied, baseName)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("quarantine checkpoint: stat destination %s: %w", baseName, statErr)
	}

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
				return os.Rename(target, destPath)
			},
			Compensate: func(context.Context) error {
				if renameErr := os.Rename(destPath, target); renameErr != nil {
					slog.Error("quarantine checkpoint: failed to rename back after downstream failure",
						"file", baseName, "error", renameErr)
					return fmt.Errorf("rename back %s: %w", baseName, renameErr)
				}
				return nil
			},
		},
		{
			Name: "write-disposition-sidecar",
			Apply: func(context.Context) error {
				return writeBytesAtomically(sidecarPath, sidecarData, 0o644)
			},
		},
	})
	if err != nil {
		return fmt.Errorf("quarantine checkpoint %s: %w", baseName, err)
	}
	return nil
}

// writeBytesAtomically writes data to path via a temp-file-then-rename
// sequence, mirroring writeStringAtomically (stash.go) for byte slices.
func writeBytesAtomically(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename file %s: %w", path, err)
	}
	return nil
}
