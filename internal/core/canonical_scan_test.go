package core

// 066.001-T (Unit 1) harness: the shared recursive canonical-ID scanner.
// RED until scanCanonicalArtifacts is implemented in canonical_scan.go.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanCanonicalArtifacts_ReturnsParsedRefsAcrossQueueAndArchive(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	archiveDir := filepath.Join(wsRoot, "archive")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	helperWriteArtifact(t, queueDir, "001-F.md", `---
id: "001-F"
title: "Queued feature"
status: active
artifact_type: feature
level: 1
---
Body.
`)
	helperWriteArtifact(t, archiveDir, "001-F.md", `---
id: "001-F"
title: "Archived feature"
status: archived
artifact_type: feature
level: 1
---
Body.
`)
	helperWriteArtifact(t, queueDir, "002-F.md", `---
id: "002-F"
title: "Standalone feature"
status: active
artifact_type: feature
level: 1
---
Body.
`)

	// Nil-config workspace => artifactSearchDirs scans every non-hidden dir under
	// .backlogit (queue + archive), exercising the recursive multi-dir walk.
	ws := &Workspace{RootPath: tmp}

	refs, err := scanCanonicalArtifacts(ws)
	require.NoError(t, err)
	require.NotNil(t, refs)

	require.Len(t, refs["001-F"], 2, "001-F must be found in both queue and archive")
	require.Len(t, refs["002-F"], 1, "002-F must be found exactly once")

	// Parsed fields must be populated so consumers do not re-parse.
	for _, r := range refs["001-F"] {
		assert.Equal(t, "001-F", r.id)
		assert.Equal(t, "feature", r.artifactType)
		assert.Equal(t, 1, r.level)
		assert.NotEmpty(t, r.path)
		assert.NotEmpty(t, r.status)
	}
}

func TestScanCanonicalArtifacts_RecursesNestedHierarchyDirs(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	nested := filepath.Join(wsRoot, "queue", "001", "001.001")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	helperWriteArtifact(t, nested, "001.001-T.md", `---
id: "001.001-T"
title: "Nested task"
status: active
artifact_type: task
parent_id: "001-F"
level: 2
---
Body.
`)

	ws := &Workspace{RootPath: tmp}
	refs, err := scanCanonicalArtifacts(ws)
	require.NoError(t, err)
	require.Len(t, refs["001.001-T"], 1, "scanner must recurse into nested hierarchy directories")
	assert.Equal(t, "001-F", refs["001.001-T"][0].parentID)
}
