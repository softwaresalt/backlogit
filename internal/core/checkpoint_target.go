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

	// Verify the workspace storage root (.backlogit) itself has not escaped
	// the workspace via a symlink before trusting anything derived from it.
	// confineToStorageRoot's containment check is relative to
	// WorkspaceStorageRoot(ws.RootPath) as resolved (EvalSymlinks) — if
	// .backlogit itself is a symlink pointing entirely outside ws.RootPath,
	// that check would pass trivially (the target is "contained" within
	// wherever the symlink points), letting a disposition verb mutate files
	// far outside the workspace. This check proves .backlogit's real path
	// still lives under ws.RootPath's real path before anything else runs.
	if err := verifyStorageRootContained(ws); err != nil {
		return "", err
	}

	checkpointsDirAbs := filepath.Join(WorkspaceStorageRoot(ws.RootPath), checkpointsSubdir)

	// Reject a symlinked checkpoints directory itself, not just a symlinked
	// leaf file. confineToStorageRoot below only proves the resolved target is
	// contained somewhere under the workspace storage root (.backlogit); if
	// the checkpoints directory is a symlink to another location still inside
	// .backlogit (e.g. .backlogit/tasks), that broader containment check
	// alone would pass while the disposition verb rewrites or moves a file
	// that never actually lived in the intended checkpoints directory.
	if err := rejectSymlinkedDir(checkpointsDirAbs, "checkpoints directory"); err != nil {
		return "", err
	}

	absCandidate := filepath.Join(checkpointsDirAbs, filename)
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

// verifyStorageRootContained proves that WorkspaceStorageRoot(ws.RootPath)
// (.backlogit) resolves, after following any symlinks, to a location still
// contained within ws.RootPath's own resolved real path. A missing directory
// is not an error here (a fresh workspace may not have created .backlogit's
// checkpoints subtree yet); only an existing, escaping symlink chain is
// rejected.
func verifyStorageRootContained(ws *Workspace) error {
	absRoot, err := filepath.Abs(ws.RootPath)
	if err != nil {
		return fmt.Errorf("resolve workspace root: %w", err)
	}
	storageRoot := WorkspaceStorageRoot(ws.RootPath)
	absStorage, err := filepath.Abs(storageRoot)
	if err != nil {
		return fmt.Errorf("resolve storage root: %w", err)
	}

	realRoot, rootErr := filepath.EvalSymlinks(absRoot)
	if rootErr != nil {
		realRoot = absRoot
	}
	realStorage, storageErr := filepath.EvalSymlinks(absStorage)
	if storageErr != nil {
		// .backlogit does not exist yet (or a component is missing): nothing
		// to escape through. Not this function's concern.
		return nil
	}

	if realStorage != realRoot && !pathContained(realRoot, realStorage) {
		return fmt.Errorf("%w: workspace storage root (.backlogit) escapes the workspace via a symlink", blerrors.ErrCheckpointTargetUnsafe)
	}
	return nil
}

// rejectSymlinkedDir returns ErrCheckpointTargetUnsafe if path exists and is a
// symlink. A non-existent path is not an error here (callers create the
// directory afterward via os.MkdirAll); label is used only in the error
// message for diagnostics.
func rejectSymlinkedDir(path, label string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s must not be a symlink", blerrors.ErrCheckpointTargetUnsafe, label)
	}
	return nil
}



