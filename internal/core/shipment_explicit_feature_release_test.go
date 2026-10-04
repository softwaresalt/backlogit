package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestUSR4_ShipShipmentActiveExplicitFeatureWithPreArchivedTasksIsReleased(t *testing.T) {
	type releaseFixture struct {
		ctx      context.Context
		ws       *Workspace
		feature  *models.Artifact
		taskOne  *models.Artifact
		taskTwo  *models.Artifact
		shipment *models.Artifact
	}

	prepareFixture := func(t *testing.T) releaseFixture {
		t.Helper()

		ctx := context.Background()
		ws := setupShipmentWorkspace(t)

		feature, err := CreateArtifact(ctx, ws, "Explicit feature release", "feature")
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

		taskOne, err := CreateArtifact(ctx, ws, "Explicit feature task one", "task", WithParent(feature.ID))
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskOne))

		taskTwo, err := CreateArtifact(ctx, ws, "Explicit feature task two", "task", WithParent(feature.ID))
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskTwo))

		shipment, err := CreateShipment(ctx, ws, "Explicit feature shipment", []string{feature.ID, taskOne.ID, taskTwo.ID})
		require.NoError(t, err)
		_, err = ClaimShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)

		for _, task := range []*models.Artifact{taskOne, taskTwo} {
			_, err = UpdateArtifact(ctx, ws, task.ID, map[string]any{"status": string(models.StatusDone)})
			require.NoError(t, err)
			_, err = ArchiveItem(ctx, ws.DB, ws, task.ID)
			require.NoError(t, err)

			archivedTask, err := findArtifact(ctx, ws, task.ID)
			require.NoError(t, err)
			require.Equal(t, models.StatusArchived, archivedTask.Status)
			require.Equal(t, string(models.StatusDone), archivedTask.ArchivedStatus)
		}

		activeFeature, err := loadArtifact(ctx, ws, feature.ID)
		require.NoError(t, err)
		require.Equal(t, models.StatusActive, activeFeature.Status)

		return releaseFixture{
			ctx:      ctx,
			ws:       ws,
			feature:  feature,
			taskOne:  taskOne,
			taskTwo:  taskTwo,
			shipment: shipment,
		}
	}

	tests := []struct {
		name string
		run  func(*testing.T, releaseFixture)
	}{
		{
			name: "CompletionAndArchival",
			run: func(t *testing.T, fixture releaseFixture) {
				result, err := ShipShipment(fixture.ctx, fixture.ws, fixture.shipment.ID, nil)
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Contains(t, result.ArchivedIDs, fixture.feature.ID)
				assert.Contains(t, result.ArchivedIDs, fixture.shipment.ID)
				assert.NotContains(t, result.ArchivedIDs, fixture.taskOne.ID)
				assert.NotContains(t, result.ArchivedIDs, fixture.taskTwo.ID)

				archivedShipment, err := findArtifact(fixture.ctx, fixture.ws, fixture.shipment.ID)
				require.NoError(t, err)
				assert.Equal(t, models.StatusArchived, archivedShipment.Status)
				assert.Equal(t, string(ShipmentShipped), archivedShipment.ArchivedStatus)

				archivedFeature, err := loadArtifact(fixture.ctx, fixture.ws, fixture.feature.ID)
				require.NoError(t, err)
				assert.Equal(t, models.StatusArchived, archivedFeature.Status)
				archivedFeatureFile, err := findArtifact(fixture.ctx, fixture.ws, fixture.feature.ID)
				require.NoError(t, err)
				assert.Equal(t, string(models.StatusDone), archivedFeatureFile.ArchivedStatus)

				for _, taskID := range []string{fixture.taskOne.ID, fixture.taskTwo.ID} {
					archivedTask, err := findArtifact(fixture.ctx, fixture.ws, taskID)
					require.NoError(t, err)
					assert.Equal(t, models.StatusArchived, archivedTask.Status)
					assert.Equal(t, string(models.StatusDone), archivedTask.ArchivedStatus)
				}
			},
		},
		{
			name: "GovernedStatusTransition",
			run: func(t *testing.T, fixture releaseFixture) {
				_, err := ShipShipment(fixture.ctx, fixture.ws, fixture.shipment.ID, nil)
				require.NoError(t, err)

				featureEvents, err := events.ReadAllEvents(fixture.ctx, WorkspaceLogsRoot(fixture.ws.RootPath), fixture.feature.ID)
				require.NoError(t, err)

				var releaseEvent *events.Event
				for i := range featureEvents {
					event := &featureEvents[i]
					if event.EventType != "status_changed" {
						continue
					}
					reason, ok := event.Delta["reason"].(string)
					if ok && (reason == "shipment released" || reason == "feature released") {
						releaseEvent = event
						break
					}
				}

				require.NotNil(t, releaseEvent, "feature release status event must be durable")
				assert.Equal(t, string(models.StatusDone), releaseEvent.Delta["to"])
				assert.Contains(t, []string{"shipment released", "feature released"}, releaseEvent.Delta["reason"])
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := prepareFixture(t)
			test.run(t, fixture)
		})
	}
}
