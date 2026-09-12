package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func classifierEvent(t *testing.T, itemID, idempotencyKey, requestIdentityDigest string, mutate func(*ShipmentReconciledShippedDelta)) []byte {
	t.Helper()
	delta := validReconciledShippedDelta()
	delta.IdempotencyKey = idempotencyKey
	delta.RequestIdentityDigest = requestIdentityDigest
	if mutate != nil {
		mutate(&delta)
	}
	return marshalReconciledShippedEvent(t, delta, itemID)
}

func classifierFrontmatter(archivedStatus, idempotencyKey, requestIdentityDigest string, preparedEvent []byte, eventDigest string) map[string]any {
	fm := map[string]any{}
	if archivedStatus != "" {
		fm["archived_status"] = archivedStatus
	}
	if idempotencyKey != "" {
		fm["custom_fields"] = map[string]any{
			"shipment_reconciliation_idempotency_key": idempotencyKey,
		}
	}
	if requestIdentityDigest != "" {
		fm["shipment_reconciliation_request_identity_digest"] = requestIdentityDigest
	}
	if preparedEvent != nil {
		fm["shipment_reconciliation_prepared_event"] = string(preparedEvent)
	}
	if eventDigest != "" {
		fm["shipment_reconciliation_event_digest"] = eventDigest
	}
	return fm
}

func TestClassifyShipmentReconcileState_Totality(t *testing.T) {
	const (
		itemID      = "167.010-T-shipment"
		reqKey      = "idem-001"
		reqDigest   = "abc123"
		otherKey    = "idem-999"
		otherDigest = "digest-999"
	)

	validEvent := classifierEvent(t, itemID, reqKey, reqDigest, nil)
	otherEvent := classifierEvent(t, itemID, otherKey, otherDigest, nil)
	invalidEvent := classifierEvent(t, itemID, reqKey, reqDigest, func(delta *ShipmentReconciledShippedDelta) {
		delta.Evidence.EvidenceDigest = ""
	})

	cases := []struct {
		name        string
		log         []byte
		frontmatter map[string]any
		reqKey      string
		reqDigest   string
		wantOutcome ShipmentReconcileOutcome
		wantErrIs   error
	}{
		{
			name:        "branch_0b_partial_trailing_line_is_indeterminate",
			log:         []byte(`{"timestamp":`),
			frontmatter: classifierFrontmatter("active", "", "", nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeIndeterminate,
			wantErrIs:   blerrors.ErrValidation,
		},
		{
			name:        "branch_0b_non_target_malformed_jsonl_line_is_indeterminate",
			log:         []byte("{not-json}\n"),
			frontmatter: classifierFrontmatter("active", "", "", nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeIndeterminate,
			wantErrIs:   blerrors.ErrValidation,
		},
		{
			name:        "branch_1a_invalid_logged_reconcile_event_is_indeterminate",
			log:         invalidEvent,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, reqDigest, nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeIndeterminate,
			wantErrIs:   blerrors.ErrValidation,
		},
		{
			name:        "branch_1b_single_valid_logged_reconcile_event_same_request_is_no_op",
			log:         validEvent,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, reqDigest, nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeNoOp,
		},
		{
			name:        "branch_1c_logged_event_for_different_request_is_conflict",
			log:         otherEvent,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, reqDigest, nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeConflict,
		},
		{
			name:        "branch_1c_same_key_different_request_identity_is_conflict",
			log:         validEvent,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, otherDigest, nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeConflict,
		},
		{
			name:        "branch_1c_missing_persisted_key_with_logged_event_is_conflict",
			log:         validEvent,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), "", reqDigest, nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeConflict,
		},
		{
			name:        "branch_1c_multiple_valid_logged_reconcile_events_are_conflict",
			log:         append(append([]byte{}, validEvent...), otherEvent...),
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, reqDigest, nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeConflict,
		},
		{
			name:        "branch_1c_logged_reconcile_event_without_shipped_frontmatter_is_conflict",
			log:         validEvent,
			frontmatter: classifierFrontmatter("active", reqKey, reqDigest, nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeConflict,
		},
		{
			name:        "branch_2a_shipped_without_logged_event_but_torn_resume_state_is_indeterminate",
			log:         nil,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, reqDigest, nil, ShipmentReconciledShippedEventDigest(validEvent)),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeIndeterminate,
			wantErrIs:   blerrors.ErrValidation,
		},
		{
			name:        "branch_2a_shipped_without_logged_event_but_prepared_event_digest_mismatch_is_indeterminate",
			log:         nil,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, reqDigest, validEvent, "deadbeef"),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeIndeterminate,
			wantErrIs:   blerrors.ErrValidation,
		},
		{
			name:        "branch_2a_non_shipped_with_resume_markers_is_indeterminate",
			log:         nil,
			frontmatter: classifierFrontmatter("active", reqKey, reqDigest, validEvent, ShipmentReconciledShippedEventDigest(validEvent)),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeIndeterminate,
			wantErrIs:   blerrors.ErrValidation,
		},
		{
			name:        "branch_2b_shipped_without_logged_event_but_matching_prepared_event_is_no_op",
			log:         nil,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, reqDigest, validEvent, ShipmentReconciledShippedEventDigest(validEvent)),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeNoOp,
		},
		{
			name:        "branch_2c_shipped_without_logged_event_and_different_key_is_conflict",
			log:         nil,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), otherKey, otherDigest, otherEvent, ShipmentReconciledShippedEventDigest(otherEvent)),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeConflict,
		},
		{
			name:        "branch_2c_shipped_without_logged_event_and_tampered_frontmatter_markers_is_conflict",
			log:         nil,
			frontmatter: classifierFrontmatter(string(ShipmentShipped), reqKey, reqDigest, otherEvent, ShipmentReconciledShippedEventDigest(otherEvent)),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeConflict,
		},
		{
			name:        "branch_2d_empty_readable_log_without_resume_markers_is_reconciled_not_branch_0",
			log:         nil,
			frontmatter: classifierFrontmatter("active", "", "", nil, ""),
			reqKey:      reqKey,
			reqDigest:   reqDigest,
			wantOutcome: ShipmentReconcileOutcomeReconciled,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				gotOutcome ShipmentReconcileOutcome
				gotErr     error
			)

			assert.NotPanics(t, func() {
				gotOutcome, gotErr = classifyShipmentReconcileState(tc.log, tc.frontmatter, tc.reqKey, tc.reqDigest)
			})
			assert.Equal(t, tc.wantOutcome, gotOutcome)
			if tc.wantErrIs == nil {
				require.NoError(t, gotErr)
				return
			}
			require.Error(t, gotErr)
			assert.True(t, errors.Is(gotErr, tc.wantErrIs), "expected error to match %v, got %v", tc.wantErrIs, gotErr)
		})
	}
}
