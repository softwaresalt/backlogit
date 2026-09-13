package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// 167.002-T (U3) behavior harness: the shipment_reconciled_shipped event
// type, its delta schema, the shared full-event validator (raw token-level
// duplicate-member rejection BEFORE canonicalization), and doctor-local
// recognition. This unit does NOT append the event (167.007-T does); these
// tests exercise the schema/validator/recognition surface only.

func validReconciledShippedDelta() ShipmentReconciledShippedDelta {
	return ShipmentReconciledShippedDelta{
		Before:                ShipmentReconciledShippedBefore{Status: "archived", ArchivedStatus: "active"},
		After:                 ShipmentReconciledShippedAfter{ArchivedStatus: "shipped"},
		Reason:                "governed repair",
		Actor:                 "operator",
		SecondApprover:        "",
		IdempotencyKey:        "idem-001",
		RequestIdentityDigest: "abc123",
		TrustedRefName:        "main",
		TrustedRefTip:         "deadbeef",
		Evidence: ShipmentReconciledShippedEvidence{
			MergeSHA:           "deadbeef",
			ClosureEvidence:    "closure notes",
			ManifestDigest:     "manifestdigest",
			MemberTerminalIDs:  []string{"001-F"},
			ClosureContentHash: "contenthash",
			EvidenceRefs:       []string{},
			EvidenceDigest:     "evdigest",
		},
	}
}

func marshalReconciledShippedEvent(t *testing.T, delta ShipmentReconciledShippedDelta, itemID string) []byte {
	t.Helper()
	deltaBytes, err := json.Marshal(delta)
	require.NoError(t, err)
	var deltaMap map[string]any
	require.NoError(t, json.Unmarshal(deltaBytes, &deltaMap))
	event := events.Event{
		Timestamp: time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: EventShipmentReconciledShipped,
		Delta:     deltaMap,
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)
	return append(raw, '\n')
}

func TestEventShipmentReconciledShipped_Constant(t *testing.T) {
	assert.Equal(t, "shipment_reconciled_shipped", EventShipmentReconciledShipped)
}

func TestValidateShipmentReconciledShippedEvent_AcceptsWellFormedEvent(t *testing.T) {
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")
	err := ValidateShipmentReconciledShippedEvent(raw, nil, "", "")
	require.NoError(t, err)
}

// TestValidateShipmentReconciledShippedEvent_ShipmentIDBinding covers
// PR #440 review finding 3/6: an otherwise well-formed event naming a
// DIFFERENT shipment than expectedShipmentID must be rejected, while a
// blank expectedShipmentID (no specific identity to bind against) and a
// matching expectedShipmentID both pass.
func TestValidateShipmentReconciledShippedEvent_ShipmentIDBinding(t *testing.T) {
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")

	require.NoError(t, ValidateShipmentReconciledShippedEvent(raw, nil, "", "001-S"), "matching expected shipment id must pass")
	require.NoError(t, ValidateShipmentReconciledShippedEvent(raw, nil, "", ""), "blank expected shipment id must skip the identity check")

	err := ValidateShipmentReconciledShippedEvent(raw, nil, "", "047-S")
	require.Error(t, err, "an event naming a different shipment than expected must be rejected")
}

func TestValidateShipmentReconciledShippedEvent_RejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*ShipmentReconciledShippedDelta)
	}{
		{"empty_reason", func(d *ShipmentReconciledShippedDelta) { d.Reason = "" }},
		{"empty_actor", func(d *ShipmentReconciledShippedDelta) { d.Actor = "" }},
		{"empty_idempotency_key", func(d *ShipmentReconciledShippedDelta) { d.IdempotencyKey = "" }},
		{"empty_request_identity_digest", func(d *ShipmentReconciledShippedDelta) { d.RequestIdentityDigest = "" }},
		{"empty_evidence_digest", func(d *ShipmentReconciledShippedDelta) { d.Evidence.EvidenceDigest = "" }},
		{"wrong_after_archived_status", func(d *ShipmentReconciledShippedDelta) { d.After.ArchivedStatus = "active" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			delta := validReconciledShippedDelta()
			tc.mutate(&delta)
			raw := marshalReconciledShippedEvent(t, delta, "001-S")
			err := ValidateShipmentReconciledShippedEvent(raw, nil, "", "")
			assert.Error(t, err, "case %s must be rejected", tc.name)
		})
	}
}

func TestValidateShipmentReconciledShippedEvent_RejectsWrongEventType(t *testing.T) {
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    "001-S",
		EventType: "shipment_status_changed",
		Delta:     map[string]any{"status": "shipped"},
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)
	raw = append(raw, '\n')
	assert.Error(t, ValidateShipmentReconciledShippedEvent(raw, nil, "", ""))
}

func TestValidateShipmentReconciledShippedEvent_RejectsNonNewlineTerminated(t *testing.T) {
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")
	raw = bytes.TrimSuffix(raw, []byte("\n"))
	assert.Error(t, ValidateShipmentReconciledShippedEvent(raw, nil, "", ""))
}

func TestValidateShipmentReconciledShippedEvent_RejectsDuplicateTopLevelMember(t *testing.T) {
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")
	// Inject a duplicate "actor" member at the top level by hand: encoding/json
	// would silently keep the last value on a map decode, so this must be
	// caught by the raw token-level scan BEFORE any canonicalization.
	tampered := bytes.Replace(raw, []byte(`"actor":"backlogit"`), []byte(`"actor":"backlogit","actor":"forged"`), 1)
	require.NotEqual(t, string(raw), string(tampered), "the fixture must actually contain the literal being replaced")
	err := ValidateShipmentReconciledShippedEvent(tampered, nil, "", "")
	require.Error(t, err)
}

func TestValidateShipmentReconciledShippedEvent_RejectsDuplicateNestedDeltaMember(t *testing.T) {
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")
	tampered := bytes.Replace(raw, []byte(`"reason":"governed repair"`), []byte(`"reason":"governed repair","reason":"forged"`), 1)
	require.NotEqual(t, string(raw), string(tampered))
	err := ValidateShipmentReconciledShippedEvent(tampered, nil, "", "")
	require.Error(t, err)
}

func TestValidateShipmentReconciledShippedEvent_ComparesAgainstPreparedEventAndDigest(t *testing.T) {
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")
	digest := ShipmentReconciledShippedEventDigest(raw)

	require.NoError(t, ValidateShipmentReconciledShippedEvent(raw, raw, digest, ""))

	t.Run("mutated_trusted_ref_tip_fails_digest", func(t *testing.T) {
		mutatedDelta := validReconciledShippedDelta()
		mutatedDelta.TrustedRefTip = "mutated"
		mutated := marshalReconciledShippedEvent(t, mutatedDelta, "001-S")
		err := ValidateShipmentReconciledShippedEvent(mutated, raw, digest, "")
		assert.Error(t, err, "a mutated trusted_ref_tip must fail the prepared-event/digest comparison")
	})

	t.Run("frontmatter_only_identity_digest_edit_fails", func(t *testing.T) {
		// Simulate an attacker editing ONLY the frontmatter scalar copy of
		// request_identity_digest without touching the durable event: the
		// prepared event bytes (representing the durable event) stay as
		// `raw`, but the logged bytes being validated now diverge from it.
		mutatedDelta := validReconciledShippedDelta()
		mutatedDelta.RequestIdentityDigest = "attacker-supplied"
		mutated := marshalReconciledShippedEvent(t, mutatedDelta, "001-S")
		err := ValidateShipmentReconciledShippedEvent(mutated, raw, digest, "")
		assert.Error(t, err, "a frontmatter-only request_identity_digest edit must be caught by the event-digest comparison")
	})
}

func TestDetectDuplicateJSONMembers_AcceptsWellFormed(t *testing.T) {
	require.NoError(t, detectDuplicateJSONMembers([]byte(`{"a":1,"b":{"c":2},"d":[{"e":3},{"e":4}]}`)))
}

func TestDetectDuplicateJSONMembers_RejectsTopLevelDuplicate(t *testing.T) {
	assert.Error(t, detectDuplicateJSONMembers([]byte(`{"a":1,"a":2}`)))
}

func TestDetectDuplicateJSONMembers_RejectsNestedDuplicate(t *testing.T) {
	assert.Error(t, detectDuplicateJSONMembers([]byte(`{"a":{"b":1,"b":2}}`)))
}

func TestDetectDuplicateJSONMembers_RejectsDuplicateInsideArrayElement(t *testing.T) {
	assert.Error(t, detectDuplicateJSONMembers([]byte(`{"a":[{"b":1,"b":2}]}`)))
}

func TestDetectDuplicateJSONMembers_DistinctArrayElementsAreNotDuplicates(t *testing.T) {
	require.NoError(t, detectDuplicateJSONMembers([]byte(`{"a":[{"b":1},{"b":2}]}`)))
}

func TestShipmentReconciledShippedEventDigest_Deterministic(t *testing.T) {
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")
	sum := sha256.Sum256(raw)
	want := hex.EncodeToString(sum[:])
	assert.Equal(t, want, ShipmentReconciledShippedEventDigest(raw))
}

// --- Doctor-local recognition (167.002-T) ---

func TestDetectMissingShippedEvents_SuppressedByFullyValidReconciledEvent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	itemID := "167.002-T-shipment"
	raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), itemID)
	require.NoError(t, os.WriteFile(events.LogPathForItem(logsDir, itemID), raw, 0o644))

	refs := map[string][]artifactRef{
		itemID: {{artifactType: "shipment", archivedStatus: string(ShipmentShipped)}},
	}
	findings := detectMissingShippedEvents(context.Background(), logsDir, refs, []string{itemID})
	assert.Empty(t, findings, "a fully-valid reconciled-shipped event must suppress the missing-shipped-event finding")
}

func TestDetectMissingShippedEvents_NotSuppressedByMalformedReconciledEvent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	itemID := "167.002-T-malformed"
	delta := validReconciledShippedDelta()
	delta.Evidence.EvidenceDigest = "" // malformed: required field missing
	raw := marshalReconciledShippedEvent(t, delta, itemID)
	require.NoError(t, os.WriteFile(events.LogPathForItem(logsDir, itemID), raw, 0o644))

	refs := map[string][]artifactRef{
		itemID: {{artifactType: "shipment", archivedStatus: string(ShipmentShipped)}},
	}
	findings := detectMissingShippedEvents(context.Background(), logsDir, refs, []string{itemID})
	require.Len(t, findings, 1, "a malformed reconciled-shipped event must NOT suppress the finding (144-F guard 2 unweakened)")
	assert.Equal(t, FindingMissingShippedEvent, findings[0].Type)
}

func TestDetectMissingShippedEvents_PlainShippedEventStillSuppresses(t *testing.T) {
	// Negative-guard test (144-F guard 2 not weakened): the EXISTING plain
	// shipment_status_changed/shipped recognition path continues to work
	// unchanged after this task's additions.
	ws := setupShipmentWorkspace(t)
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	itemID := "167.002-T-plain"
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: "shipment_status_changed",
		Delta:     map[string]any{"status": string(ShipmentShipped)},
	}
	raw, err := json.Marshal(event)
	require.NoError(t, err)
	raw = append(raw, '\n')
	require.NoError(t, os.WriteFile(events.LogPathForItem(logsDir, itemID), raw, 0o644))

	refs := map[string][]artifactRef{
		itemID: {{artifactType: "shipment", archivedStatus: string(ShipmentShipped)}},
	}
	findings := detectMissingShippedEvents(context.Background(), logsDir, refs, []string{itemID})
	assert.Empty(t, findings, "the plain shipped-event recognition path must be unchanged")
}

func TestDetectMissingShippedEvents_NoEventAtAllStillReportsMissing(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	itemID := "167.002-T-absent"
	refs := map[string][]artifactRef{
		itemID: {{artifactType: "shipment", archivedStatus: string(ShipmentShipped)}},
	}
	findings := detectMissingShippedEvents(context.Background(), logsDir, refs, []string{itemID})
	require.Len(t, findings, 1)
}

func TestDetectMissingShippedEvents_UnreadableLogIsPresenceUnknownNotSuppressed(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(logsDir, 0o755))

	itemID := "167.002-T-unreadable"
	// Plant a directory where the log file should be, forcing a read failure
	// distinct from "absent" -- matching the existing shippedEventPresence
	// convention (unreadable, not merely missing).
	logPath := events.LogPathForItem(logsDir, itemID)
	require.NoError(t, os.MkdirAll(logPath, 0o755))
	t.Cleanup(func() { _ = os.RemoveAll(logPath) })

	refs := map[string][]artifactRef{
		itemID: {{artifactType: "shipment", archivedStatus: string(ShipmentShipped)}},
	}
	findings := detectMissingShippedEvents(context.Background(), logsDir, refs, []string{itemID})
	require.Len(t, findings, 1, "an unreadable log must retain the finding (presence-unknown), never suppress")
	assert.Contains(t, findings[0].Description, "unreadable")
}
