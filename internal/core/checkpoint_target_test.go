package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// newCheckpointTargetTestWorkspace builds an isolated workspace in a fresh
// t.TempDir() with a checkpoints directory ready for target-resolution tests.
// Each test gets its own workspace root (no t.Parallel(); per plan constraint).
func newCheckpointTargetTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	ctx := context.Background()
	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, checkpointsSubdir), 0o755))
	return ws
}

// TestResolveDispositionTarget_ConfinementMatrix is the 136-F/U4 failing
// confinement matrix (now green against the U5 implementation): a
// table-driven sweep of every rejection class ResolveDispositionTarget must
// enforce, plus the one accept case (a bare basename).
func TestResolveDispositionTarget_ConfinementMatrix(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		wantErr   bool
		wantIsErr error
	}{
		{name: "bare filename accepted", filename: "checkpoint-20260810-100000.json", wantErr: false},
		{name: "empty filename rejected", filename: "", wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
		{name: "forward slash separator rejected", filename: "sub/checkpoint-1.json", wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
		{name: "backslash separator rejected", filename: `sub\checkpoint-1.json`, wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
		{name: "dot-dot traversal rejected", filename: "../checkpoint-1.json", wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
		{name: "bare dot-dot rejected", filename: "..", wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
		{name: "absolute path rejected", filename: filepath.Join(os.TempDir(), "checkpoint-1.json"), wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
		{name: "volume-qualified path rejected", filename: `C:checkpoint-1.json`, wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
		{name: "UNC path rejected", filename: `\\host\share\checkpoint-1.json`, wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
		{name: "double-slash UNC-style path rejected", filename: `//host/share/checkpoint-1.json`, wantErr: true, wantIsErr: blerrors.ErrCheckpointTargetUnsafe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := newCheckpointTargetTestWorkspace(t)
			resolved, err := ResolveDispositionTarget(ws, tt.filename)
			if tt.wantErr {
				require.Error(t, err)
				if tt.wantIsErr != nil {
					assert.ErrorIs(t, err, tt.wantIsErr)
				}
				assert.Empty(t, resolved)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, resolved)
			assert.True(t, filepath.IsAbs(resolved))
		})
	}
}

// TestResolveDispositionTarget_RejectsSymlink asserts a symlinked checkpoint
// leaf is refused regardless of where the symlink points, since disposition
// verbs must operate on real on-disk bytes.
func TestResolveDispositionTarget_RejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevated privileges on Windows CI runners")
	}
	ws := newCheckpointTargetTestWorkspace(t)
	checkpointDir := filepath.Join(ws.RootPath, ".backlogit", checkpointsSubdir)

	// Create a real file outside the checkpoints dir, then a symlink to it
	// inside the checkpoints dir.
	outside := filepath.Join(t.TempDir(), "real-target.json")
	require.NoError(t, os.WriteFile(outside, []byte(`{}`), 0o644))
	linkPath := filepath.Join(checkpointDir, "checkpoint-link.json")
	require.NoError(t, os.Symlink(outside, linkPath))

	_, err := ResolveDispositionTarget(ws, "checkpoint-link.json")
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrCheckpointTargetUnsafe)
}
