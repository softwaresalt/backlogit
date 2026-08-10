package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// helperWriteArtifact writes a minimal markdown artifact to dir/filename.
func helperWriteArtifact(t *testing.T, dir, filename, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644))
}

// newDoctorTestWorkspace creates a Workspace with standard queue layout config.
// When withConfig is true, it sets Workspace.Config so tests exercise the
// config-driven artifact search path. Pass nil Config to use the fallback
// "scan all dirs" path.
func newDoctorTestWorkspace(t *testing.T, rootPath string, withConfig bool) *Workspace {
	t.Helper()
	ws := &Workspace{
		RootPath:    rootPath,
		StorageRoot: filepath.Join(rootPath, ".backlogit"),
	}
	if withConfig {
		ws.Config = &config.WorkspaceConfig{
			ArtifactTypes: map[string]*config.ArtifactTypeConfig{
				"feature": {Prefix: "F", NameFormat: "{id}-{prefix}"},
				"task":    {Prefix: "T", NameFormat: "{id}-{prefix}"},
				"subtask": {Prefix: "ST", NameFormat: "{id}-{prefix}"},
			},
			MaxSlugLength: 60,
			QueueLayout: &config.QueueLayoutConfig{
				RootDir: "queue",
				Levels: []config.HierarchyLevel{
					{Level: 1, Types: []string{"feature", "deliberation", "shipment"}},
					{Level: 2, Types: []string{"task", "review"}},
					{Level: 3, Types: []string{"subtask"}},
				},
			},
		}
	}
	return ws
}

func TestDoctor_LegacyRootTaskNotFlaggedAsOrphan(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	archiveDir := filepath.Join(wsRoot, "archive")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "logs"), 0o755))

	// Legacy task at root level (level 1) with no parent — should NOT be flagged.
	helperWriteArtifact(t, archiveDir, "001-T.md", `---
id: "001-T"
title: "Legacy root task"
status: done
artifact_type: task
level: 1
hierarchy_path: "001"
---
Legacy task.
`)

	// Legacy bug at root level — type not in queue layout, no level field.
	helperWriteArtifact(t, archiveDir, "001-B.md", `---
id: "001-B"
title: "Legacy root bug"
status: done
artifact_type: bug
---
Legacy bug.
`)

	// Use a configured workspace and verify these legacy root-level artifacts are not flagged as orphans.
	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckOrphans: true})
	require.NoError(t, err)

	for _, f := range report.Findings {
		if f.Type == FindingOrphanedArtifact {
			t.Errorf("unexpected orphan finding: %s (%s)", f.ArtifactID, f.Description)
		}
	}
}

func TestDoctor_RealOrphanedChildTaskStillFlagged(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "logs"), 0o755))

	// Genuine orphaned child task at level 2 — SHOULD be flagged.
	helperWriteArtifact(t, queueDir, "099.001-T.md", `---
id: "099.001-T"
title: "Orphaned child task"
status: active
artifact_type: task
level: 2
hierarchy_path: "099/099.001"
---
This task has no parent.
`)

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckOrphans: true})
	require.NoError(t, err)

	var orphanIDs []string
	for _, f := range report.Findings {
		if f.Type == FindingOrphanedArtifact {
			orphanIDs = append(orphanIDs, f.ArtifactID)
		}
	}
	assert.Contains(t, orphanIDs, "099.001-T", "real orphaned child task should be flagged")
}

func TestDoctor_ReturnedToBacklogEventStillIgnored(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	logsDir := filepath.Join(wsRoot, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Task at level 2 with no parent but has returned_to_backlog event.
	helperWriteArtifact(t, queueDir, "050.001-T.md", `---
id: "050.001-T"
title: "Returned task"
status: active
artifact_type: task
level: 2
hierarchy_path: "050/050.001"
---
Returned to backlog.
`)

	logContent := `{"event_type":"returned_to_backlog","timestamp":"2026-04-20T00:00:00Z"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "050.001-T.jsonl"), []byte(logContent), 0o644))

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckOrphans: true})
	require.NoError(t, err)

	for _, f := range report.Findings {
		if f.Type == FindingOrphanedArtifact && f.ArtifactID == "050.001-T" {
			t.Error("returned_to_backlog task should not be flagged as orphan")
		}
	}
}

func TestDoctor_FixOrphansArchivesOrphanedTask(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "logs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "archive"), 0o755))

	// Genuine orphaned child task at level 2 — should be archived.
	helperWriteArtifact(t, queueDir, "099.001-T.md", `---
id: "099.001-T"
title: "Orphaned child task"
status: active
artifact_type: task
level: 2
hierarchy_path: "099/099.001"
---
This task has no parent.
`)

	ws := newDoctorTestWorkspace(t, tmp, true)

	dbPath := filepath.Join(wsRoot, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })
	ws.DB = database

	// Index the artifact.
	a := &models.Artifact{
		ID: "099.001-T", Title: "Orphaned child task", Status: models.StatusActive,
		ArtifactType: "task", Level: 2,
	}
	require.NoError(t, db.UpsertItem(context.Background(), database, a))

	report, err := Doctor(context.Background(), ws, &DoctorOptions{
		CheckOrphans: true,
		FixOrphans:   true,
	})
	require.NoError(t, err)

	// Should have the finding AND a fix action.
	assert.Len(t, report.Findings, 1)
	assert.Len(t, report.FixActions, 1)
	assert.Equal(t, "099.001-T", report.FixActions[0].ArtifactID)
	assert.Equal(t, FixArchived, report.FixActions[0].Type)

	// File should be in archive/, not queue/.
	assert.NoFileExists(t, filepath.Join(queueDir, "099.001-T.md"))
	assert.FileExists(t, filepath.Join(wsRoot, "archive", "099.001-T.md"))
}

func TestDoctor_FixOrphansSkipsReturnedToBacklog(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	logsDir := filepath.Join(wsRoot, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "archive"), 0o755))

	// Task with returned_to_backlog event — should NOT be fixed.
	helperWriteArtifact(t, queueDir, "050.001-T.md", `---
id: "050.001-T"
title: "Returned task"
status: active
artifact_type: task
level: 2
hierarchy_path: "050/050.001"
---
Returned to backlog.
`)
	logContent := `{"event_type":"returned_to_backlog","timestamp":"2026-04-20T00:00:00Z"}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "050.001-T.jsonl"), []byte(logContent), 0o644))

	ws := newDoctorTestWorkspace(t, tmp, true)

	dbPath := filepath.Join(wsRoot, "backlogit.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })
	ws.DB = database

	report, err := Doctor(context.Background(), ws, &DoctorOptions{
		CheckOrphans: true,
		FixOrphans:   true,
	})
	require.NoError(t, err)

	assert.Empty(t, report.Findings, "returned task should not appear as orphan")
	assert.Empty(t, report.FixActions, "no fix actions for returned task")
	assert.FileExists(t, filepath.Join(queueDir, "050.001-T.md"), "file should remain in queue")
}

func TestDoctor_ReportOnlyByDefault(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	queueDir := filepath.Join(wsRoot, "queue")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "logs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "archive"), 0o755))

	helperWriteArtifact(t, queueDir, "099.001-T.md", `---
id: "099.001-T"
title: "Orphaned child task"
status: active
artifact_type: task
level: 2
hierarchy_path: "099/099.001"
---
This task has no parent.
`)

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{
		CheckOrphans: true,
	})
	require.NoError(t, err)

	assert.Len(t, report.Findings, 1, "orphan should be reported")
	assert.Empty(t, report.FixActions, "no fix actions when fix-orphans is false")
	assert.FileExists(t, filepath.Join(queueDir, "099.001-T.md"), "file should remain in queue")
}

// TestDoctor_ArchivedFromAudit verifies U4 (067.004-T): the read-only audit flags
// archive records whose archived_from is self-referential (resolves to the record's
// own archive path) or malformed (not a markdown path), while leaving canonical,
// fieldless, and legitimate non-self-ref subdir records untouched.
func TestDoctor_ArchivedFromAudit(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	archiveDir := filepath.Join(wsRoot, "archive")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "queue"), 0o755))

	// Self-referential: archived_from points at its own archive path -> SELF-REF.
	helperWriteArtifact(t, archiveDir, "100-T.md", "---\nid: 100-T\ntitle: Self ref\nstatus: archived\nartifact_type: task\narchived_from: .backlogit/archive/100-T.md\n---\nBody\n")
	// Canonical: archived_from points at the queue restore path -> NO finding.
	helperWriteArtifact(t, archiveDir, "101-T.md", "---\nid: 101-T\ntitle: Canonical\nstatus: archived\nartifact_type: task\narchived_from: .backlogit/queue/101-T.md\n---\nBody\n")
	// Fieldless: no archived_from at all -> NO finding.
	helperWriteArtifact(t, archiveDir, "102-T.md", "---\nid: 102-T\ntitle: Fieldless\nstatus: archived\nartifact_type: task\n---\nBody\n")
	// Legitimate non-self-ref subdir record (036-DL style) -> NO finding.
	helperWriteArtifact(t, archiveDir, "036-DL.md", "---\nid: 036-DL\ntitle: Deliberation\nstatus: archived\nartifact_type: deliberation\narchived_from: .backlogit/deliberations/036-DL.md\n---\nBody\n")
	// Malformed: archived_from is a stray status value, not a markdown path -> MALFORMED.
	helperWriteArtifact(t, archiveDir, "103-T.md", "---\nid: 103-T\ntitle: Malformed\nstatus: archived\nartifact_type: task\narchived_from: done\n---\nBody\n")

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: true})
	require.NoError(t, err)

	var selfRef, malformed []string
	for _, f := range report.Findings {
		switch f.Type {
		case FindingArchivedFromSelfRef:
			selfRef = append(selfRef, f.ArtifactID)
		case FindingArchivedFromMalformed:
			malformed = append(malformed, f.ArtifactID)
		default:
			t.Errorf("unexpected finding type %q for %s", f.Type, f.ArtifactID)
		}
	}

	assert.Equal(t, []string{"100-T"}, selfRef, "only the self-ref record must be flagged self-ref")
	assert.Equal(t, []string{"103-T"}, malformed, "only the malformed record must be flagged malformed")
	assert.NotContains(t, selfRef, "101-T")
	assert.NotContains(t, selfRef, "102-T")
	assert.NotContains(t, selfRef, "036-DL")
}

// TestDoctor_FixArchivedFromRepairsSelfRef verifies U5 (067.005-T): the
// --fix-archived-from repair rewrites a self-referential archived_from to the
// canonical queue restore path using the body-preserving docline codec, emits a
// per-record FixAction, is idempotent (a second run is a byte-stable no-op), and
// leaves canonical and malformed records untouched.
func TestDoctor_FixArchivedFromRepairsSelfRef(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	archiveDir := filepath.Join(wsRoot, "archive")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "queue"), 0o755))

	// Self-ref record with a CRLF body to prove body bytes are preserved verbatim.
	selfRefContent := "---\nid: 200-T\ntitle: Self ref\nstatus: archived\nartifact_type: task\narchived_from: .backlogit/archive/200-T.md\n---\nBody line 1\r\nBody line 2\n"
	helperWriteArtifact(t, archiveDir, "200-T.md", selfRefContent)
	// Canonical record: must be left byte-for-byte untouched.
	canonContent := "---\nid: 201-T\ntitle: Canonical\nstatus: archived\nartifact_type: task\narchived_from: .backlogit/queue/201-T.md\n---\nCanon body\n"
	helperWriteArtifact(t, archiveDir, "201-T.md", canonContent)
	// Malformed record: flagged only, never repaired.
	malformedContent := "---\nid: 202-T\ntitle: Malformed\nstatus: archived\nartifact_type: task\narchived_from: done\n---\nMal body\n"
	helperWriteArtifact(t, archiveDir, "202-T.md", malformedContent)

	ws := newDoctorTestWorkspace(t, tmp, true)
	selfRefPath := filepath.Join(archiveDir, "200-T.md")
	const wantBody = "Body line 1\r\nBody line 2\n"

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: true, FixArchivedFrom: true})
	require.NoError(t, err)

	// Exactly one FixAction, for the self-ref record only.
	var repaired []string
	for _, a := range report.FixActions {
		if a.Type == FixArchivedFromRepaired {
			repaired = append(repaired, a.ArtifactID)
		}
	}
	assert.Equal(t, []string{"200-T"}, repaired, "only the self-ref record is repaired")

	// archived_from rewritten to the canonical queue path; body bytes preserved.
	rawAfter, readErr := os.ReadFile(selfRefPath)
	require.NoError(t, readErr)
	fm, _, perr := models.ParseFrontmatter(string(rawAfter))
	require.NoError(t, perr)
	assert.Equal(t, ".backlogit/queue/200-T.md", fm["archived_from"])
	assert.True(t, bytes.HasSuffix(rawAfter, []byte(wantBody)),
		"body bytes must be preserved verbatim (incl. CRLF)")

	// Canonical and malformed records left byte-for-byte untouched.
	canonAfter, _ := os.ReadFile(filepath.Join(archiveDir, "201-T.md"))
	assert.Equal(t, canonContent, string(canonAfter), "canonical record must be untouched")
	malAfter, _ := os.ReadFile(filepath.Join(archiveDir, "202-T.md"))
	assert.Equal(t, malformedContent, string(malAfter), "malformed record must be untouched (flag-only)")

	// Idempotency: a second --fix run is a no-op and byte-stable.
	report2, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: true, FixArchivedFrom: true})
	require.NoError(t, err)
	for _, a := range report2.FixActions {
		assert.NotEqual(t, FixArchivedFromRepaired, a.Type, "second run must not repair anything")
	}
	rawSecond, _ := os.ReadFile(selfRefPath)
	assert.Equal(t, rawAfter, rawSecond, "second run must be byte-stable")
}

// TestDoctor_FixArchivedFromRequiresCheck verifies the destructive repair flag is
// rejected with an explicit error when the archived_from audit is not also enabled,
// instead of silently doing nothing.
func TestDoctor_FixArchivedFromRequiresCheck(t *testing.T) {
	tmp := t.TempDir()
	ws := newDoctorTestWorkspace(t, tmp, true)

	_, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: false, FixArchivedFrom: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FixArchivedFrom requires CheckArchivedFrom")
}

// TestDoctor_FixMalformedClearsArchivedFrom verifies 069.001-T: --fix-malformed
// clears the bogus archived_from field on the malformed class (records with no
// queue restore target), body-preserving, leaving self-ref and canonical records
// untouched, dropping the malformed audit count to zero, and idempotent.
func TestDoctor_FixMalformedClearsArchivedFrom(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	archiveDir := filepath.Join(wsRoot, "archive")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "queue"), 0o755))

	// Malformed record with a CRLF body to prove body bytes are preserved verbatim.
	malformedContent := "---\nid: 202-T\ntitle: Malformed\nstatus: archived\narchived_from: done\nartifact_type: deliberation\n---\nMal line 1\r\nMal line 2\n"
	helperWriteArtifact(t, archiveDir, "202-T.md", malformedContent)
	// Self-ref record: --fix-malformed must NOT touch it.
	selfRefContent := "---\nid: 200-T\ntitle: Self ref\nstatus: archived\nartifact_type: task\narchived_from: .backlogit/archive/200-T.md\n---\nSelf body\n"
	helperWriteArtifact(t, archiveDir, "200-T.md", selfRefContent)
	// Canonical record: untouched.
	canonContent := "---\nid: 201-T\ntitle: Canonical\nstatus: archived\nartifact_type: task\narchived_from: .backlogit/queue/201-T.md\n---\nCanon body\n"
	helperWriteArtifact(t, archiveDir, "201-T.md", canonContent)

	ws := newDoctorTestWorkspace(t, tmp, true)
	malPath := filepath.Join(archiveDir, "202-T.md")
	const wantBody = "Mal line 1\r\nMal line 2\n"

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: true, FixMalformed: true})
	require.NoError(t, err)

	// Exactly one cleared FixAction, for the malformed record only.
	var cleared []string
	for _, a := range report.FixActions {
		if a.Type == FixArchivedFromCleared {
			cleared = append(cleared, a.ArtifactID)
		}
	}
	assert.Equal(t, []string{"202-T"}, cleared, "only the malformed record is cleared")

	// archived_from removed entirely (not stamped); body bytes preserved (incl. CRLF).
	rawAfter, readErr := os.ReadFile(malPath)
	require.NoError(t, readErr)
	fm, _, perr := models.ParseFrontmatter(string(rawAfter))
	require.NoError(t, perr)
	_, present := fm["archived_from"]
	assert.False(t, present, "archived_from field must be removed, not stamped")
	assert.True(t, bytes.HasSuffix(rawAfter, []byte(wantBody)), "body bytes preserved verbatim (incl. CRLF)")

	// Self-ref and canonical untouched.
	selfAfter, _ := os.ReadFile(filepath.Join(archiveDir, "200-T.md"))
	assert.Equal(t, selfRefContent, string(selfAfter), "self-ref record must be untouched by --fix-malformed")
	canonAfter, _ := os.ReadFile(filepath.Join(archiveDir, "201-T.md"))
	assert.Equal(t, canonContent, string(canonAfter), "canonical record must be untouched")

	// Audit after fix: zero malformed findings.
	report2, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: true})
	require.NoError(t, err)
	for _, f := range report2.Findings {
		assert.NotEqual(t, FindingArchivedFromMalformed, f.Type, "no malformed findings after clear")
	}

	// Idempotent: a second --fix-malformed run is a byte-stable no-op.
	report3, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: true, FixMalformed: true})
	require.NoError(t, err)
	for _, a := range report3.FixActions {
		assert.NotEqual(t, FixArchivedFromCleared, a.Type, "second run must not clear anything")
	}
	rawSecond, _ := os.ReadFile(malPath)
	assert.Equal(t, rawAfter, rawSecond, "second run must be byte-stable")
}

// TestDoctor_FixMalformedRequiresCheck verifies --fix-malformed is rejected with an
// explicit error when the archived_from audit is not also enabled.
func TestDoctor_FixMalformedRequiresCheck(t *testing.T) {
	tmp := t.TempDir()
	ws := newDoctorTestWorkspace(t, tmp, true)

	_, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: false, FixMalformed: true})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "FixMalformed requires CheckArchivedFrom")
}

// TestDoctor_OverArchivedFeatureAudit_FlagsWronglyClosedCoveringFeature is the
// 133.005-T (Unit 3) crafted-fixture regression: a feature closed (status
// done) that was never an explicit member of any shipment manifest, with a
// sibling task's returned_to_backlog event naming it as the source feature,
// must be reported. This reconstructs the historical pre-Unit-2 bug signature
// without needing a live ShipShipment run.
func TestDoctor_OverArchivedFeatureAudit_FlagsWronglyClosedCoveringFeature(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	archiveDir := filepath.Join(wsRoot, "archive")
	logsDir := filepath.Join(wsRoot, "logs")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "queue"), 0o755))
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// Covering feature rolled up to "done" and relocated to archive/ by the
	// pre-Unit-2 cascade bug, even though it was never itself shipped.
	helperWriteArtifact(t, archiveDir, "001-F.md", `---
id: "001-F"
title: "Over-archived covering feature"
status: done
artifact_type: feature
level: 1
hierarchy_path: "001"
---
Covering feature.
`)

	// The shipment that triggered the rollup only ever listed a sibling task
	// as an explicit member -- the feature itself is NOT in the manifest.
	helperWriteArtifact(t, archiveDir, "001-S.md", `---
id: "001-S"
title: "Partial-feature shipment"
status: shipped
artifact_type: shipment
level: 1
hierarchy_path: "001-S"
custom_fields:
    items:
        - 001.001-T
---
Shipment record.
`)

	// A sibling task excluded from the shipment was returned to the backlog,
	// recording feature_id "001-F" as its provenance -- the signal detection
	// must use instead of parent_id (which is cleared on return).
	logContent := `{"event_type":"returned_to_backlog","timestamp":"2026-04-20T00:00:00Z","delta":{"feature_id":"001-F"}}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "001.002-T.jsonl"), []byte(logContent), 0o644))

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckOverArchivedFeatures: true})
	require.NoError(t, err)

	var flaggedIDs []string
	for _, f := range report.Findings {
		if f.Type == FindingOverArchivedCoveringFeature {
			flaggedIDs = append(flaggedIDs, f.ArtifactID)
		}
	}
	assert.Contains(t, flaggedIDs, "001-F", "over-archived covering feature must be reported")

	// The audit performs no mutation: the feature record must be byte-identical.
	after, readErr := os.ReadFile(filepath.Join(archiveDir, "001-F.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(after), "status: done", "audit must not mutate the feature record")
}

// TestDoctor_OverArchivedFeatureAudit_CleanWorkspaceReportsNoFindings covers
// three near-miss scenarios that must NOT be flagged, proving the audit's
// precision and not just its recall: (1) a feature closed by a legitimate
// full-feature shipment where it IS an explicit manifest member; (2) a
// feature that has a returned-to-backlog sibling but is itself still open
// (not done/archived); (3) a closed feature with no returned_to_backlog
// provenance at all (closed through an unrelated path).
func TestDoctor_OverArchivedFeatureAudit_CleanWorkspaceReportsNoFindings(t *testing.T) {
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	archiveDir := filepath.Join(wsRoot, "archive")
	queueDir := filepath.Join(wsRoot, "queue")
	logsDir := filepath.Join(wsRoot, "logs")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	// (1) Legitimate full-feature ship: the feature IS an explicit member.
	helperWriteArtifact(t, archiveDir, "002-F.md", `---
id: "002-F"
title: "Fully-shipped feature"
status: done
artifact_type: feature
level: 1
hierarchy_path: "002"
---
Fully shipped.
`)
	helperWriteArtifact(t, archiveDir, "002-S.md", `---
id: "002-S"
title: "Full-feature shipment"
status: shipped
artifact_type: shipment
level: 1
hierarchy_path: "002-S"
custom_fields:
    items:
        - 002-F
---
Shipment record.
`)

	// (2) Feature still open (queued), even though a sibling names it via
	// returned_to_backlog -- must not be flagged because it is not closed.
	helperWriteArtifact(t, queueDir, "003-F.md", `---
id: "003-F"
title: "Still-open covering feature"
status: queued
artifact_type: feature
level: 1
hierarchy_path: "003"
---
Still open.
`)
	logContent := `{"event_type":"returned_to_backlog","timestamp":"2026-04-20T00:00:00Z","delta":{"feature_id":"003-F"}}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "003.002-T.jsonl"), []byte(logContent), 0o644))

	// (3) Closed feature with no returned_to_backlog provenance anywhere --
	// closed through an ordinary, unrelated path.
	helperWriteArtifact(t, archiveDir, "004-F.md", `---
id: "004-F"
title: "Ordinarily closed feature"
status: archived
artifact_type: feature
level: 1
hierarchy_path: "004"
---
Closed normally.
`)

	ws := newDoctorTestWorkspace(t, tmp, true)

	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckOverArchivedFeatures: true})
	require.NoError(t, err)

	for _, f := range report.Findings {
		if f.Type == FindingOverArchivedCoveringFeature {
			t.Errorf("unexpected over-archived-feature finding: %s (%s)", f.ArtifactID, f.Description)
		}
	}
}
