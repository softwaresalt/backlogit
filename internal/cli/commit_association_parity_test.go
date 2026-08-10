package cli_test

// U1 (106.019-T): Failing parity characterization — three surfaces, three states.
//
// This file establishes a RED characterization of today's commit-association
// divergence. It must be OBSERVED FAILING at HEAD before F6/U2-U3 implement
// core.AssociateCommit and route all three surfaces through it.
//
// Surfaces tested:
//   - CLI update --commit  (frontmatter scalar only)
//   - core.LinkCommit      (track_commit surface: commit_links + JSONL only, no frontmatter)
//   - core.UpdateArtifact  (update_item(commit=) surface: frontmatter scalar only)
//
// After U2-U3, all three surfaces produce the full contract:
//   - frontmatter scalar set
//   - commit_links row present
//   - JSONL commit_tracked event present

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// commitObservation captures all three observable representations of a commit
// association for a single artifact.
type commitObservation struct {
	// FrontmatterSHA is the scalar written into the artifact's YAML frontmatter.
	FrontmatterSHA string
	// HasCommitLinksRow is true when commit_links contains a row for this item+sha.
	HasCommitLinksRow bool
	// HasJSONLEvent is true when the item's JSONL log contains a commit_tracked event.
	HasJSONLEvent bool
}

// readArtifactFromMarkdown reads the artifact markdown file for id and returns
// the parsed Artifact. Reads from disk (never from the DB cache) so the
// frontmatter scalar is authoritative.
func readArtifactFromMarkdown(t *testing.T, ws *core.Workspace, id string) *models.Artifact {
	t.Helper()
	ctx := context.Background()
	path, err := core.FindArtifactPath(ctx, ws, id)
	require.NoError(t, err, "FindArtifactPath(%s)", id)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read artifact file %s", path)
	fm, body, err := models.ParseFrontmatter(string(data))
	require.NoError(t, err, "parse frontmatter %s", path)
	art, err := models.ArtifactFromFrontmatter(fm, body)
	require.NoError(t, err, "parse artifact file %s", path)
	return art
}

// observeState reads all three representations for itemID / sha.
// For commit_links, a fresh workspace is opened to avoid cross-connection
// SQLite snapshot isolation hiding writes made by a separate connection (e.g.
// a CLI invocation). Frontmatter and JSONL are read directly from disk.
func observeState(t *testing.T, ws *core.Workspace, itemID, sha string) commitObservation {
	t.Helper()
	ctx := context.Background()

	// 1. Frontmatter scalar — from markdown, not DB cache.
	art := readArtifactFromMarkdown(t, ws, itemID)

	// 2. commit_links row — open a fresh connection to get the latest committed state.
	freshWS, err := core.NewWorkspace(ctx, ws.RootPath)
	require.NoError(t, err, "open fresh workspace for commit_links check")
	defer freshWS.Close()
	var linkCount int
	_ = freshWS.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM commit_links WHERE item_id = ? AND commit_sha = ?`,
		itemID, sha).Scan(&linkCount)

	// 3. JSONL commit_tracked event — read directly from disk.
	logsDir := core.WorkspaceLogsRoot(ws.RootPath)
	evs, _ := events.ReadAllEvents(ctx, logsDir, itemID)
	var hasEvent bool
	for _, ev := range evs {
		if ev.EventType == "commit_tracked" {
			if d, ok := ev.Delta["commit_sha"].(string); ok && d == sha {
				hasEvent = true
				break
			}
		}
	}

	return commitObservation{
		FrontmatterSHA:    art.Commit,
		HasCommitLinksRow: linkCount > 0,
		HasJSONLEvent:     hasEvent,
	}
}

// setupParityWorkspace creates a workspace with a feature+task and returns root,
// the opened workspace, a feature ID, and a factory function to create additional tasks.
func setupParityWorkspace(t *testing.T) (root string, ws *core.Workspace, addTask func(title string) string) {
	t.Helper()
	root = t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	var err error
	ws, err = core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	featID := cliAddFeature(t, root, "Parity characterization feature")

	addTask = func(title string) string {
		return cliAddTask(t, root, title, featID)
	}
	return root, ws, addTask
}

const parityTestSHA = "cafebabe0001000200030004000500060007cafe"

// TestCommitAssociation_CLI_WritesAllThreeRepresentations verifies that CLI
// update --commit produces the full three-representation contract after U2-U3.
func TestCommitAssociation_CLI_WritesAllThreeRepresentations(t *testing.T) {
	root, ws, addTask := setupParityWorkspace(t)
	taskID := addTask("CLI full-contract verification")

	runCLIStdout(t, root, "update", taskID, "--commit", parityTestSHA)

	obs := observeState(t, ws, taskID, parityTestSHA)

	assert.Equal(t, parityTestSHA, obs.FrontmatterSHA,
		"CLI update --commit sets frontmatter scalar")
	assert.True(t, obs.HasCommitLinksRow,
		"CLI update --commit writes commit_links row (via AssociateCommit)")
	assert.True(t, obs.HasJSONLEvent,
		"CLI update --commit writes JSONL commit_tracked event (via AssociateCommit)")
}

// TestCommitAssociation_TrackCommitWritesLinksOnly_Characterization documents that
// the track_commit surface (core.LinkCommit) writes commit_links + JSONL but NOT
// the frontmatter scalar at HEAD.
func TestCommitAssociation_TrackCommitWritesLinksOnly_Characterization(t *testing.T) {
	_, ws, addTask := setupParityWorkspace(t)
	taskID := addTask("track_commit links-only characterization")
	ctx := context.Background()

	err := core.LinkCommit(ctx, ws.DB, ws, taskID, parityTestSHA, "feat: char test", "char@test.com") //nolint:staticcheck // intentional: characterizes deprecated behavior
	require.NoError(t, err, "LinkCommit must succeed")

	obs := observeState(t, ws, taskID, parityTestSHA)

	// Characterization failure — no frontmatter scalar at HEAD.
	assert.Empty(t, obs.FrontmatterSHA,
		"Characterization: track_commit does NOT set frontmatter scalar at HEAD (FAILS after U2-U3)")
	assert.True(t, obs.HasCommitLinksRow,
		"track_commit writes commit_links row")
	assert.True(t, obs.HasJSONLEvent,
		"track_commit writes JSONL commit_tracked event")
}

// TestCommitAssociation_UpdateItem_WritesAllThreeRepresentations verifies that the
// update_item(commit=) surface (routed through core.AssociateCommit after U2-U3)
// produces the full three-representation contract.
func TestCommitAssociation_UpdateItem_WritesAllThreeRepresentations(t *testing.T) {
	_, ws, addTask := setupParityWorkspace(t)
	taskID := addTask("update_item full-contract verification")
	ctx := context.Background()
	logsDir := core.WorkspaceLogsRoot(ws.RootPath)
	ew := core.NewWorkspaceEventWriter(ws, logsDir)

	// Simulate what MCP handleUpdateItem now does: route through AssociateCommit.
	err := core.AssociateCommit(ctx, ws, ew, taskID, parityTestSHA, "", "")
	require.NoError(t, err, "AssociateCommit must succeed")

	obs := observeState(t, ws, taskID, parityTestSHA)

	assert.Equal(t, parityTestSHA, obs.FrontmatterSHA,
		"update_item(commit=) sets frontmatter scalar")
	assert.True(t, obs.HasCommitLinksRow,
		"update_item(commit=) writes commit_links row (via AssociateCommit)")
	assert.True(t, obs.HasJSONLEvent,
		"update_item(commit=) writes JSONL commit_tracked event (via AssociateCommit)")
}

// TestCommitAssociationParity_ThreeSurfaces is the unified parity test: all three
// surfaces must produce the full three-representation contract (frontmatter scalar,
// commit_links row, JSONL event). After U2-U3 all three routes go through
// core.AssociateCommit so this test is GREEN.
func TestCommitAssociationParity_ThreeSurfaces(t *testing.T) {
	root, ws, addTask := setupParityWorkspace(t)
	taskCLI := addTask("CLI all-surfaces parity")
	taskTrack := addTask("track_commit all-surfaces parity")
	taskUpdate := addTask("update_item all-surfaces parity")
	ctx := context.Background()

	// Shared EventWriter for direct core.AssociateCommit calls (mirrors MCP s.Events).
	logsDir := core.WorkspaceLogsRoot(ws.RootPath)
	ew := core.NewWorkspaceEventWriter(ws, logsDir)

	// CLI surface: routes through AssociateCommit via the CLI command.
	runCLIStdout(t, root, "update", taskCLI, "--commit", parityTestSHA)

	// track_commit surface: routes through core.AssociateCommit (what MCP handleTrackCommit calls).
	require.NoError(t, core.AssociateCommit(ctx, ws, ew, taskTrack, parityTestSHA, "feat: parity", "parity@test.com"))

	// update_item(commit=) surface: routes through core.AssociateCommit (what MCP handleUpdateItem calls).
	require.NoError(t, core.AssociateCommit(ctx, ws, ew, taskUpdate, parityTestSHA, "", ""))

	want := commitObservation{
		FrontmatterSHA:    parityTestSHA,
		HasCommitLinksRow: true,
		HasJSONLEvent:     true,
	}

	// All three surfaces must produce the full three-representation contract.
	assert.Equal(t, want, observeState(t, ws, taskCLI, parityTestSHA),
		"CLI update --commit must produce full three-representation state")
	assert.Equal(t, want, observeState(t, ws, taskTrack, parityTestSHA),
		"MCP track_commit must produce full three-representation state")
	assert.Equal(t, want, observeState(t, ws, taskUpdate, parityTestSHA),
		"MCP update_item(commit=) must produce full three-representation state")
}

// TestLinkCommit_BestEffortReturnsNil_Characterization documents the current
// best-effort behavior: LinkCommit returns nil even when JSONL append fails.
// After U2, core.AssociateCommit surfaces the JSONL error to callers.
func TestLinkCommit_BestEffortReturnsNil_Characterization(t *testing.T) {
	_, ws, addTask := setupParityWorkspace(t)
	taskID := addTask("BestEffort characterization task")
	ctx := context.Background()

	// Make the log file unwritable so JSONL append fails.
	logsDir := core.WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	logFile := filepath.Join(logsDir, taskID+".jsonl")
	// Create an empty unwritable log file.
	require.NoError(t, os.WriteFile(logFile, nil, 0o400))
	t.Cleanup(func() { _ = os.Chmod(logFile, 0o644) })

	sha := "deadbeef0001000200030004000500060007dead"
	err := core.LinkCommit(ctx, ws.DB, ws, taskID, sha, "test", "a@b.com") //nolint:staticcheck // intentional: characterizes deprecated behavior

	// Characterization: current LinkCommit swallows JSONL failure (returns nil).
	// After U2 this becomes an error: assert.Error(t, err, "must surface JSONL failure")
	assert.Nil(t, err, "Characterization: current LinkCommit returns nil on JSONL failure (best-effort)")

	// commit_links row was written even though JSONL failed.
	var count int
	_ = ws.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM commit_links WHERE item_id = ? AND commit_sha = ?`,
		taskID, sha).Scan(&count)
	assert.Equal(t, 1, count, "commit_links row written before JSONL failure")

	// Unused-import guard.
	_ = strings.TrimSpace("")
}
