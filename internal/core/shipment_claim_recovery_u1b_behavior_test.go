package core

import (
	"testing"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU1b_ClaimRecoveryCandidatesAcceptOnlyPreimageAndJournalShipmentMarker(t *testing.T) {
	const markerKey = "scheduler_baseline_claim"
	const shipmentID = "154-S"

	tests := []struct {
		name                      string
		customFields              map[string]any
		wantPreimageMarker        any
		wantPreimageMarkerPresent bool
		wantRetainedValue         any
		wantRetainedValuePresent  bool
	}{
		{
			name: "stale preimage marker",
			customFields: map[string]any{
				markerKey:  "OLD-S",
				"retained": "preimage",
			},
			wantPreimageMarker:        "OLD-S",
			wantPreimageMarkerPresent: true,
			wantRetainedValue:         "preimage",
			wantRetainedValuePresent:  true,
		},
		{
			name: "nil preimage custom fields",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			customFields := make(map[string]any, len(test.customFields))
			for key, value := range test.customFields {
				customFields[key] = value
			}
			if test.customFields == nil {
				customFields = nil
			}

			preimage := &models.Artifact{
				ID:           "173.009-T-member",
				Status:       models.StatusQueued,
				CustomFields: customFields,
			}
			journal := shipmentLifecycleJournal{
				Operation:      "claim",
				RecoveryPolicy: "rollback",
				ShipmentID:     shipmentID,
			}

			candidates, err := memberRecoveryCandidates(journal, preimage, nil)
			if err != nil {
				t.Fatalf("memberRecoveryCandidates returned an error: %v", err)
			}
			if len(candidates) != 3 {
				t.Fatalf("claim recovery must return preimage, unmarked active, and marked active candidates; got %d", len(candidates))
			}

			checkMarker := func(candidate *models.Artifact, want any, wantPresent bool, name string) {
				t.Helper()
				if candidate == nil {
					t.Fatalf("%s candidate has no artifact", name)
				}
				got, present := candidate.CustomFields[markerKey]
				if present != wantPresent {
					t.Errorf("%s candidate marker presence = %t, want %t", name, present, wantPresent)
					return
				}
				if present && got != want {
					t.Errorf("%s candidate marker = %v, want %v", name, got, want)
				}
			}

			preimageCandidate := candidates[0].artifact
			if preimageCandidate == nil {
				t.Fatal("first recovery candidate must be the preimage")
			}
			if preimageCandidate.ID != preimage.ID || preimageCandidate.Status != models.StatusQueued {
				t.Errorf("first recovery candidate = (%s, %s), want preimage (%s, %s)",
					preimageCandidate.ID, preimageCandidate.Status, preimage.ID, models.StatusQueued)
			}
			checkMarker(preimageCandidate, test.wantPreimageMarker, test.wantPreimageMarkerPresent, "preimage")

			unmarkedCandidate := candidates[1].artifact
			if unmarkedCandidate == nil {
				t.Fatal("second recovery candidate must contain an artifact")
			}
			if unmarkedCandidate.Status != models.StatusActive {
				t.Errorf("second recovery candidate status = %s, want %s",
					unmarkedCandidate.Status, models.StatusActive)
			}
			checkMarker(unmarkedCandidate, test.wantPreimageMarker, test.wantPreimageMarkerPresent, "unmarked active")

			markedCandidate := candidates[len(candidates)-1].artifact
			if markedCandidate == nil {
				t.Fatal("last recovery candidate must contain the marked artifact")
			}
			if markedCandidate.Status != models.StatusActive {
				t.Errorf("last recovery candidate status = %s, want %s",
					markedCandidate.Status, models.StatusActive)
			}
			checkMarker(markedCandidate, shipmentID, true, "marked active")

			retained, retainedPresent := markedCandidate.CustomFields["retained"]
			if retainedPresent != test.wantRetainedValuePresent {
				t.Errorf("last recovery candidate retained-field presence = %t, want %t",
					retainedPresent, test.wantRetainedValuePresent)
			} else if retainedPresent && retained != test.wantRetainedValue {
				t.Errorf("last recovery candidate retained field = %v, want %v",
					retained, test.wantRetainedValue)
			}

			wantMarkedFields := 1
			if test.wantRetainedValuePresent {
				wantMarkedFields++
			}
			if got := len(markedCandidate.CustomFields); got != wantMarkedFields {
				t.Errorf("last recovery candidate custom_fields has %d entries, want %d",
					got, wantMarkedFields)
			}
		})
	}
}
