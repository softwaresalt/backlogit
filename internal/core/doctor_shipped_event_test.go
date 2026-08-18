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
	require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentShipped))
	reloaded, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	return reloaded
}

// removeShippedEvent rewrites the item log without any shipped
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

		shipment := seedShippedShipment(t, ws, "Missing shipped event shipment", []string{task.ID})
		removeShippedEvent(t, ws, shipment.ID)
		_, err = ArchiveItem(ctx, ws.DB, ws, shipment.ID)
		require.NoError(t, err)

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
