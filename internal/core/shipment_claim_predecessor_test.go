package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// claimPredecessorFixture builds a temp workspace with a queued shipment that
// has one queued member and a hand-written dependency edge onto predecessorID.
func claimPredecessorFixture(t *testing.T, predecessorID, edgeType string) (*Workspace, *models.Artifact, string) {
	t.Helper()
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	feature, err := CreateArtifact(ctx, ws, "Claim guard feature", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "Claim guard member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Claim guard shipment", []string{member.ID})
	require.NoError(t, err)
	require.Equal(t, models.StatusQueued, shipment.Status)

	path, err := FindArtifactPath(ctx, ws, shipment.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	text := strings.ReplaceAll(string(raw), "\r\n", "\n")
	require.True(t, strings.HasPrefix(text, "---\n"))
	// An explicit empty type is rejected at parse time, so the empty-type
	// shape is hand-written as a map edge that omits the type key.
	edge := "---\ndependencies:\n    - id: " + predecessorID + "\n"
	if edgeType != "" {
		edge += "      type: " + edgeType + "\n"
	}
	text = edge + strings.TrimPrefix(text, "---\n")
	require.NoError(t, os.WriteFile(path, []byte(text), 0o644))
	return ws, shipment, member.ID
}

func assertNoClaimArtifacts(t *testing.T, ws *Workspace) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(workspaceStorageRoot(ws), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		for _, bad := range []string{"shipment-operation", "preimage", "reconcile", "intent"} {
			assert.NotContains(t, name, bad, "unexpected claim artifact %s", p)
		}
		if strings.HasSuffix(name, ".md") {
			data, readErr := os.ReadFile(p)
			require.NoError(t, readErr)
			assert.NotContains(t, string(data), "scheduler_baseline_claim", "claim marker written in %s", p)
		}
		return nil
	}))
}

func TestClaimShipmentPredecessorGuard(t *testing.T) {
	ctx := context.Background()

	refuse := func(t *testing.T, ws *Workspace, shipmentID, memberID, predecessorID, reasonWant string) {
		t.Helper()
		_, err := ClaimShipment(ctx, ws, shipmentID)
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrShipmentPredecessorNotShipped)
		assert.Contains(t, err.Error(), "shipment_predecessor_not_shipped")
		assert.Contains(t, err.Error(), predecessorID)
		if reasonWant != "" {
			assert.Contains(t, err.Error(), reasonWant)
		}
		got, getErr := GetShipment(ctx, ws, shipmentID)
		require.NoError(t, getErr)
		assert.Equal(t, models.StatusQueued, got.Status)
		member, findErr := findArtifact(ctx, ws, memberID)
		require.NoError(t, findErr)
		assert.Equal(t, models.StatusQueued, member.Status)
		assertNoClaimArtifacts(t, ws)
	}

	t.Run("refuses blocks edge onto queued shipment", func(t *testing.T) {
		ws, shipment, memberID := claimPredecessorFixture(t, "169-S", "blocks")
		writeReadinessFixture(t, ws, "queue", "169-S", readinessShipmentMarkdown("169-S", "status: queued\n"))
		refuse(t, ws, shipment.ID, memberID, "169-S", "not_shipped:queued")
	})

	t.Run("refuses empty edge type as blocks", func(t *testing.T) {
		ws, shipment, memberID := claimPredecessorFixture(t, "169-S", "")
		writeReadinessFixture(t, ws, "queue", "169-S", readinessShipmentMarkdown("169-S", "status: queued\n"))
		refuse(t, ws, shipment.ID, memberID, "169-S", "not_shipped:queued")
	})

	t.Run("fail closed archived abandoned", func(t *testing.T) {
		ws, shipment, memberID := claimPredecessorFixture(t, "170-S", "blocks")
		writeReadinessFixture(t, ws, "archive", "170-S", readinessShipmentMarkdown("170-S", "status: archived\narchived_status: abandoned\n"))
		refuse(t, ws, shipment.ID, memberID, "170-S", "")
	})

	t.Run("fail closed unresolvable predecessor", func(t *testing.T) {
		ws, shipment, memberID := claimPredecessorFixture(t, "999-S", "blocks")
		refuse(t, ws, shipment.ID, memberID, "999-S", "provenance_missing")
	})

	t.Run("fail closed unparsable predecessor", func(t *testing.T) {
		// An unparsable shipment file in any canonical directory is rejected by
		// the active-slot scan before the predecessor guard runs, so the
		// refusal carries the parse error rather than the sentinel. The claim
		// must still fail closed and leave no claim state behind.
		ws, shipment, memberID := claimPredecessorFixture(t, "171-S", "blocks")
		writeReadinessFixture(t, ws, "archive", "171-S", "---\nid: 171-S\nstatus: [unterminated\n---\n")
		_, err := ClaimShipment(ctx, ws, shipment.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "171-S")
		got, getErr := GetShipment(ctx, ws, shipment.ID)
		require.NoError(t, getErr)
		assert.Equal(t, models.StatusQueued, got.Status)
		member, findErr := findArtifact(ctx, ws, memberID)
		require.NoError(t, findErr)
		assert.Equal(t, models.StatusQueued, member.Status)
		assertNoClaimArtifacts(t, ws)
	})
	t.Run("allows archived shipped predecessor", func(t *testing.T) {
		ws, shipment, _ := claimPredecessorFixture(t, "172-S", "blocks")
		writeReadinessFixture(t, ws, "archive", "172-S", readinessShipmentMarkdown("172-S", "status: archived\narchived_status: shipped\n"))
		claimed, err := ClaimShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusActive, claimed.Status)
	})

	t.Run("allows blocks edge onto non-shipment", func(t *testing.T) {
		ws, shipment, _ := claimPredecessorFixture(t, "173-T", "blocks")
		writeReadinessFixture(t, ws, "queue", "173-T", "---\nid: 173-T\nartifact_type: task\ntitle: Fixture task\nstatus: queued\n---\n\nbody\n")
		claimed, err := ClaimShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusActive, claimed.Status)
	})
}
