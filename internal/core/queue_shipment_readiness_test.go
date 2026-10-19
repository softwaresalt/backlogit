package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

var errReadinessResolverBoom = errors.New("resolver boom")

type readinessEdge struct {
	itemType string
	predType string
	predStat models.ArtifactStatus
	depType  string
}

// seedReadinessEdge indexes a queued dependent and a predecessor connected by
// one dependency edge and returns the dependent ID.
func seedReadinessEdge(t *testing.T, ws *Workspace, e readinessEdge) string {
	t.Helper()
	ctx := context.Background()
	itemID, predID := "950-S", "951-S"
	if e.itemType == "task" {
		itemID = "950-T"
	}
	if e.predType == "task" {
		predID = "951-T"
	}
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, &models.Artifact{ID: itemID, Title: "Dependent", Status: models.StatusQueued, ArtifactType: e.itemType}))
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, &models.Artifact{ID: predID, Title: "Predecessor", Status: e.predStat, ArtifactType: e.predType}))
	require.NoError(t, bldb.UpsertDependency(ctx, ws.DB, itemID, predID, e.depType))
	return itemID
}

func queueContains(view *QueueView, id string) bool {
	for _, item := range view.Items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func readinessResolver(shipped bool, err error) func(context.Context, string) (bool, string, error) {
	return func(context.Context, string) (bool, string, error) {
		return shipped, "", err
	}
}

// TestQueueShipmentReadiness pins shipment predecessor readiness filtering in
// QueryQueue (A-U5 / 188.001-T).
func TestQueueShipmentReadiness(t *testing.T) {
	ctx := context.Background()

	run := func(t *testing.T, e readinessEdge, resolver func(context.Context, string) (bool, string, error), itemType string) (*QueueView, string, error) {
		t.Helper()
		ws := setupShipmentWorkspace(t)
		id := seedReadinessEdge(t, ws, e)
		view, err := QueryQueue(ctx, ws.DB, &QueueFilter{
			Types:                       []string{itemType},
			Statuses:                    []string{"queued"},
			shipmentPredecessorResolver: resolver,
		})
		return view, id, err
	}

	t.Run("group1 shipment blocks edge", func(t *testing.T) {
		cases := []struct {
			name     string
			pred     models.ArtifactStatus
			resolver func(context.Context, string) (bool, string, error)
			want     bool
		}{
			{"abandoned predecessor excluded", models.StatusAbandoned, readinessResolver(false, nil), false},
			{"archived non-shipped predecessor excluded", models.StatusArchived, readinessResolver(false, nil), false},
			{"archived resolver-shipped predecessor included", models.StatusArchived, readinessResolver(true, nil), true},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				view, id, err := run(t, readinessEdge{"shipment", "shipment", c.pred, "blocks"}, c.resolver, "shipment")
				require.NoError(t, err)
				assert.Equal(t, c.want, queueContains(view, id))
			})
		}
	})

	t.Run("group2 non-shipment semantics unchanged", func(t *testing.T) {
		cases := []struct {
			name string
			edge readinessEdge
			typ  string
		}{
			{"task blocked on abandoned task released", readinessEdge{"task", "task", models.StatusAbandoned, "blocks"}, "task"},
			{"shipment blocks onto abandoned task keeps cascade", readinessEdge{"shipment", "task", models.StatusAbandoned, "blocks"}, "shipment"},
			{"shipment relates_to abandoned shipment keeps cascade", readinessEdge{"shipment", "shipment", models.StatusAbandoned, "relates_to"}, "shipment"},
			{"shipment parent_of abandoned shipment keeps cascade", readinessEdge{"shipment", "shipment", models.StatusAbandoned, "parent_of"}, "shipment"},
		}
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				view, id, err := run(t, c.edge, readinessResolver(false, nil), c.typ)
				require.NoError(t, err)
				assert.True(t, queueContains(view, id))
			})
		}
	})

	t.Run("group3 without resolver and error propagation", func(t *testing.T) {
		t.Run("archived shipment predecessor stays blocking", func(t *testing.T) {
			view, id, err := run(t, readinessEdge{"shipment", "shipment", models.StatusArchived, "blocks"}, nil, "shipment")
			require.NoError(t, err)
			assert.False(t, queueContains(view, id))
		})
		t.Run("live shipped predecessor releases", func(t *testing.T) {
			view, id, err := run(t, readinessEdge{"shipment", "shipment", models.StatusShipped, "blocks"}, nil, "shipment")
			require.NoError(t, err)
			assert.True(t, queueContains(view, id))
		})
		t.Run("resolver error propagates wrapped", func(t *testing.T) {
			_, _, err := run(t, readinessEdge{"shipment", "shipment", models.StatusArchived, "blocks"}, readinessResolver(false, errReadinessResolverBoom), "shipment")
			require.Error(t, err)
			assert.ErrorIs(t, err, errReadinessResolverBoom)
		})
	})
}
