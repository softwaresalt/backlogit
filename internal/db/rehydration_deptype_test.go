package db_test

// rehydration_deptype_test.go — U1 red characterization for F4.
//
// These tests verify that dep_type is durable across a Rehydrate cycle:
// a relates_to or parent_of edge written in frontmatter must survive sync_index.
//
// RED (failing) at HEAD: upsertDependencyTx hardcodes dep_type='blocks' and
// ArtifactFromFrontmatter reads dependencies as bare []string, discarding type.
//
// GREEN after U2–U5 are implemented.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

// TestRehydrate_DependencyTypePreserved_RelatesToSurvivesSync is the primary
// red characterization: a typed dep entry with type=relates_to must survive
// Rehydrate. FAILS at HEAD because the type is hardcoded to blocks.
func TestRehydrate_DependencyTypePreserved_RelatesToSurvivesSync(t *testing.T) {
	ws := t.TempDir()
	queueDir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	feat := "---\nid: FEAT-001\ntitle: Feature\nstatus: queued\nartifact_type: feature\n---\n"
	task1 := "---\nid: TASK-001\ntitle: Task one\nstatus: queued\nartifact_type: task\nparent_id: FEAT-001\n---\n"
	// typed dep entry (new format) — parsed as bare string at HEAD, type discarded
	task2 := "---\nid: TASK-002\ntitle: Task two\nstatus: queued\nartifact_type: task\nparent_id: FEAT-001\ndependencies:\n  - id: TASK-001\n    type: relates_to\n---\n"

	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "FEAT-001.md"), []byte(feat), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "TASK-001.md"), []byte(task1), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "TASK-002.md"), []byte(task2), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	edges, err := db.GetDependencies(ctx, database, "TASK-002")
	require.NoError(t, err)
	require.Len(t, edges, 1, "TASK-002 must have exactly one dependency edge after rehydrate")

	// RED at HEAD: hardcoded 'blocks' is written; after U2–U5 this becomes 'relates_to'.
	assert.Equal(t, "relates_to", edges[0].DepType,
		"dep_type=relates_to must survive a Rehydrate cycle (RED at HEAD: hardcoded to blocks)")
}

// TestRehydrate_DependencyTypePreserved_ParentOfSurvivesSync verifies that
// parent_of dep_type also survives rehydrate.
func TestRehydrate_DependencyTypePreserved_ParentOfSurvivesSync(t *testing.T) {
	ws := t.TempDir()
	queueDir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	feat := "---\nid: FEAT-002\ntitle: Feature\nstatus: queued\nartifact_type: feature\n---\n"
	task := "---\nid: TASK-003\ntitle: Task\nstatus: queued\nartifact_type: task\nparent_id: FEAT-002\ndependencies:\n  - id: FEAT-002\n    type: parent_of\n---\n"

	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "FEAT-002.md"), []byte(feat), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "TASK-003.md"), []byte(task), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	edges, err := db.GetDependencies(ctx, database, "TASK-003")
	require.NoError(t, err)
	require.Len(t, edges, 1)

	assert.Equal(t, "parent_of", edges[0].DepType,
		"dep_type=parent_of must survive a Rehydrate cycle (RED at HEAD: hardcoded to blocks)")
}

// TestRehydrate_DependencyTypePreserved_BareStringDefaultsToBlocks verifies
// that a bare-string dep entry (legacy/default format) loads with dep_type=blocks
// after rehydrate. This scenario must remain GREEN before and after the fix.
func TestRehydrate_DependencyTypePreserved_BareStringDefaultsToBlocks(t *testing.T) {
	ws := t.TempDir()
	queueDir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	src := "---\nid: SRC-001\ntitle: Source\nstatus: queued\nartifact_type: task\ndependencies:\n  - TGT-001\n---\n"
	tgt := "---\nid: TGT-001\ntitle: Target\nstatus: queued\nartifact_type: task\n---\n"

	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "SRC-001.md"), []byte(src), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "TGT-001.md"), []byte(tgt), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	edges, err := db.GetDependencies(ctx, database, "SRC-001")
	require.NoError(t, err)
	require.Len(t, edges, 1, "bare dep entry must produce exactly one edge")
	assert.Equal(t, "blocks", edges[0].DepType, "bare dep entry must default to blocks")
}

// TestRehydrate_DependencyTypePreserved_GoldenSerializationNoKeyDropped captures
// a golden fixture: a round-trip (load artifact → write frontmatter) must not
// drop any key. This guards U3's explicit-key enumeration invariant.
func TestRehydrate_DependencyTypePreserved_GoldenSerializationNoKeyDropped(t *testing.T) {
	ws := t.TempDir()
	queueDir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	// Golden fixture: an artifact with all commonly populated optional fields.
	golden := "---\nid: GOLD-001\ntitle: Golden fixture\nstatus: active\nartifact_type: task\nparent_id: FEAT-999\npriority: high\nassigned_to: agent\nlabels:\n  - reliability\n  - tests\ndependencies:\n  - DEP-001\nreferences:\n  - docs/exec-plans/plan.md\ncommit: abc123\n---\n\nBody text.\n"

	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "GOLD-001.md"), []byte(golden), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	item, err := db.GetItem(ctx, database, "GOLD-001")
	require.NoError(t, err)

	// Verify all fields survived the load (no key dropped by ArtifactFromFrontmatter).
	assert.Equal(t, "GOLD-001", item.ID)
	assert.Equal(t, "Golden fixture", item.Title)
	assert.Equal(t, "FEAT-999", item.ParentID)
	assert.Equal(t, "high", item.Priority)
	assert.Equal(t, "agent", item.AssignedTo)
	assert.Contains(t, item.Labels, "reliability")
}
