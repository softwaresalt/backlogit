package telemetry

import "path/filepath"

// workspaceStorageRoot returns the storage root to use for telemetry paths.
// It accepts either an already-resolved storage-root directory (whose base
// matches one of the supported candidate names) or a parent directory and
// returns the path unchanged in both cases — callers that hold a *core.Workspace
// should pass ws.StorageRoot directly so that the immutable resolved root is
// honoured and the closed-set / override / fail-closed contract is preserved.
// A plain parent-directory path falls through unchanged; callers that cannot
// yet supply a pre-resolved storage root accept the resulting path as-is.
func workspaceStorageRoot(workspacePath string) string {
	return filepath.Clean(workspacePath)
}
