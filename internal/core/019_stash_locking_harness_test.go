package core_test

// 019.007-T: Stash harvest advisory file locking.

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestAddStashEntry_ConcurrentCallsProduceDistinctEntries(t *testing.T) {
	// 019.007-T: Concurrent AddStashEntry calls must each produce a distinct, valid
	// entry. Without file locking, concurrent writers can corrupt the JSONL file
	// or produce duplicate/missing entries. This test MUST be run with -race.
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	const concurrent = 5
	type result struct {
		id  string
		err error
	}
	results := make([]result, concurrent)

	var wg sync.WaitGroup
	for i := 0; i < concurrent; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry, err := core.AddStashEntry(ctx, ws, "task", "medium", "Concurrent stash idea")
			id := ""
			if entry != nil {
				id = entry.ID
			}
			results[i] = result{id: id, err: err}
		}()
	}
	wg.Wait()

	// All calls must succeed
	for _, r := range results {
		require.NoError(t, r.err, "concurrent AddStashEntry must not return an error")
	}

	// All IDs must be distinct
	seen := make(map[string]bool)
	for _, r := range results {
		assert.False(t, seen[r.id], "duplicate stash ID %s detected under concurrent access", r.id)
		seen[r.id] = true
	}

	// The stash must contain exactly 'concurrent' entries
	fetched, err := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, err)
	assert.Len(t, fetched.Entries, concurrent,
		"stash must contain exactly %d entries after %d concurrent adds", concurrent, concurrent)
}
