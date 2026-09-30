package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU1_ClaimedQueuedMemberGetsMarkerInActivationWrite(t *testing.T) {
	tests := []struct {
		name          string
		initialMarker string
		nilFields     bool
	}{
		{name: "initialize_nil_custom_fields", nilFields: true},
		{name: "overwrite_stale_marker", initialMarker: "previous-shipment"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()

			parent, err := CreateArtifact(ctx, ws, "Marker scope parent", "feature")
			require.NoError(t, err)
			member, err := CreateArtifact(ctx, ws, "Marker queued member", "task", WithParent(parent.ID))
			require.NoError(t, err)
			if testCase.nilFields {
				member.CustomFields = nil
			} else {
				member.CustomFields = map[string]any{
					schedulerBaselineClaimKey: testCase.initialMarker,
					"preserved":               "value",
				}
			}
			require.NoError(t, persistArtifact(ctx, ws, member, false))

			shipment, err := CreateShipment(ctx, ws, "Marker claim", []string{member.ID})
			require.NoError(t, err)

			originalWrite := persistArtifactWriteFn
			memberWrites := make([]*models.Artifact, 0, 1)
			persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
				if artifact.ID == member.ID {
					memberWrites = append(memberWrites, cloneArtifact(artifact))
				}
				return originalWrite(artifact, filePath, durable)
			}
			t.Cleanup(func() { persistArtifactWriteFn = originalWrite })

			_, err = ClaimShipment(ctx, ws, shipment.ID)
			require.NoError(t, err)

			require.Len(t, memberWrites, 1, "claim activation and marker must use one persistArtifact write")
			assert.Equal(t, models.StatusActive, memberWrites[0].Status)
			assert.Equal(t, shipment.ID, memberWrites[0].CustomFields[schedulerBaselineClaimKey])
			if !testCase.nilFields {
				assert.Equal(t, "value", memberWrites[0].CustomFields["preserved"])
			}

			persistedParent, err := findArtifact(ctx, ws, parent.ID)
			require.NoError(t, err)
			assert.Equal(t, models.StatusQueued, persistedParent.Status)
			assert.NotContains(t, persistedParent.CustomFields, schedulerBaselineClaimKey,
				"a hierarchy-derived non-member parent must not receive the claim marker")
		})
	}
}

func TestU1_CascadeActivatedMemberParentGetsMarkerOnlyWrite(t *testing.T) {
	tests := []struct {
		name          string
		markerMatches bool
		wantWrites    int
	}{
		{name: "write_stale_marker", wantWrites: 2},
		{name: "same_marker_is_noop", markerMatches: true, wantWrites: 1},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()

			parent, err := CreateArtifact(ctx, ws, "Cascade member parent", "feature")
			require.NoError(t, err)
			child, err := CreateArtifact(ctx, ws, "Cascade member child", "task", WithParent(parent.ID))
			require.NoError(t, err)
			shipment, err := CreateShipment(ctx, ws, "Cascade marker claim", []string{child.ID, parent.ID})
			require.NoError(t, err)

			parent, err = findArtifact(ctx, ws, parent.ID)
			require.NoError(t, err)
			initialMarker := "previous-shipment"
			if testCase.markerMatches {
				initialMarker = shipment.ID
			}
			parent.CustomFields = map[string]any{
				schedulerBaselineClaimKey: initialMarker,
				"preserved":               "value",
			}
			require.NoError(t, persistArtifact(ctx, ws, parent, false))
			parentStatusEventsBefore := u1StatusChangedEventCount(t, ws, parent.ID)

			originalWrite := persistArtifactWriteFn
			parentWrites := make([]*models.Artifact, 0, testCase.wantWrites)
			persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
				if artifact.ID == parent.ID {
					parentWrites = append(parentWrites, cloneArtifact(artifact))
				}
				return originalWrite(artifact, filePath, durable)
			}
			t.Cleanup(func() { persistArtifactWriteFn = originalWrite })

			_, err = ClaimShipment(ctx, ws, shipment.ID)
			require.NoError(t, err)

			require.Len(t, parentWrites, testCase.wantWrites,
				"an already-active member parent needs only a marker write, unless it already has this marker")
			persistedParent, err := findArtifact(ctx, ws, parent.ID)
			require.NoError(t, err)
			assert.Equal(t, models.StatusActive, persistedParent.Status)
			assert.Equal(t, shipment.ID, persistedParent.CustomFields[schedulerBaselineClaimKey])
			assert.Equal(t, "value", persistedParent.CustomFields["preserved"])
			assert.Equal(t, parentStatusEventsBefore+1, u1StatusChangedEventCount(t, ws, parent.ID),
				"only the earlier parent status transition may emit status_changed")

			persistedChild, err := findArtifact(ctx, ws, child.ID)
			require.NoError(t, err)
			assert.Equal(t, models.StatusActive, persistedChild.Status)
			assert.Equal(t, shipment.ID, persistedChild.CustomFields[schedulerBaselineClaimKey])

			if !testCase.markerMatches && len(parentWrites) == 2 {
				assert.Equal(t, models.StatusActive, parentWrites[1].Status,
					"the marker-only write must not transition the already-active member")
				assert.Equal(t, shipment.ID, parentWrites[1].CustomFields[schedulerBaselineClaimKey])
			}
		})
	}
}

func u1StatusChangedEventCount(t *testing.T, ws *Workspace, itemID string) int {
	t.Helper()

	data, err := os.ReadFile(events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), itemID))
	if errors.Is(err, os.ErrNotExist) {
		return 0
	}
	require.NoError(t, err)

	count := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var event events.Event
		require.NoError(t, json.Unmarshal([]byte(line), &event))
		if event.EventType == "status_changed" {
			count++
		}
	}
	return count
}
