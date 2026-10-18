package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeReadinessFixture writes a Markdown artifact fixture directly into the
// workspace storage root. The SQLite index is deliberately never populated, so
// every verdict must come from Markdown provenance (R2).
func writeReadinessFixture(t *testing.T, ws *Workspace, dir, id, frontmatter string) {
	t.Helper()
	target := filepath.Join(workspaceStorageRoot(ws), dir)
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, id+".md"), []byte(frontmatter), 0o644))
}

func readinessShipmentMarkdown(id, extra string) string {
	return "---\nid: " + id + "\nartifact_type: shipment\ntitle: Fixture " + id + "\n" + extra + "---\n\nfixture body\n"
}

// TestShipmentPredecessorShipped pins the shipped-provenance predicate
// behavior (A-U3 / 187.003-T). No case may ever expect true for missing or
// ambiguous provenance.
func TestShipmentPredecessorShipped(t *testing.T) {
	type tc struct {
		name       string
		dir        string
		id         string
		content    string
		queryID    string
		wantOK     bool
		wantReason string
	}
	cases := []tc{
		{name: "accept live shipped", dir: "queue", id: "900-S", content: readinessShipmentMarkdown("900-S", "status: shipped\n"), wantOK: true},
		{name: "accept archived with archived_status shipped", dir: "archive", id: "901-S", content: readinessShipmentMarkdown("901-S", "status: archived\narchived_status: shipped\n"), wantOK: true},
		{name: "reject archived abandoned", dir: "archive", id: "902-S", content: readinessShipmentMarkdown("902-S", "status: archived\narchived_status: abandoned\n"), wantReason: "not_shipped:archived/abandoned"},
		{name: "reject archived without archived_status", dir: "archive", id: "903-S", content: readinessShipmentMarkdown("903-S", "status: archived\n"), wantReason: "not_shipped:archived/unknown"},
		{name: "reject live abandoned", dir: "queue", id: "904-S", content: readinessShipmentMarkdown("904-S", "status: abandoned\n"), wantReason: "not_shipped:abandoned"},
		{name: "reject live queued", dir: "queue", id: "905-S", content: readinessShipmentMarkdown("905-S", "status: queued\n"), wantReason: "not_shipped:queued"},
		{name: "reject live queued despite stale archived_status shipped", dir: "queue", id: "911-S", content: readinessShipmentMarkdown("911-S", "status: queued\narchived_status: shipped\n"), wantReason: "not_shipped:queued"},
		{name: "reject disposed shipment live", dir: "queue", id: "906-S", content: readinessShipmentMarkdown("906-S", "status: abandoned\ncustom_fields:\n    disposition: disposed\n    disposition_reason: superseded\n"), wantReason: "not_shipped:abandoned"},
		{name: "reject disposed shipment archived", dir: "archive", id: "907-S", content: readinessShipmentMarkdown("907-S", "status: archived\narchived_status: abandoned\ncustom_fields:\n    disposition: disposed\n"), wantReason: "not_shipped:archived/abandoned"},
		{name: "unknown id fails closed", queryID: "999-S", wantReason: "provenance_missing"},
		{name: "unparsable frontmatter fails closed", dir: "queue", id: "908-S", content: "---\nid: 908-S\nstatus: [unterminated\n---\n", wantReason: "provenance_missing"},
		{name: "non shipment returns not_shipment", dir: "queue", id: "909-T", content: "---\nid: 909-T\nartifact_type: task\ntitle: Fixture task\nstatus: shipped\n---\n\nbody\n", wantReason: "not_shipment"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			if c.content != "" {
				writeReadinessFixture(t, ws, c.dir, c.id, c.content)
			}
			id := c.id
			if c.queryID != "" {
				id = c.queryID
			}
			ok, reason, err := ShipmentPredecessorShipped(context.Background(), ws, id)
			require.NoError(t, err, "missing or ambiguous provenance is a reason, not an error")
			assert.Equal(t, c.wantOK, ok)
			if c.wantOK {
				assert.Empty(t, reason)
			} else {
				assert.Equal(t, c.wantReason, reason)
			}
		})
	}

	t.Run("cancelled context is an infrastructure error", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		writeReadinessFixture(t, ws, "queue", "910-S", readinessShipmentMarkdown("910-S", "status: shipped\n"))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ok, _, err := ShipmentPredecessorShipped(ctx, ws, "910-S")
		require.Error(t, err)
		assert.False(t, ok, "a cancelled context must never report shipped")
	})
}
