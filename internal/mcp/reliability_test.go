package mcp

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
)

func TestEnsureWorkspace_ConcurrentCallsShareWorkspace(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	s := NewServerForRoot(root)
	ctx := context.Background()

	results := make([]*core.Workspace, 8)
	errs := make([]error, 8)

	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = s.ensureWorkspace(ctx)
		}(i)
	}
	wg.Wait()
	t.Cleanup(func() {
		if s.Workspace != nil {
			s.Workspace.Close()
		}
	})

	for _, err := range errs {
		require.NoError(t, err)
	}
	for i := 1; i < len(results); i++ {
		assert.Same(t, results[0], results[i])
	}
}

func TestHandleGetItem_NotFoundReturnsNotFound(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	result, err := s.handleGetItem(ctx, shipmentRequest(map[string]any{"id": "missing-item"}))

	require.NoError(t, err)
	assert.Equal(t, "not_found", shipmentErrorField(t, result))
}

func TestHandleMoveItem_RelocatesFileByStatus(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Feature to archive", "feature")
	require.NoError(t, err)

	// Transition to active first (queued→done is not a valid transition).
	_, err = core.UpdateArtifact(ctx, ws, feature.ID, map[string]any{"status": "active"})
	require.NoError(t, err)

	result, err := s.handleMoveItem(ctx, shipmentRequest(map[string]any{
		"id":     feature.ID,
		"status": "done",
	}))

	require.NoError(t, err)
	assert.False(t, result.IsError)

	filePath, err := core.FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, err)
	assert.Contains(t, filepath.ToSlash(filePath), "/.backlogit/archive/")
}

func TestHandleDeleteItem_CascadesRelatedRows(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Source feature", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Target feature", "feature")
	require.NoError(t, err)

	_, err = s.handleAddLink(ctx, linkRequest(map[string]any{
		"source_id": source.ID,
		"target_id": target.ID,
		"link_type": "related_to",
	}))
	require.NoError(t, err)

	_, err = ws.DB.ExecContext(ctx, `INSERT INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, 'blocks')`, target.ID, source.ID)
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)`, source.ID, "abc123", "msg", "agent")
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO stash_links (stash_id, item_id, linked_at) VALUES (?, ?, ?)`, "STASH-1", source.ID, time.Now().Format(time.RFC3339Nano))
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO item_logs (item_id, log_path, updated_at) VALUES (?, ?, ?)`, source.ID, "logs/"+source.ID+".jsonl", time.Now().Format(time.RFC3339Nano))
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO item_log_entries (item_id, log_path, timestamp, actor, event_type, content, delta_json) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		source.ID, "logs/"+source.ID+".jsonl", time.Now().Format(time.RFC3339Nano), "agent", "comment", "content", "{}")
	require.NoError(t, err)

	request := mcplib.CallToolRequest{}
	request.Params.Arguments = map[string]any{"id": source.ID}
	result, err := s.handleDeleteItem(ctx, request)
	require.NoError(t, err)
	assert.False(t, result.IsError)

	for name, query := range map[string]string{
		"commit_links":     `SELECT COUNT(*) FROM commit_links WHERE item_id = ?`,
		"stash_links":      `SELECT COUNT(*) FROM stash_links WHERE item_id = ?`,
		"item_links":       `SELECT COUNT(*) FROM item_links WHERE source_id = ? OR target_id = ?`,
		"item_deps":        `SELECT COUNT(*) FROM item_deps WHERE item_id = ? OR depends_on = ?`,
		"item_logs":        `SELECT COUNT(*) FROM item_logs WHERE item_id = ?`,
		"item_log_entries": `SELECT COUNT(*) FROM item_log_entries WHERE item_id = ?`,
	} {
		var count int
		switch name {
		case "item_links", "item_deps":
			require.NoError(t, ws.DB.QueryRowContext(ctx, query, source.ID, source.ID).Scan(&count))
		default:
			require.NoError(t, ws.DB.QueryRowContext(ctx, query, source.ID).Scan(&count))
		}
		assert.Zero(t, count, name)
	}
}
