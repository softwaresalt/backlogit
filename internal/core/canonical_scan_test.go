package core

// 066.001-T (Unit 1) harness: the shared recursive canonical-ID scanner.
// RED until scanCanonicalArtifacts is implemented in canonical_scan.go.

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/config"
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

// TestScanCanonicalArtifacts_IncludesArchiveWhenRegistryOmitsIt guards the
// 066-F core invariant in the degraded-config case: when ws.Config is non-nil
// and a registry.yaml loads successfully but does NOT route the archive
// directory (a custom/misconfigured registry), artifactSearchDirs surfaces only
// the registry-listed dirs plus the queue root. Because ArchiveItem always
// relocates into the fixed .backlogit/archive directory, the canonical scan
// must still include the archive so a queued + an archived item sharing a root
// ID is detected, rather than silently missed and later overwritten at archive
// time.
func TestScanCanonicalArtifacts_IncludesArchiveWhenRegistryOmitsIt(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	archiveDir := filepath.Join(wsRoot, "archive")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	helperWriteArtifact(t, queueDir, "003-F.md", `---
id: "003-F"
title: "Queued feature"
status: active
artifact_type: feature
level: 1
---
Body.
`)
	helperWriteArtifact(t, archiveDir, "003-F.md", `---
id: "003-F"
title: "Archived feature"
status: archived
artifact_type: feature
level: 1
---
Body.
`)

	// A valid registry that routes only a non-archive directory. LoadRegistry
	// succeeds, so artifactSearchDirs adds "review" + the queue root but NOT the
	// archive. The scanner must force the fixed archive dir into the scan set
	// regardless, or the collision guard goes blind to already-archived IDs.
	registry := "directories:\n  - path: review\n    condition:\n      status:\n        - in-review\n"
	require.NoError(t, os.WriteFile(filepath.Join(wsRoot, "registry.yaml"), []byte(registry), 0o644))
	ws := &Workspace{RootPath: tmp, Config: config.DefaultConfig()}

	refs, err := scanCanonicalArtifacts(ws)
	require.NoError(t, err)
	require.Len(t, refs["003-F"], 2,
		"archived 003-F must be discovered even when registry.yaml does not route the archive dir, or the collision guard goes blind to the archive")
}
