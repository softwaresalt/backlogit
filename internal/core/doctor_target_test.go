package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// newTargetTestWorkspace creates a fully-initialized workspace (config,
// header-def, DB) rooted at a temp dir and returns it plus the queue dir.
func newTargetTestWorkspace(t *testing.T) (*Workspace, string) {
	t.Helper()
	tmp := t.TempDir()
	backlogitDir := filepath.Join(tmp, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "logs"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ws, err := NewWorkspace(context.Background(), tmp)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return ws, filepath.Join(backlogitDir, "queue")
}

const validTaskContent = `---
artifact_type: task
id: 100.001-T
title: A valid task
status: active
priority: medium
parent_id: 100-F
---

Body text that must be preserved.
`

func TestDoctorTarget_ValidTaskPasses(t *testing.T) {
	ws, queueDir := newTargetTestWorkspace(t)
	path := filepath.Join(queueDir, "100.001-T.md")
	require.NoError(t, os.WriteFile(path, []byte(validTaskContent), 0o644))

	res, err := DoctorTarget(ws, path)
	require.NoError(t, err)
	assert.True(t, res.OK, "valid task should pass: %+v", res)
	assert.Equal(t, DoctorTargetPass, res.Kind)
	assert.Equal(t, "100.001-T", res.ArtifactID)
	assert.Equal(t, "task", res.ArtifactType)
	assert.Equal(t, "target", res.Mode)
	assert.Empty(t, res.FieldErrors)
}

func TestDoctorTarget_MissingRequiredFieldFailsValidation(t *testing.T) {
	ws, queueDir := newTargetTestWorkspace(t)
	// Omit priority — a header-def-required task field — but keep the
	// struct-required base fields so the failure is a header-def validation
	// fail (not a decode/build error).
	const missingPriority = `---
artifact_type: task
id: 100.002-T
title: Task missing priority
status: active
parent_id: 100-F
---

Body.
`
	path := filepath.Join(queueDir, "100.002-T.md")
	require.NoError(t, os.WriteFile(path, []byte(missingPriority), 0o644))

	res, err := DoctorTarget(ws, path)
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Equal(t, DoctorTargetValidation, res.Kind)
	assert.Contains(t, res.FieldErrors, "priority", "field-level error should name the missing field: %+v", res)
}

func TestDoctorTarget_OutsideStorageRootRejectedAsScope(t *testing.T) {
	ws, _ := newTargetTestWorkspace(t)

	// A parent-traversal path that escapes .backlogit.
	res, err := DoctorTarget(ws, filepath.Join("..", "escape.md"))
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Equal(t, DoctorTargetScope, res.Kind)

	// A docs/ path inside the repo but outside .backlogit.
	res2, err := DoctorTarget(ws, filepath.Join("docs", "note.md"))
	require.NoError(t, err)
	assert.Equal(t, DoctorTargetScope, res2.Kind)

	// An absolute path far outside the workspace.
	res3, err := DoctorTarget(ws, filepath.Join(t.TempDir(), "elsewhere.md"))
	require.NoError(t, err)
	assert.Equal(t, DoctorTargetScope, res3.Kind)
}

func TestDoctorTarget_UnreadableFileIsIO(t *testing.T) {
	ws, queueDir := newTargetTestWorkspace(t)
	// A path inside scope that does not exist → IO kind (not scope, not pass).
	res, err := DoctorTarget(ws, filepath.Join(queueDir, "does-not-exist.md"))
	require.NoError(t, err)
	assert.False(t, res.OK)
	assert.Equal(t, DoctorTargetIO, res.Kind)
}
