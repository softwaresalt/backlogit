package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 143.005-T (Unit 5): RED harness for the report-only shipped-event
// reconciliation audit. The two finding-type constants and the
// DoctorOptions.CheckShippedEventCompleteness field this harness needs to
// compile are declared in internal/core/doctor.go by this same task and carry
// no behavior: no check is registered, so both scenarios fail because zero
// findings come back.
//
// The scenarios assert on DoctorFinding.Description because DoctorFinding keeps
// its current three-field shape; no structured detail field is added.

// seedShippedShipment writes a shipment artifact directly and returns the
// resolved path the routing registry actually produced, so the harness never
// assumes queue/ for a "shipped" status.
func seedShippedShipment(t *testing.T, ws *Workspace, title string, memberIDs []string) *models.Artifact {
	t.Helper()
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, title, memberIDs)
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	// Use the internal ungoverned path (topLevel=false) as a test fixture to
	// reach "shipped" status without the governed ShipShipment envelope;
	// this is intentional so the doctor audit's detection tests see a shipment
	// that may lack a durable shipped event.
	require.NoError(t, moveShipmentStatusWithHeadGuard(ctx, ws, shipment.ID, ShipmentShipped, false, ""))
	reloaded, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	return reloaded
}

// seedArchivedShippedNoEvent creates an "archived with archived_status: shipped
// but no durable shipped event" fixture by writing the archive file directly.
// This bypasses guard 2 (ArchiveItem) intentionally: guard 2 prevents new
// residue; doctor tests verify DETECTION of existing pre-144-F residue.
func seedArchivedShippedNoEvent(t *testing.T, ws *Workspace, title string, memberIDs []string) *models.Artifact {
	t.Helper()
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, title, memberIDs)
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	require.NoError(t, moveShipmentStatusWithHeadGuard(ctx, ws, shipment.ID, ShipmentShipped, false, ""))
	removeShippedEvent(t, ws, shipment.ID)

	// Write the archive file directly, bypassing ArchiveItem guard 2.
	// Mirrors the minimal path used by pre-144-F ungoverned archive operations.
	backlogDir := workspaceStorageRoot(ws)
	currentPath, findErr := FindArtifactPath(ctx, ws, shipment.ID)
	require.NoError(t, findErr)
	archiveDir := filepath.Join(backlogDir, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	archivePath := filepath.Join(archiveDir, filepath.Base(currentPath))

	raw, readErr := os.ReadFile(currentPath)
	require.NoError(t, readErr)
	fm, body, parseErr := models.ParseFrontmatter(string(raw))
	require.NoError(t, parseErr)
	if fm == nil {
		fm = map[string]any{}
	}
	fm["archived_from"] = workspaceRelativePath(ws.RootPath, currentPath)
	fm["archived_status"] = fm["status"]
	fm["status"] = "archived"
	newContent := models.SerializeFrontmatter(fm, body)
	require.NoError(t, os.WriteFile(archivePath, []byte(newContent), 0o644))
	require.NoError(t, os.Remove(currentPath))

	// Upsert to DB so the doctor can find it.
	art := &models.Artifact{
		ID:           shipment.ID,
		Title:        title,
		Status:       models.StatusArchived,
		ArtifactType: "shipment",
	}
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, art))
	reloaded, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	return reloaded
}

// shipment_status_changed record, simulating the ungoverned producers this
// audit detects report-only. It is a fixture helper: the audit itself never
// writes JSONL.
func removeShippedEvent(t *testing.T, ws *Workspace, shipmentID string) {
	t.Helper()
	logPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), shipmentID)
	raw, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return
	}
	require.NoError(t, err)
	kept := make([]string, 0)
	for _, line := range strings.Split(string(raw), "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(line, `"shipment_status_changed"`) && strings.Contains(line, `"shipped"`) {
			continue
		}
		kept = append(kept, line)
	}
	body := ""
	if len(kept) > 0 {
		body = strings.Join(kept, "\n") + "\n"
	}
	require.NoError(t, os.WriteFile(logPath, []byte(body), 0o644))
}

// mustFeature creates a covering feature and returns its ID. Tasks require a
// parent, so every task fixture in this file hangs off one.
func mustFeature(t *testing.T, ws *Workspace, title string) string {
	t.Helper()
	feature, err := CreateArtifact(context.Background(), ws, title, "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(context.Background(), ws.DB, feature))
	return feature.ID
}

func runShippedEventAudit(t *testing.T, ws *Workspace) *DoctorReport {
	t.Helper()
	report, err := Doctor(context.Background(), ws, &DoctorOptions{CheckShippedEventCompleteness: true})
	require.NoError(t, err)
	require.NotNil(t, report)
	return report
}

func findingsOfType(report *DoctorReport, findingType DoctorFindingType) []DoctorFinding {
	out := make([]DoctorFinding, 0)
	for _, finding := range report.Findings {
		if finding.Type == findingType {
			out = append(out, finding)
		}
	}
	return out
}

// Scenario 1: an archived shipment with archived_status "shipped" and no
// shipped event yields exactly one missing_shipped_event finding. The same
// fixture with the shipped event present yields none, and an archived shipment
// whose archived_status is NOT "shipped" yields none, proving the check filters
// on archived_status rather than on "archived".
func TestDoctorShippedEventCompleteness_MissingShippedEvent(t *testing.T) {
	t.Run("archived_shipped_without_event_is_reported", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		task, err := CreateArtifact(ctx, ws, "Missing shipped event task", "task", WithParent(mustFeature(t, ws, "Missing shipped event feature")))
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

		// Use the bypass helper: guard 2 prevents ArchiveItem on shipped-without-event,
		// so the fixture writes the archive file directly (simulating pre-144-F residue).
		shipment := seedArchivedShippedNoEvent(t, ws, "Missing shipped event shipment", []string{task.ID})

		report := runShippedEventAudit(t, ws)
		findings := findingsOfType(report, FindingMissingShippedEvent)
		require.Len(t, findings, 1, "an archived shipped shipment with no shipped event must be reported exactly once")
		assert.Equal(t, shipment.ID, findings[0].ArtifactID)
	})

	t.Run("archived_shipped_with_event_is_clean", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		task, err := CreateArtifact(ctx, ws, "Present shipped event task", "task", WithParent(mustFeature(t, ws, "Present shipped event feature")))
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

		shipment := seedShippedShipment(t, ws, "Present shipped event shipment", []string{task.ID})
		_, err = ArchiveItem(ctx, ws.DB, ws, shipment.ID)
		require.NoError(t, err)

		report := runShippedEventAudit(t, ws)
		assert.Empty(t, findingsOfType(report, FindingMissingShippedEvent),
			"a durable shipped event must clear the finding")
	})

	t.Run("archived_non_shipped_status_is_clean", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()

		task, err := CreateArtifact(ctx, ws, "Abandoned shipment task", "task", WithParent(mustFeature(t, ws, "Abandoned shipment feature")))
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

		shipment, err := CreateShipment(ctx, ws, "Abandoned shipment", []string{task.ID})
		require.NoError(t, err)
		_, err = ClaimShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)
		require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentAbandoned))
		_, err = ArchiveItem(ctx, ws.DB, ws, shipment.ID)
		require.NoError(t, err)

		report := runShippedEventAudit(t, ws)
		assert.Empty(t, findingsOfType(report, FindingMissingShippedEvent),
			"the check must filter on archived_status == shipped, not on archived")
	})
}

// Scenario 2: a shipped, unarchived shipment yields exactly one
// shipped_unarchived_residue finding whose Description records shipped-event
// presence and enumerates the archive candidates stranded alongside it. The
// scenario asserts the ACTUALLY RESOLVED artifact path produced by the routing
// registry for a "shipped" status rather than assuming queue/.
func TestDoctorShippedEventCompleteness_ShippedUnarchivedResidue(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Residue covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := CreateArtifact(ctx, ws, "Residue release scope task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	shipment := seedShippedShipment(t, ws, "Residue shipment", []string{task.ID})

	resolvedPath, err := FindArtifactPath(ctx, ws, shipment.ID)
	require.NoError(t, err, "the shipped shipment must remain discoverable through the routing registry")

	report := runShippedEventAudit(t, ws)
	findings := findingsOfType(report, FindingShippedUnarchivedResidue)
	require.Len(t, findings, 1, "a shipped, unarchived shipment must be reported exactly once")
	assert.Equal(t, shipment.ID, findings[0].ArtifactID)
	assert.Contains(t, findings[0].Description, filepath.ToSlash(workspaceRelativePath(ws.RootPath, resolvedPath)),
		"the finding must report the path the canonical scan actually observed")
	assert.Contains(t, strings.ToLower(findings[0].Description), "shipped event",
		"the finding must record whether the shipped event is present")
	assert.Contains(t, findings[0].Description, task.ID,
		"the finding must enumerate the archive candidates stranded alongside the shipment")
}

// 143.006-T (Unit 6) non-mutation guard: the audit is report-only by contract.
// Running it must leave every file under the workspace logs directory
// byte-identical. This is green from the moment the check exists, so it is a
// guard rather than a red scenario. Doctor EXIT-CODE neutrality is asserted in
// the CLI unit, because core.Doctor returns a report and an error, not a code.
func TestDoctorShippedEventCompleteness_NeverMutatesItemLogs(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	task, err := CreateArtifact(ctx, ws, "Non-mutation task", "task", WithParent(mustFeature(t, ws, "Non-mutation feature")))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Use the bypass helper: guard 2 prevents ArchiveItem on shipped-without-event,
	// so the fixture writes the archive file directly (simulating pre-144-F residue).
	_ = seedArchivedShippedNoEvent(t, ws, "Non-mutation archived shipment", []string{task.ID})

	residueTask, err := CreateArtifact(ctx, ws, "Non-mutation residue task", "task", WithParent(mustFeature(t, ws, "Non-mutation residue feature")))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, residueTask))
	seedShippedShipment(t, ws, "Non-mutation residue shipment", []string{residueTask.ID})

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	before := snapshotLogsTree(t, logsDir)

	report := runShippedEventAudit(t, ws)
	require.NotEmpty(t, report.Findings, "the fixture must actually produce findings for the guard to be meaningful")

	after := snapshotLogsTree(t, logsDir)
	assert.Equal(t, before, after, "the shipped-event audit must never write, synthesize, or rewrite JSONL")
}

// snapshotLogsTree returns a recursive path-to-bytes map of the logs directory.
func snapshotLogsTree(t *testing.T, logsDir string) map[string]string {
	t.Helper()
	tree := make(map[string]string)
	walkErr := filepath.WalkDir(logsDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(logsDir, path)
		if relErr != nil {
			return relErr
		}
		tree[filepath.ToSlash(rel)] = string(raw)
		return nil
	})
	require.NoError(t, walkErr)
	return tree
}

// Review follow-up (143.007-T): a shipment whose manifest member is a FEATURE
// must report the stranded descendants under it, not just the member itself.
// This pins the descendant-expansion semantics so the enumeration cannot
// silently regress to manifest-members-only.
func TestDoctorShippedEventCompleteness_ResidueEnumeratesFeatureDescendants(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Residue feature member", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	child, err := CreateArtifact(ctx, ws, "Residue feature child task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, child))

	shipment := seedShippedShipment(t, ws, "Residue feature shipment", []string{feature.ID})

	report := runShippedEventAudit(t, ws)
	findings := findingsOfType(report, FindingShippedUnarchivedResidue)
	require.Len(t, findings, 1)
	assert.Equal(t, shipment.ID, findings[0].ArtifactID)
	assert.Contains(t, findings[0].Description, feature.ID,
		"the feature manifest member must be enumerated")
	assert.Contains(t, findings[0].Description, child.ID,
		"a descendant stranded under the feature member must be enumerated")
	assert.Contains(t, findings[0].Description, "approximate",
		"the enumeration must be labelled as an approximation of the ship-time sweep")
}
