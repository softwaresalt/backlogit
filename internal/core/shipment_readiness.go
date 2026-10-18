package core

import (
	"context"
	"errors"
	"fmt"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// Reason codes returned by ShipmentPredecessorShipped when the predecessor is
// not ready. They are stable strings suitable for operator-facing messages.
const (
	readinessReasonProvenanceMissing = "provenance_missing"
	readinessReasonNotShipment       = "not_shipment"
	readinessReasonNotShippedPrefix  = "not_shipped:"
)

// ShipmentPredecessorShipped reports whether the shipment predecessor id has
// shipped. Provenance is read from Markdown via findArtifact, never from the
// SQLite index, because the index does not project archived_status.
//
// A predecessor counts as shipped only through an explicit allowlist: live
// status "shipped", or status "archived" whose Markdown archived_status is
// "shipped". Every other state returns false with a reason and a nil error. A
// missing or unparsable predecessor returns "provenance_missing"; a non-shipment
// artifact returns "not_shipment". Only infrastructure failures (including a
// cancelled context) return a non-nil error, and never alongside true.
func ShipmentPredecessorShipped(ctx context.Context, ws *Workspace, id string) (bool, string, error) {
	if err := ctx.Err(); err != nil {
		return false, "", fmt.Errorf("shipment predecessor %s: %w", id, err)
	}

	artifact, err := findArtifact(ctx, ws, id)
	if err != nil {
		if errors.Is(err, blerrors.ErrNotFound) {
			return false, readinessReasonProvenanceMissing, nil
		}
		return false, "", fmt.Errorf("shipment predecessor %s: load provenance: %w", id, err)
	}

	if artifact.ArtifactType != "shipment" {
		return false, readinessReasonNotShipment, nil
	}

	switch artifact.Status {
	case models.StatusShipped:
		return true, "", nil
	case models.StatusArchived:
		if artifact.ArchivedStatus == string(models.StatusShipped) {
			return true, "", nil
		}
		archivedStatus := artifact.ArchivedStatus
		if archivedStatus == "" {
			archivedStatus = "unknown"
		}
		return false, readinessReasonNotShippedPrefix + string(artifact.Status) + "/" + archivedStatus, nil
	default:
		return false, readinessReasonNotShippedPrefix + string(artifact.Status), nil
	}
}
