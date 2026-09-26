package core

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

const (
	u20c1BlockedReason       = "waiting for upstream review"
	u20c1BlockedBy           = "reviewer"
	u20c1ResumeCheckpointRef = "checkpoints/review.json"
)

type u20c1EnvelopeValue struct {
	present bool
	value   any
}

func setupU20C1BlockedShipment(
	t *testing.T,
	ws *Workspace,
) (*models.Artifact, map[string]u20c1EnvelopeValue) {
	t.Helper()

	fixture := newURBlockedActiveFixture(t, ws)
	blocked, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{
		Reason:              u20c1BlockedReason,
		BlockedBy:           u20c1BlockedBy,
		ResumeCheckpointRef: u20c1ResumeCheckpointRef,
	})
	require.NoError(t, err)
	require.Equal(t, models.StatusBlocked, blocked.Status)
	require.NotEmpty(t, NormalizeShipmentItems(blocked), "canonical blocked fixture must have members")

	for _, key := range []string{
		"blocked_reason",
		"blocked_at",
		"member_status_snapshot",
		"blocked_by",
		"resume_checkpoint_ref",
	} {
		value, present := blocked.CustomFields[key]
		require.True(t, present, "canonical blocked envelope must contain %q", key)
		require.NotEmpty(t, value, "canonical blocked envelope field %q must be non-empty", key)
	}
	require.NotContains(t, blocked.CustomFields, "branch")

	snapshot := snapshotU20C1Envelope(t, blocked.CustomFields)
	_, err = validatePersistedBlockedShipmentEnvelope(context.Background(), ws, blocked)
	require.NoError(t, err, "fixture must have a valid persisted blocked envelope")

	return blocked, snapshot
}

func snapshotU20C1Envelope(
	t *testing.T,
	customFields map[string]any,
) map[string]u20c1EnvelopeValue {
	t.Helper()

	keys := []string{
		"blocked_reason",
		"blocked_at",
		"member_status_snapshot",
		"branch",
		"blocked_by",
		"resume_checkpoint_ref",
	}
	snapshot := make(map[string]u20c1EnvelopeValue, len(keys))
	for _, key := range keys {
		value, present := customFields[key]
		entry := u20c1EnvelopeValue{present: present}
		if present {
			encoded, err := json.Marshal(value)
			require.NoError(t, err, "snapshot blocked envelope field %q", key)
			var cloned any
			require.NoError(t, json.Unmarshal(encoded, &cloned), "clone blocked envelope field %q", key)
			entry.value = cloned
		}
		snapshot[key] = entry
	}
	return snapshot
}

func assertU20C1EnvelopeEqual(
	t *testing.T,
	customFields map[string]any,
	want map[string]u20c1EnvelopeValue,
) {
	t.Helper()

	for key, expected := range want {
		value, present := customFields[key]
		assert.Equal(t, expected.present, present, "blocked envelope field %q presence", key)
		if present {
			assert.Equal(t, expected.value, value, "blocked envelope field %q value", key)
		}
	}
}

func readU20C1ShipmentFile(t *testing.T, ws *Workspace, shipmentID string) []byte {
	t.Helper()

	path, err := FindArtifactPath(context.Background(), ws, shipmentID)
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	return content
}

func TestU20C1_GenericUpdatePreservesBlockedEnvelope(t *testing.T) {
	ctx := context.Background()
	_, ws := setupUR3Workspace(t)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })
	blocked, envelopeSnapshot := setupU20C1BlockedShipment(t, ws)

	t.Run("forged", func(t *testing.T) {
		const (
			forgedReason = "forged replacement"
			forgedAt     = "not-the-original-blocked-at"
			forgedBy     = "forged actor"
			forgedBranch = "forged-branch"
		)
		submittedCustomFields := map[string]any{
			"items":                  blocked.CustomFields["items"],
			"custom_note":            "keep this update",
			"blocked_reason":         forgedReason,
			"blocked_at":             forgedAt,
			"blocked_by":             forgedBy,
			"member_status_snapshot": map[string]any{},
			"branch":                 forgedBranch,
		}
		baselineEvents := len(readUREvents(t, ws, blocked.ID))

		updated, err := UpdateArtifact(ctx, ws, blocked.ID, map[string]any{
			"custom_fields": submittedCustomFields,
		})
		require.NoError(t, err)
		assertU20C1EnvelopeEqual(t, updated.CustomFields, envelopeSnapshot)
		assert.Equal(t, "keep this update", updated.CustomFields["custom_note"])

		assert.Equal(t, forgedReason, submittedCustomFields["blocked_reason"], "caller reason must not be rewritten")
		assert.Equal(t, forgedAt, submittedCustomFields["blocked_at"], "caller timestamp must not be rewritten")
		assert.Equal(t, forgedBy, submittedCustomFields["blocked_by"], "caller actor must not be rewritten")
		assert.Equal(t, map[string]any{}, submittedCustomFields["member_status_snapshot"],
			"caller member snapshot must not be rewritten")
		assert.Equal(t, forgedBranch, submittedCustomFields["branch"], "caller branch must not be rewritten")

		itemEvents := readUREvents(t, ws, blocked.ID)
		assert.GreaterOrEqual(t, len(itemEvents), baselineEvents)
		var intentCorrelation string
		var committedCorrelation string
		if baselineEvents <= len(itemEvents) {
			for _, event := range itemEvents[baselineEvents:] {
				if event.EventType != "artifact_mutation" {
					continue
				}
				operation, _ := event.Delta["operation"].(string)
				if operation != "update" {
					continue
				}
				correlationID, _ := event.Delta["correlation_id"].(string)
				phase, _ := event.Delta["phase"].(string)
				switch phase {
				case "intent":
					intentCorrelation = correlationID
				case "committed":
					committedCorrelation = correlationID
				}

				changes, ok := event.Delta["changes"].(map[string]any)
				assert.True(t, ok, "correlated artifact_mutation audit must include changes")
				if !ok {
					continue
				}
				auditedFields, ok := changes["custom_fields"].(map[string]any)
				assert.True(t, ok, "correlated artifact_mutation audit must include custom_fields")
				if ok {
					assertU20C1EnvelopeEqual(t, auditedFields, envelopeSnapshot)
				}
			}
		}
		assert.NotEmpty(t, intentCorrelation, "update must have a correlated artifact_mutation intent")
		assert.Equal(t, intentCorrelation, committedCorrelation,
			"artifact_mutation intent and commit must share a correlation ID")
	})

	t.Run("dropping items while blocked is refused without changing file bytes", func(t *testing.T) {
		before := readU20C1ShipmentFile(t, ws, blocked.ID)
		_, err := UpdateArtifact(ctx, ws, blocked.ID, map[string]any{
			"custom_fields": map[string]any{
				"custom_note": "items intentionally omitted",
			},
		})
		assert.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		assert.Equal(t, before, readU20C1ShipmentFile(t, ws, blocked.ID))
	})

	t.Run("omission round trip", func(t *testing.T) {
		persisted := loadURCanonicalArtifact(t, ws, blocked.ID)
		items, present := persisted.CustomFields["items"]
		require.True(t, present, "omission round trip must carry the existing items")
		updated, err := UpdateArtifact(ctx, ws, blocked.ID, map[string]any{
			"custom_fields": map[string]any{
				"items":         items,
				"custom_note_2": "keep the omission update",
			},
		})
		require.NoError(t, err)
		assertU20C1EnvelopeEqual(t, updated.CustomFields, envelopeSnapshot)
		assert.Equal(t, "keep the omission update", updated.CustomFields["custom_note_2"])

		_, err = validatePersistedBlockedShipmentEnvelope(ctx, ws, updated)
		assert.NoError(t, err, "omission round trip must leave a valid persisted blocked envelope")

		report, doctorErr := Doctor(ctx, ws, &DoctorOptions{
			CheckOrphans:    false,
			CheckDuplicates: false,
		})
		assert.NoError(t, doctorErr)
		if report != nil {
			for _, finding := range report.Findings {
				assert.False(
					t,
					finding.Type == FindingMalformedBlockedShipment && finding.ArtifactID == blocked.ID,
					"Doctor must not report a malformed blocked shipment: %+v",
					finding,
				)
			}
		}

		unblocked, unblockErr := UnblockShipment(ctx, ws, blocked.ID, UnblockOptions{
			Target:      ShipmentQueued,
			Confirm:     true,
			UnblockedBy: u20c1BlockedBy,
		})
		assert.NoError(t, unblockErr, "a valid omission round trip must still allow unblock")
		if unblockErr == nil {
			assert.Equal(t, models.StatusQueued, unblocked.Status)
			for _, key := range []string{"blocked_reason", "blocked_at", "member_status_snapshot", "blocked_by"} {
				assert.NotContains(t, unblocked.CustomFields, key)
			}
		}
	})

	t.Run("queued shipment branch update is allowed", func(t *testing.T) {
		feature, err := CreateArtifact(ctx, ws, "U20C1 queued control feature", "feature")
		require.NoError(t, err)
		member, err := CreateArtifact(ctx, ws, "U20C1 queued control member", "task", WithParent(feature.ID))
		require.NoError(t, err)
		queued, err := CreateShipment(ctx, ws, "U20C1 queued control shipment", []string{member.ID})
		require.NoError(t, err)
		require.Equal(t, models.StatusQueued, queued.Status)
		items, present := queued.CustomFields["items"]
		require.True(t, present)

		updated, err := UpdateArtifact(ctx, ws, queued.ID, map[string]any{
			"custom_fields": map[string]any{
				"items":  items,
				"branch": "queued-control-branch",
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "queued-control-branch", updated.CustomFields["branch"])
	})
}

func TestU20C1_UngovernedWriteCannotAlterBlockedEnvelope(t *testing.T) {
	t.Run("changed envelope is refused without changing file bytes", func(t *testing.T) {
		_, ws := setupUR3Workspace(t)
		t.Cleanup(func() { require.NoError(t, ws.Close()) })
		blocked, _ := setupU20C1BlockedShipment(t, ws)
		mutated := cloneArtifact(loadURCanonicalArtifact(t, ws, blocked.ID))
		mutated.CustomFields["blocked_reason"] = "ungoverned replacement"
		delete(mutated.CustomFields, "member_status_snapshot")
		assert.Equal(t, blocked.CustomFields["items"], mutated.CustomFields["items"],
			"the ungoverned mutation must leave shipment membership unchanged")

		before := readU20C1ShipmentFile(t, ws, blocked.ID)
		err := persistArtifact(context.Background(), ws, mutated, false)
		assert.ErrorIs(t, err, blerrors.ErrShipmentBlockedRequiresEnvelope)
		assert.Equal(t, before, readU20C1ShipmentFile(t, ws, blocked.ID))
	})

	t.Run("identical envelope title update is allowed", func(t *testing.T) {
		_, ws := setupUR3Workspace(t)
		t.Cleanup(func() { require.NoError(t, ws.Close()) })
		blocked, envelopeSnapshot := setupU20C1BlockedShipment(t, ws)
		mutated := cloneArtifact(loadURCanonicalArtifact(t, ws, blocked.ID))
		mutated.Title = "U20C1 ungoverned title control"

		before := readU20C1ShipmentFile(t, ws, blocked.ID)
		err := persistArtifact(context.Background(), ws, mutated, false)
		require.NoError(t, err, "an ungoverned rewrite with an identical envelope may change title")
		after := readU20C1ShipmentFile(t, ws, blocked.ID)
		assert.NotEqual(t, before, after, "title update must be persisted")
		persisted := loadURCanonicalArtifact(t, ws, blocked.ID)
		assert.Equal(t, "U20C1 ungoverned title control", persisted.Title)
		assertU20C1EnvelopeEqual(t, persisted.CustomFields, envelopeSnapshot)
		_, err = validatePersistedBlockedShipmentEnvelope(context.Background(), ws, persisted)
		assert.NoError(t, err)
	})

	t.Run("unparsable blocked_at change is refused without changing file bytes", func(t *testing.T) {
		_, ws := setupUR3Workspace(t)
		t.Cleanup(func() { require.NoError(t, ws.Close()) })
		blocked, _ := setupU20C1BlockedShipment(t, ws)
		mutated := cloneArtifact(loadURCanonicalArtifact(t, ws, blocked.ID))
		mutated.CustomFields["blocked_at"] = "not-a-timestamp"

		before := readU20C1ShipmentFile(t, ws, blocked.ID)
		err := persistArtifact(context.Background(), ws, mutated, false)
		assert.ErrorIs(t, err, blerrors.ErrShipmentBlockedRequiresEnvelope)
		assert.Equal(t, before, readU20C1ShipmentFile(t, ws, blocked.ID))
	})
}
