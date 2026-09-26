package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU20C2_UnblockStatusEvidencePrecedesEnvelopeClear(t *testing.T) {
	const (
		reason   = "waiting for upstream review"
		actor    = "reviewer"
		checkRef = "checkpoints/review.json"
	)

	targets := []struct {
		name   string
		target ShipmentStatus
	}{
		{name: "queued", target: ShipmentQueued},
		{name: "active", target: ShipmentActive},
	}

	for _, targetCase := range targets {
		t.Run(targetCase.name, func(t *testing.T) {
			ctx := context.Background()
			root, ws := setupUR3Workspace(t)
			t.Cleanup(func() { require.NoError(t, ws.Close()) })

			fixture := newURBlockedActiveFixture(t, ws)
			blocked, err := BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
				Reason:              reason,
				BlockedBy:           actor,
				ResumeCheckpointRef: checkRef,
			})
			require.NoError(t, err)

			firstShipmentWriteObserved := false
			observerErr := error(nil)
			reasonPresentAtFirstWrite := false
			journalCountAtFirstWrite := 0
			journalPhaseAtFirstWrite := ""
			correlationID := ""
			appliedEventCountAtFirstWrite := 0
			var appliedEventAtFirstWrite *events.Event

			previousHook := persistArtifactPreLockHook
			persistArtifactPreLockHook = func(artifactID string) {
				if artifactID != blocked.ID || firstShipmentWriteObserved {
					return
				}
				firstShipmentWriteObserved = true

				current, loadErr := findArtifact(ctx, ws, blocked.ID)
				if loadErr != nil {
					observerErr = loadErr
					return
				}
				reasonPresentAtFirstWrite = current.CustomFields["blocked_reason"] == reason

				journalEntries, readErr := os.ReadDir(shipmentOpsRoot(root))
				if readErr != nil {
					observerErr = readErr
					return
				}
				for _, entry := range journalEntries {
					if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
						continue
					}
					journalBytes, fileErr := os.ReadFile(filepath.Join(shipmentOpsRoot(root), entry.Name()))
					if fileErr != nil {
						observerErr = fileErr
						return
					}
					var journal shipmentLifecycleJournal
					if unmarshalErr := json.Unmarshal(journalBytes, &journal); unmarshalErr != nil {
						observerErr = unmarshalErr
						return
					}
					if journal.Operation != "unblock" ||
						journal.ShipmentID != blocked.ID ||
						journal.Target != string(targetCase.target) {
						continue
					}
					journalCountAtFirstWrite++
					correlationID = journal.CorrelationID
					journalPhaseAtFirstWrite = journal.Phase
				}

				itemEvents, eventsErr := events.ReadAllEvents(ctx, WorkspaceLogsRoot(root), blocked.ID)
				if eventsErr != nil {
					observerErr = eventsErr
					return
				}
				for _, event := range itemEvents {
					eventCorrelation, _ := event.Delta["correlation_id"].(string)
					eventPhase, _ := event.Delta["phase"].(string)
					eventOperation, _ := event.Delta["operation"].(string)
					if event.EventType != "shipment_status_changed" ||
						eventCorrelation != correlationID ||
						eventPhase != "applied" ||
						eventOperation != "unblock" {
						continue
					}
					appliedEventCountAtFirstWrite++
					copiedEvent := event
					appliedEventAtFirstWrite = &copiedEvent
				}
			}
			t.Cleanup(func() { persistArtifactPreLockHook = previousHook })

			unblocked, err := UnblockShipment(ctx, ws, blocked.ID, UnblockOptions{
				Target:      targetCase.target,
				Confirm:     true,
				UnblockedBy: actor,
			})
			require.NoError(t, err)
			require.NotNil(t, unblocked)

			assert.NoError(t, observerErr, "the first shipment-write observer must complete")
			assert.True(t, firstShipmentWriteObserved, "the observer must run at the first shipment write")
			assert.True(t, reasonPresentAtFirstWrite,
				"blocked_reason must still be present on disk at the first shipment write")
			assert.Equal(t, 1, journalCountAtFirstWrite,
				"the first shipment write must have one matching unblock journal")
			assert.Equal(t, "intent", journalPhaseAtFirstWrite)
			assert.NotEmpty(t, correlationID, "the unblock journal must provide its correlation ID")
			assert.Equal(t, 1, appliedEventCountAtFirstWrite,
				"exactly one correlated applied shipment_status_changed event must precede the first shipment write")
			if appliedEventAtFirstWrite != nil {
				assert.Equal(t, string(targetCase.target), appliedEventAtFirstWrite.Delta["target"])
				assert.Equal(t, string(targetCase.target), appliedEventAtFirstWrite.Delta["status"])
				assert.Equal(t, reason, appliedEventAtFirstWrite.Delta["reason"])
				assert.Equal(t, actor, appliedEventAtFirstWrite.Delta["unblocked_by"])
				assert.Equal(t, checkRef, appliedEventAtFirstWrite.Delta["resume_checkpoint_ref"])
				assert.Equal(t, actor, appliedEventAtFirstWrite.Actor)
			}

			finalShipment, err := findArtifact(ctx, ws, blocked.ID)
			require.NoError(t, err)
			assert.Equal(t, models.ArtifactStatus(targetCase.target), finalShipment.Status)
			for key := range finalShipment.CustomFields {
				if strings.HasPrefix(key, "blocked_") {
					assert.Fail(t, "blocked metadata must be cleared after unblock", "unexpected key %q", key)
				}
			}

			journalPath := filepath.Join(shipmentOpsRoot(root), shipmentLifecycleJournalName(correlationID))
			journalBytes, err := os.ReadFile(journalPath)
			require.NoError(t, err)
			var committedJournal shipmentLifecycleJournal
			require.NoError(t, json.Unmarshal(journalBytes, &committedJournal))
			assert.Equal(t, "committed", committedJournal.Phase)

			itemEvents, err := events.ReadAllEvents(ctx, WorkspaceLogsRoot(root), blocked.ID)
			require.NoError(t, err)
			appliedEventCount := 0
			committedEventCount := 0
			var appliedStatusEvent *events.Event
			for _, event := range itemEvents {
				eventCorrelation, _ := event.Delta["correlation_id"].(string)
				eventOperation, _ := event.Delta["operation"].(string)
				eventPhase, _ := event.Delta["phase"].(string)
				if eventCorrelation != correlationID || eventOperation != "unblock" {
					continue
				}
				switch {
				case event.EventType == "shipment_status_changed" && eventPhase == "applied":
					appliedEventCount++
					copiedEvent := event
					appliedStatusEvent = &copiedEvent
				case event.EventType == "shipment_lifecycle" && eventPhase == "committed":
					committedEventCount++
				}
			}
			assert.Equal(t, 1, appliedEventCount, "exactly one correlated applied status event must be durable")
			assert.Equal(t, 1, committedEventCount, "exactly one correlated terminal committed event must be durable")
			if appliedStatusEvent != nil {
				assert.Equal(t, string(targetCase.target), appliedStatusEvent.Delta["target"])
				assert.Equal(t, string(targetCase.target), appliedStatusEvent.Delta["status"])
				assert.Equal(t, reason, appliedStatusEvent.Delta["reason"])
				assert.Equal(t, actor, appliedStatusEvent.Delta["unblocked_by"])
				assert.Equal(t, checkRef, appliedStatusEvent.Delta["resume_checkpoint_ref"])
				assert.Equal(t, actor, appliedStatusEvent.Actor)
			}
		})
	}
}
