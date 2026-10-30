package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

const (
	u2LockName    = "mcp-daemon.lock"
	u2SidecarName = "mcp-daemon.json"
)

func u2Workspace(t *testing.T) (root, storageRoot, runtimeDir string) {
	t.Helper()
	root = t.TempDir()
	storageRoot = filepath.Join(root, core.WorkspaceRootCandidates()[0])
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	return root, storageRoot, filepath.Join(storageRoot, "runtime")
}

func u2ReadSidecar(t *testing.T, runtimeDir string) MCPDaemonInfo {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(runtimeDir, u2SidecarName))
	require.NoError(t, err)
	var info MCPDaemonInfo
	require.NoError(t, json.Unmarshal(data, &info))
	return info
}

func u2WriteStaleSidecar(t *testing.T, runtimeDir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(runtimeDir, 0o755))
	stale, err := json.Marshal(MCPDaemonInfo{PID: 999999, ProcessStartTime: 1, ExecutablePath: "stale"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(runtimeDir, u2SidecarName), stale, 0o600))
}

func TestU2_AcquireLockAndContention(t *testing.T) {
	root, storageRoot, runtimeDir := u2Workspace(t)

	release, err := AcquireMCPDaemonLock(t.Context(), root, false)
	require.NoError(t, err)
	require.NotNil(t, release)

	info := u2ReadSidecar(t, runtimeDir)
	assert.Equal(t, os.Getpid(), info.PID)
	assert.NotZero(t, info.ProcessStartTime)
	assert.NotEmpty(t, info.ExecutablePath)
	assert.False(t, info.StartedAt.IsZero())
	assert.Equal(t, root, info.WorkspaceRoot)
	assert.Equal(t, storageRoot, info.StorageRoot)
	assert.NotEmpty(t, info.Version)
	assert.Equal(t, filepath.Join(runtimeDir, u2LockName), info.LockPath)

	status, err := InspectMCPDaemonStatus(root)
	require.NoError(t, err)
	assert.True(t, status.Running)
	assert.Equal(t, os.Getpid(), status.PID)
	assert.Equal(t, root, status.WorkspaceRoot)
	assert.Equal(t, storageRoot, status.StorageRoot)
	assert.Equal(t, info.Version, status.Version)

	_, err = AcquireMCPDaemonLock(t.Context(), root, false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, corerrors.ErrDaemonAlreadyRunning), "got %v", err)
	assert.Contains(t, err.Error(), strconv.Itoa(os.Getpid()))

	require.NoError(t, release())
	require.NoError(t, release(), "release must be idempotent")
	_, statErr := os.Stat(filepath.Join(runtimeDir, u2SidecarName))
	assert.True(t, os.IsNotExist(statErr), "sidecar must be removed on release")

	status, err = InspectMCPDaemonStatus(root)
	require.NoError(t, err)
	assert.False(t, status.Running)

	release2, err := AcquireMCPDaemonLock(t.Context(), root, false)
	require.NoError(t, err)
	require.NoError(t, release2())
}

func TestU2_CrashRecoveryCleansStaleMetadata(t *testing.T) {
	root, _, runtimeDir := u2Workspace(t)

	u2WriteStaleSidecar(t, runtimeDir)
	status, err := InspectMCPDaemonStatus(root)
	require.NoError(t, err)
	assert.False(t, status.Running)
	_, statErr := os.Stat(filepath.Join(runtimeDir, u2SidecarName))
	assert.True(t, os.IsNotExist(statErr), "stale sidecar must be removed by status probe")

	u2WriteStaleSidecar(t, runtimeDir)
	release, err := AcquireMCPDaemonLock(t.Context(), root, false)
	require.NoError(t, err)
	info := u2ReadSidecar(t, runtimeDir)
	assert.Equal(t, os.Getpid(), info.PID)
	assert.NotEqual(t, "stale", info.ExecutablePath)
	assert.NotZero(t, info.ProcessStartTime)
	require.NoError(t, release())
}

func TestU2_RejectsSymlinkOrReparseRuntimeDir(t *testing.T) {
	setup := func(t *testing.T) (root, storageRoot, runtimeDir, external string) {
		root, storageRoot, runtimeDir = u2Workspace(t)
		external = t.TempDir()
		return root, storageRoot, runtimeDir, external
	}
	link := func(t *testing.T, target, name string) {
		t.Helper()
		if err := os.Symlink(target, name); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
	}

	t.Run("runtime dir symlink", func(t *testing.T) {
		root, _, runtimeDir, external := setup(t)
		link(t, external, runtimeDir)
		_, err := AcquireMCPDaemonLock(t.Context(), root, false)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "acquire: %v", err)
		_, err = InspectMCPDaemonStatus(root)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "inspect: %v", err)
		entries, _ := os.ReadDir(external)
		assert.Empty(t, entries)
	})

	t.Run("lock leaf symlink", func(t *testing.T) {
		root, _, runtimeDir, external := setup(t)
		require.NoError(t, os.MkdirAll(runtimeDir, 0o755))
		target := filepath.Join(external, "target.lock")
		link(t, target, filepath.Join(runtimeDir, u2LockName))
		_, err := AcquireMCPDaemonLock(t.Context(), root, false)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "acquire: %v", err)
		_, err = InspectMCPDaemonStatus(root)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "inspect: %v", err)
		_, statErr := os.Lstat(target)
		assert.True(t, os.IsNotExist(statErr), "external target must not be created")
	})

	t.Run("sidecar leaf swapped for symlink while held", func(t *testing.T) {
		root, _, runtimeDir, external := setup(t)
		release, err := AcquireMCPDaemonLock(t.Context(), root, false)
		require.NoError(t, err)
		defer func() { _ = release() }()

		secret := filepath.Join(external, "secret.json")
		require.NoError(t, os.WriteFile(secret, []byte(`{"pid":4242}`), 0o600))
		require.NoError(t, os.Remove(filepath.Join(runtimeDir, u2SidecarName)))
		link(t, secret, filepath.Join(runtimeDir, u2SidecarName))

		_, err = InspectMCPDaemonStatus(root)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "inspect: %v", err)
		_, err = AcquireMCPDaemonLock(t.Context(), root, false)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "acquire: %v", err)
		data, readErr := os.ReadFile(secret)
		require.NoError(t, readErr)
		assert.JSONEq(t, `{"pid":4242}`, string(data))
	})

	t.Run("storage root symlink", func(t *testing.T) {
		root := t.TempDir()
		external := t.TempDir()
		link(t, external, filepath.Join(root, core.WorkspaceRootCandidates()[0]))
		_, err := AcquireMCPDaemonLock(t.Context(), root, false)
		assert.True(t, errors.Is(err, corerrors.ErrValidation), "acquire: %v", err)
		entries, _ := os.ReadDir(external)
		assert.Empty(t, entries)
	})
}
