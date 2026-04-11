package mcp

// 026.013-T: ensureWorkspace mutex/double-check tests.
//
// These tests verify three invariants of the ensureWorkspace implementation:
//  1. A first initialisation failure does NOT prevent a successful second attempt.
//  2. A successful initialisation is cached — the second call returns the same
//     *core.Workspace without re-running NewWorkspace.
//  3. Concurrent calls are safe under the race detector.

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
)

// TestEnsureWorkspace_RetryOnFailure verifies that when the first call fails
// (no .backlogit directory), a second call after the directory is created
// succeeds — proving that failures are never cached.
func TestEnsureWorkspace_RetryOnFailure(t *testing.T) {
	root := t.TempDir()
	s := NewServerForRoot(root)

	ctx := context.Background()

	// First call: .backlogit does not exist → should return ErrNotExist.
	_, firstErr := s.ensureWorkspace(ctx)
	require.ErrorIs(t, firstErr, os.ErrNotExist,
		"call before .backlogit exists must return ErrNotExist")

	// Create the .backlogit directory and write config defaults.
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	// Second call: directory now exists → must succeed.
	ws, secondErr := s.ensureWorkspace(ctx)
	require.NoError(t, secondErr,
		"call after .backlogit is created must succeed")
	require.NotNil(t, ws)
	t.Cleanup(func() { ws.Close() })
}

// TestEnsureWorkspace_CachesSuccess verifies that two consecutive successful
// calls return the exact same *core.Workspace pointer (identity, not just
// equality), proving that successful initialisation is cached.
func TestEnsureWorkspace_CachesSuccess(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	s := NewServerForRoot(root)
	ctx := context.Background()

	ws1, err := s.ensureWorkspace(ctx)
	require.NoError(t, err)
	require.NotNil(t, ws1)
	t.Cleanup(func() { ws1.Close() })

	ws2, err := s.ensureWorkspace(ctx)
	require.NoError(t, err)

	assert.Same(t, ws1, ws2,
		"second ensureWorkspace call must return the cached workspace pointer")
}

// TestEnsureWorkspace_ConcurrentSafe spawns N goroutines that call
// ensureWorkspace simultaneously and verifies they all receive the same
// workspace pointer. Run with -race to detect data races.
func TestEnsureWorkspace_ConcurrentSafe(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	// Pre-initialise to ensure NewWorkspace is only called once even under
	// concurrency pressure.  We create the server without a workspace.
	s := NewServerForRoot(root)
	ctx := context.Background()

	const workers = 20
	results := make([]*core.Workspace, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := range workers {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = s.ensureWorkspace(ctx)
		}(i)
	}
	wg.Wait()

	// Cleanup the shared workspace once.
	if results[0] != nil {
		t.Cleanup(func() { results[0].Close() })
	}

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d returned an error", i)
	}
	// All goroutines must have received the same pointer.
	for i := 1; i < workers; i++ {
		assert.Same(t, results[0], results[i],
			"goroutine %d received a different workspace pointer", i)
	}
}
