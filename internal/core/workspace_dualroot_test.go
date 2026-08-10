package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const workspaceDirEnvVar = "BACKLOGIT_WORKSPACE_DIR"

func TestWorkspaceRootCandidates_ReturnsFreshCopy(t *testing.T) {
	got := WorkspaceRootCandidates()
	require.Equal(t, []string{".backlog", ".backlogit"}, got)

	got[0] = "mutated"
	assert.Equal(t, []string{".backlog", ".backlogit"}, WorkspaceRootCandidates())
}

func TestResolveStorageRoot_Matrix(t *testing.T) {
	t.Parallel()

	backlog := ".backlog"
	backlogit := ".backlogit"
	empty := ""

	testCases := []struct {
		name     string
		override *string
		setup    func(t *testing.T, root string)
		assert   func(t *testing.T, root string, storageRoot string, err error)
	}{
		{
			name:     "backlog only present resolves backlog",
			override: nil,
			setup: func(t *testing.T, root string) {
				writeResolverCandidate(t, root, ".backlog")
			},
			assert: func(t *testing.T, root string, storageRoot string, err error) {
				require.NoError(t, err)
				assert.Equal(t, filepath.Join(root, ".backlog"), storageRoot)
			},
		},
		{
			name:     "backlogit only present resolves backlogit",
			override: nil,
			setup: func(t *testing.T, root string) {
				writeResolverCandidate(t, root, ".backlogit")
			},
			assert: func(t *testing.T, root string, storageRoot string, err error) {
				require.NoError(t, err)
				assert.Equal(t, filepath.Join(root, ".backlogit"), storageRoot)
			},
		},
		{
			name:     "both present without override is ambiguous",
			override: nil,
			setup: func(t *testing.T, root string) {
				writeResolverCandidate(t, root, ".backlog")
				writeResolverCandidate(t, root, ".backlogit")
			},
			assert: func(t *testing.T, _ string, _ string, err error) {
				require.Error(t, err)
				var ambiguous *blerrors.AmbiguousWorkspaceRootError
				require.ErrorAs(t, err, &ambiguous)
				assert.Equal(t, []string{".backlog", ".backlogit"}, ambiguous.Roots)
				assert.ErrorIs(t, err, blerrors.ErrAmbiguousWorkspaceRoot)
			},
		},
		{
			name:     "neither present errors",
			override: nil,
			setup:    func(t *testing.T, root string) {},
			assert: func(t *testing.T, _ string, _ string, err error) {
				require.Error(t, err)
			},
		},
		{
			name:     "override backlog wins",
			override: &backlog,
			setup: func(t *testing.T, root string) {
				writeResolverCandidate(t, root, ".backlog")
				writeResolverCandidate(t, root, ".backlogit")
			},
			assert: func(t *testing.T, root string, storageRoot string, err error) {
				require.NoError(t, err)
				assert.Equal(t, filepath.Join(root, ".backlog"), storageRoot)
			},
		},
		{
			name:     "override backlogit wins",
			override: &backlogit,
			setup: func(t *testing.T, root string) {
				writeResolverCandidate(t, root, ".backlog")
				writeResolverCandidate(t, root, ".backlogit")
			},
			assert: func(t *testing.T, root string, storageRoot string, err error) {
				require.NoError(t, err)
				assert.Equal(t, filepath.Join(root, ".backlogit"), storageRoot)
			},
		},
		{
			name:     "override empty string is misconfigured",
			override: &empty,
			setup: func(t *testing.T, root string) {
				writeResolverCandidate(t, root, ".backlog")
			},
			assert: func(t *testing.T, _ string, _ string, err error) {
				require.Error(t, err)
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			setWorkspaceDirOverride(t, tc.override)

			storageRoot, err := resolveStorageRoot(root)
			tc.assert(t, root, storageRoot, err)
		})
	}
}

func TestValidateWorkspaceDirOverride_RejectsInvalidValues(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		override string
	}{
		{name: "forward slash", override: ".backlog/child"},
		{name: "backslash", override: ".backlog\\child"},
		{name: "dot", override: "."},
		{name: "dotdot", override: ".."},
		{name: "absolute path", override: filepath.Join(string(filepath.Separator), "tmp", ".backlog")},
		{name: "volume qualified drive relative", override: "C:foo"},
		{name: "unc", override: `\\server\share\.backlog`},
		{name: "nul byte", override: ".backlog\x00"},
		{name: "windows case alias", override: ".BACKLOG"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateWorkspaceDirOverride(tc.override)
			require.Error(t, err)
		})
	}
}

func TestResolveStorageRoot_RejectsCandidateReparsePoint(t *testing.T) {
	root := t.TempDir()
	realCandidate := filepath.Join(t.TempDir(), "real-backlog")
	require.NoError(t, os.MkdirAll(realCandidate, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(realCandidate, "config.yaml"), []byte("version: 1\n"), 0o644))

	linkPath := filepath.Join(root, ".backlog")
	if err := os.Symlink(realCandidate, linkPath); err != nil {
		t.Skipf("symlinks not creatable in this environment: %v", err)
	}

	_, err := resolveStorageRoot(root)
	require.Error(t, err)
}

func TestResolveStorageRoot_FailsClosedOnUnreadableConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows cannot reliably create an unreadable config.yaml with os.Chmod alone")
	}

	root := t.TempDir()
	candidate := writeResolverCandidate(t, root, ".backlog")
	configPath := filepath.Join(candidate, "config.yaml")
	require.NoError(t, os.Chmod(configPath, 0))
	t.Cleanup(func() {
		_ = os.Chmod(configPath, 0o644)
	})

	if f, err := os.Open(configPath); err == nil {
		_ = f.Close()
		t.Skip("environment still permits reading mode 000 files")
	}

	_, err := resolveStorageRoot(root)
	require.Error(t, err)
}

func TestNewWorkspace_StoresResolvedStorageRoot(t *testing.T) {
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlog")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ws.Close()
	})

	assert.Equal(t, root, ws.RootPath)
	assert.Equal(t, storageRoot, ws.StorageRoot)
}

func TestResolveStorageRoot_OverrideMissingCandidateErrors(t *testing.T) {
	root := t.TempDir()
	override := ".backlog"
	setWorkspaceDirOverride(t, &override)

	_, err := resolveStorageRoot(root)
	require.Error(t, err)
	assert.False(t, errors.Is(err, blerrors.ErrAmbiguousWorkspaceRoot))
}

func writeResolverCandidate(t *testing.T, root, dirName string) string {
	t.Helper()

	candidate := filepath.Join(root, dirName)
	require.NoError(t, os.MkdirAll(candidate, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(candidate, "config.yaml"), []byte("version: 1\n"), 0o644))
	return candidate
}

func setWorkspaceDirOverride(t *testing.T, value *string) {
	t.Helper()

	oldValue, hadOldValue := os.LookupEnv(workspaceDirEnvVar)
	if value == nil {
		require.NoError(t, os.Unsetenv(workspaceDirEnvVar))
	} else {
		require.NoError(t, os.Setenv(workspaceDirEnvVar, *value))
	}
	t.Cleanup(func() {
		if hadOldValue {
			_ = os.Setenv(workspaceDirEnvVar, oldValue)
			return
		}
		_ = os.Unsetenv(workspaceDirEnvVar)
	})
}
