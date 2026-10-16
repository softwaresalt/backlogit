package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeItem(t *testing.T, root, dir, id, atype, status, extra string) {
	t.Helper()
	d := filepath.Join(root, dir)
	require.NoError(t, os.MkdirAll(d, 0o755))
	body := "---\nid: " + id + "\ntitle: t " + id + "\nartifact_type: " + atype +
		"\nstatus: " + status + "\ncreated_at: 2026-01-01T00:00:00Z\nupdated_at: 2026-01-01T00:00:00Z\n" +
		extra + "---\n\nbody\n"
	require.NoError(t, os.WriteFile(filepath.Join(d, id+".md"), []byte(body), 0o644))
}

func fixtureWorkspace(t *testing.T, shipStatus string) string {
	t.Helper()
	root := t.TempDir()
	writeItem(t, root, "queue", "900-S", "shipment", shipStatus,
		"custom_fields:\n  items:\n    - 900-F\n    - 900.001-T\n    - 900.002-T\n    - 900.003-T\n")
	writeItem(t, root, "queue", "900-F", "feature", "done", "")
	writeItem(t, root, "queue", "900.001-T", "task", "done", "commit: abc1234\n")
	writeItem(t, root, "queue", "900.002-T", "task", "active", "")
	writeItem(t, root, "archive", "900.003-T", "task", "archived", "archived_from: queue\narchived_status: done\ncommit: def5678\n")
	return root
}

func TestU159003_ShipmentEvidenceMatchesGroundTruth(t *testing.T) {
	root := fixtureWorkspace(t, "active")
	res := (&Backlog{Root: root}).ShipmentEvidence("900-S")

	require.Equal(t, StatusVerified, res.Status, res.Reason)
	assert.Equal(t, "900-S", res.ShipmentID)
	assert.Equal(t, "active", res.ShipmentStatus)
	assert.False(t, res.Closed)
	require.Len(t, res.Members, 4)
	assert.Equal(t, 3, res.TaskTotal)
	assert.Equal(t, 2, res.TasksDone, "archived member counts by its archived_status")
	assert.Equal(t, map[string]int{"done": 2, "active": 1}, res.TaskCounts)
	assert.Equal(t, "abc1234", res.Provenance.MemberCommits["900.001-T"])
	assert.Equal(t, "def5678", res.Provenance.MemberCommits["900.003-T"])

	byID := map[string]Member{}
	for _, m := range res.Members {
		byID[m.ID] = m
	}
	assert.True(t, byID["900.003-T"].Archived)
	assert.Equal(t, "done", byID["900.003-T"].Status)
	assert.False(t, byID["900.001-T"].Archived)
}

func TestU159003_ClosureState(t *testing.T) {
	root := fixtureWorkspace(t, "shipped")
	res := (&Backlog{Root: root}).ShipmentEvidence("900-S")
	require.Equal(t, StatusVerified, res.Status, res.Reason)
	assert.True(t, res.Closed)
	assert.Equal(t, "shipped", res.ShipmentStatus)
}

func TestU159003_ArchivedShipmentProvenance(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, "archive", "901-S", "shipment", "archived",
		"archived_from: queue\narchived_status: shipped\ncommit: \"9999999\"\ncustom_fields:\n  items: []\n")
	res := (&Backlog{Root: root}).ShipmentEvidence("901-S")
	require.Equal(t, StatusVerified, res.Status, res.Reason)
	assert.True(t, res.Closed)
	assert.Equal(t, "shipped", res.ShipmentStatus)
	assert.Equal(t, "queue", res.Provenance.ArchivedFrom)
	assert.Equal(t, "9999999", res.Provenance.ShipmentCommit)
}

func TestU159003_MissingShipmentOrMemberDegrades(t *testing.T) {
	root := fixtureWorkspace(t, "active")
	assert.Equal(t, StatusDegraded, (&Backlog{Root: root}).ShipmentEvidence("777-S").Status)

	require.NoError(t, os.Remove(filepath.Join(root, "queue", "900.002-T.md")))
	res := (&Backlog{Root: root}).ShipmentEvidence("900-S")
	assert.Equal(t, StatusDegraded, res.Status)
	assert.Contains(t, res.Reason, "900.002-T")
}

func TestU159003_RejectsTraversalAndNonShipment(t *testing.T) {
	root := fixtureWorkspace(t, "active")
	for _, id := range []string{"../queue/900-S", "..\\x", "", "900-S/../../x"} {
		res := (&Backlog{Root: root}).ShipmentEvidence(id)
		assert.Equal(t, StatusDegraded, res.Status, id)
	}
	res := (&Backlog{Root: root}).ShipmentEvidence("900-F")
	assert.Equal(t, StatusDegraded, res.Status)
	assert.True(t, strings.Contains(res.Reason, "shipment"))
}

func TestU159003_TraversalMemberIDDegrades(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, "queue", "902-S", "shipment", "active",
		"custom_fields:\n  items:\n    - ../../secret\n")
	res := (&Backlog{Root: root}).ShipmentEvidence("902-S")
	assert.Equal(t, StatusDegraded, res.Status)
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink on this OS/user: %v", err)
	}
}

func TestBacklogLoad_FileSymlinkEscapeDegrades(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	outside := filepath.Join(base, "outside")
	writeItem(t, outside, "queue", "905-S", "shipment", "active", "custom_fields:\n  items: []\n")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "queue"), 0o755))
	symlinkOrSkip(t, filepath.Join(outside, "queue", "905-S.md"), filepath.Join(root, "queue", "905-S.md"))
	res := (&Backlog{Root: root}).ShipmentEvidence("905-S")
	assert.Equal(t, StatusDegraded, res.Status)
}

func TestBacklogLoad_DirSymlinkEscapeDegrades(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	outside := filepath.Join(base, "outside")
	writeItem(t, outside, "queue", "906-S", "shipment", "active", "custom_fields:\n  items: []\n")
	require.NoError(t, os.MkdirAll(root, 0o755))
	symlinkOrSkip(t, filepath.Join(outside, "queue"), filepath.Join(root, "queue"))
	res := (&Backlog{Root: root}).ShipmentEvidence("906-S")
	assert.Equal(t, StatusDegraded, res.Status)
}

func TestBacklogLoad_InRootSymlinkStillWorks(t *testing.T) {
	root := t.TempDir()
	writeItem(t, root, "real", "907-S", "shipment", "active", "custom_fields:\n  items: []\n")
	symlinkOrSkip(t, "real", filepath.Join(root, "queue"))
	res := (&Backlog{Root: root}).ShipmentEvidence("907-S")
	assert.Equal(t, StatusVerified, res.Status, res.Reason)
}

func TestShipmentManifestValidation(t *testing.T) {
	cases := []struct {
		name, items, reason string
	}{
		{"non-string entry", "custom_fields:\n  items: [900.001-T, 42]\n", "non-string"},
		{"non-list value", "custom_fields:\n  items: 900.001-T\n", "not a list"},
		{"duplicate ids", "custom_fields:\n  items: [900.001-T, 900.001-T]\n", "duplicate"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeItem(t, root, "queue", "910-S", "shipment", "active", tc.items)
			writeItem(t, root, "queue", "900.001-T", "task", "done", "")
			res := (&Backlog{Root: root}).ShipmentEvidence("910-S")
			assert.Equal(t, StatusDegraded, res.Status)
			assert.Contains(t, res.Reason, tc.reason)
			assert.Empty(t, res.Members)
		})
	}
}
