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

// TestDoctorTarget_NilHeaderDefFailsClosed is the durable regression guard for
// the fail-open defect where a nil workspace HeaderDef caused required-field
// validation to be silently skipped and the target to be reported kind=pass. A
// nil HeaderDef is a system/config precondition fault (the schema needed to judge
// the artifact is absent), so it must fail closed to kind=io (exit 3) with a
// distinct "header definition not loaded" diagnostic — never a false pass. The
// loaded-vs-nil pair pins the classification precedence deterministically.
func TestDoctorTarget_NilHeaderDefFailsClosed(t *testing.T) {
	ws, queueDir := newTargetTestWorkspace(t)
	path := filepath.Join(queueDir, "100.001-T.md")
	require.NoError(t, os.WriteFile(path, []byte(validTaskContent), 0o644))

	// Scenario 2 (regression guard): with HeaderDef loaded by WriteDefaults, a
	// valid artifact still passes — the fix must not regress the loaded path.
	loaded, err := DoctorTarget(ws, path)
	require.NoError(t, err)
	assert.True(t, loaded.OK, "loaded HeaderDef + valid artifact should pass: %+v", loaded)
	assert.Equal(t, DoctorTargetPass, loaded.Kind)

	// Scenario 1 (fail-closed): nil the HeaderDef so the required-field schema is
	// absent, then re-validate the same otherwise-valid artifact. Validation
	// cannot be performed, so the result must fail closed to io (exit 3), NOT pass.
	ws.HeaderDef = nil
	res, err := DoctorTarget(ws, path)
	require.NoError(t, err)
	assert.False(t, res.OK, "nil HeaderDef must not report a pass: %+v", res)
	assert.Equal(t, DoctorTargetIO, res.Kind,
		"nil HeaderDef is a system/config fault → io/exit 3, not pass: %+v", res)
	assert.Contains(t, res.Message, "header definition not loaded",
		"io diagnostic must distinguish an absent schema from a file-read IO fault: %+v", res)
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

// TestDoctorTarget_SymlinkEscapeRejectedAsScope is the regression guard for
// symlink-based scope confinement bypass: a path that is lexically inside
// .backlogit but is a symlink to a file OUTSIDE the storage root must be
// rejected as scope (not followed and read), because os.ReadFile follows
// symlinks.
func TestDoctorTarget_SymlinkEscapeRejectedAsScope(t *testing.T) {
	ws, queueDir := newTargetTestWorkspace(t)

	// A secret file well outside the workspace storage root.
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "secret.md")
	require.NoError(t, os.WriteFile(outside, []byte(validTaskContent), 0o644))

	// A symlink INSIDE .backlogit/queue pointing at the outside file.
	link := filepath.Join(queueDir, "100.009-T.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks not creatable in this environment: %v", err)
	}

	res, err := DoctorTarget(ws, link)
	require.NoError(t, err)
	assert.False(t, res.OK, "a symlink escaping the storage root must not pass: %+v", res)
	assert.Equal(t, DoctorTargetScope, res.Kind,
		"symlink escape must be rejected as scope, not read through: %+v", res)
}
