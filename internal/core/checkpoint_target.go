package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// checkpointsSubdir is the workspace-storage-root-relative directory holding
// checkpoint files, mirroring internal/cli's checkpointDir helper and the MCP
// server's checkpoint tool handlers.
const checkpointsSubdir = "checkpoints"

// ResolveDispositionTarget resolves filename (a checkpoint basename) to an
// absolute path within the workspace's checkpoints directory, refusing any
// input that could escape workspace confinement.
//
// filename must be a bare basename: no path separators, no "..", not absolute,
// not volume-qualified (e.g. "C:"), and not a UNC path (e.g. "\\host\share").
// ResolveDispositionTarget is a thin adapter over the existing
// confineToStorageRoot containment primitive (doctor_target.go) so the
// disposition verbs (AbandonCheckpoint, QuarantineCheckpoint) share the same
// confinement guarantee as doctor's single-file validation path, rather than
// re-implementing path-escape defenses.
//
// As a final defense specific to disposition semantics, ResolveDispositionTarget
// rejects a target that resolves to a symlink (via os.Lstat), regardless of
// where the symlink points. The disposition verbs require operating on the
// checkpoint's actual on-disk bytes (verbatim move for quarantine, in-place
// rewrite for abandon); a symlinked target could otherwise be used to redirect
// a disposition action at an arbitrary file outside the caller's intent.
func ResolveDispositionTarget(ws *Workspace, filename string) (string, error) {
	if filename == "" {
		return "", fmt.Errorf("%w: filename must not be empty", blerrors.ErrCheckpointTargetUnsafe)
	}
	if filename == "." || filename == ".." {
		return "", fmt.Errorf("%w: filename must not be \".\" or \"..\"", blerrors.ErrCheckpointTargetUnsafe)
	}
	if strings.ContainsAny(filename, `/\`) {
		return "", fmt.Errorf("%w: filename must be a basename without path separators", blerrors.ErrCheckpointTargetUnsafe)
	}
	if strings.Contains(filename, "..") {
		return "", fmt.Errorf("%w: filename must not contain \"..\"", blerrors.ErrCheckpointTargetUnsafe)
	}
	if filepath.IsAbs(filename) {
		return "", fmt.Errorf("%w: filename must not be an absolute path", blerrors.ErrCheckpointTargetUnsafe)
	}
	// Volume-qualified (e.g. "C:") and UNC (e.g. "\\host\share") prefixes are
	// rejected explicitly: on non-Windows hosts filepath.IsAbs would not catch
	// these, and even on Windows a bare basename must never contain a volume
	// or share prefix once path separators are already forbidden above.
	if len(filename) >= 2 && filename[1] == ':' {
		return "", fmt.Errorf("%w: filename must not be volume-qualified", blerrors.ErrCheckpointTargetUnsafe)
	}
	if strings.HasPrefix(filename, `\\`) || strings.HasPrefix(filename, "//") {
		return "", fmt.Errorf("%w: filename must not be a UNC path", blerrors.ErrCheckpointTargetUnsafe)
	}

	absCandidate := filepath.Join(WorkspaceStorageRoot(ws.RootPath), checkpointsSubdir, filename)
	absTarget, inScope, err := confineToStorageRoot(ws, absCandidate)
	if err != nil {
		return "", fmt.Errorf("resolve checkpoint disposition target: %w", err)
	}
	if !inScope {
		return "", fmt.Errorf("%w: target escapes checkpoints directory", blerrors.ErrCheckpointTargetUnsafe)
	}

	if info, lerr := os.Lstat(absTarget); lerr == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("%w: target is a symlink", blerrors.ErrCheckpointTargetUnsafe)
	}

	return absTarget, nil
}

