package core

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

func reconcileTestAttestation() *ShipmentReconciledShippedAttestation {
	return &ShipmentReconciledShippedAttestation{
		KeyID:           "att",
		SignerIdentity:  "ci@example",
		SigAlgo:         "ed25519",
		StatementDigest: strings.Repeat("a", 64),
	}
}

func signedClassifierEvent(t *testing.T, itemID, key, digest string, att *ShipmentReconciledShippedAttestation) []byte {
	t.Helper()
	return classifierEvent(t, itemID, key, digest, func(d *ShipmentReconciledShippedDelta) { d.Attestation = att })
}

func TestReconcileAttestationDelta(t *testing.T) {
	t.Run("unsigned delta omits the attestation member", func(t *testing.T) {
		raw := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")
		assert.NotContains(t, string(raw), `"attestation"`)
	})

	t.Run("signed delta carries a complete attestation and validates", func(t *testing.T) {
		delta := validReconciledShippedDelta()
		delta.Attestation = reconcileTestAttestation()
		raw := marshalReconciledShippedEvent(t, delta, "001-S")
		assert.Contains(t, string(raw), `"attestation":{"key_id":"att","signer_identity":"ci@example","sig_algo":"ed25519","statement_digest":`)
		require.NoError(t, ValidateShipmentReconciledShippedEvent(raw, nil, "", "001-S"))

		for _, field := range []string{"key_id", "signer_identity", "sig_algo", "statement_digest"} {
			t.Run("incomplete "+field, func(t *testing.T) {
				incomplete := validReconciledShippedDelta()
				att := *reconcileTestAttestation()
				switch field {
				case "key_id":
					att.KeyID = ""
				case "signer_identity":
					att.SignerIdentity = ""
				case "sig_algo":
					att.SigAlgo = ""
				default:
					att.StatementDigest = ""
				}
				incomplete.Attestation = &att
				err := ValidateShipmentReconciledShippedEvent(marshalReconciledShippedEvent(t, incomplete, "001-S"), nil, "", "001-S")
				require.ErrorIs(t, err, blerrors.ErrValidation)
			})
		}
	})

	t.Run("attestation folds into the evidence and event digests but not the request identity", func(t *testing.T) {
		base := shipmentReconcileEvidenceDigestInput{
			ShipmentID: "048-S", MergeSHA: strings.Repeat("a", 40), TrustedRefName: "main",
			TrustedRefTip: strings.Repeat("b", 40), ManifestDigest: "m", ClosureContentHash: strings.Repeat("c", 64),
			EvidenceRefs: []string{},
		}
		signed := base
		signed.Attestation = reconcileTestAttestation()
		assert.NotEqual(t, shipmentReconcileEvidenceDigest(base), shipmentReconcileEvidenceDigest(signed))
		other := base
		att := *reconcileTestAttestation()
		att.StatementDigest = strings.Repeat("b", 64)
		other.Attestation = &att
		assert.NotEqual(t, shipmentReconcileEvidenceDigest(signed), shipmentReconcileEvidenceDigest(other))

		unsignedEvent := marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "001-S")
		signedDelta := validReconciledShippedDelta()
		signedDelta.Attestation = reconcileTestAttestation()
		signedEvent := marshalReconciledShippedEvent(t, signedDelta, "001-S")
		assert.NotEqual(t, ShipmentReconciledShippedEventDigest(unsignedEvent), ShipmentReconciledShippedEventDigest(signedEvent))

		req := ShipmentShippedReconcileRequest{
			ShipmentID: "048-S", Reason: "governed repair", Actor: "operator", SecondApprover: "auditor",
			IdempotencyKey: "idem-1", MergeSHA: strings.Repeat("a", 40), ClosureEvidence: "docs/closure/x.md",
		}
		plain, err := shipmentReconcileRequestIdentityDigest(req)
		require.NoError(t, err)
		req.Attestation = reconcileTestAttestation()
		withAtt, err := shipmentReconcileRequestIdentityDigest(req)
		require.NoError(t, err)
		assert.Equal(t, plain, withAtt)
	})

	t.Run("prepared event threads the request attestation", func(t *testing.T) {
		fixture := setupShipmentReconcileEvidenceFixture(t)
		fixture.ws.Config.Reconcile = &config.ReconcileConfig{}
		unsigned, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, fixture.request("048-S", fixture.merge048), fixedShipmentReconcileEvidenceInput())
		require.NoError(t, err)
		req := fixture.request("048-S", fixture.merge048)
		req.Attestation = reconcileTestAttestation()
		signed, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
		require.NoError(t, err)

		assert.Equal(t, unsigned.RequestIdentityDigest, signed.RequestIdentityDigest)
		assert.NotEqual(t, unsigned.EvidenceDigest, signed.EvidenceDigest)
		assert.NotEqual(t, unsigned.EventDigest, signed.EventDigest)
		var wrapper events.Event
		require.NoError(t, json.Unmarshal(signed.EventBytes, &wrapper))
		delta, err := decodeShipmentReconciledShippedDelta(wrapper.Delta)
		require.NoError(t, err)
		require.NotNil(t, delta.Attestation)
		assert.Equal(t, *reconcileTestAttestation(), *delta.Attestation)
		require.NoError(t, ValidateShipmentReconciledShippedEvent(signed.EventBytes, signed.EventBytes, signed.EventDigest, "048-S"))
	})

	t.Run("replay with an added, removed or changed attestation is a conflict", func(t *testing.T) {
		const (
			itemID = "169.003-T-shipment"
			key    = "idem-001"
			digest = "abc123"
		)
		att := reconcileTestAttestation()
		changed := *att
		changed.StatementDigest = strings.Repeat("b", 64)

		signedEvent := signedClassifierEvent(t, itemID, key, digest, att)
		unsignedEvent := signedClassifierEvent(t, itemID, key, digest, nil)
		fm := classifierFrontmatter(string(ShipmentShipped), key, digest, nil, "")

		cases := []struct {
			name string
			log  []byte
			fm   map[string]any
			req  *ShipmentReconciledShippedAttestation
			want ShipmentReconcileOutcome
		}{
			{"logged signed, same attestation", signedEvent, fm, att, ShipmentReconcileOutcomeNoOp},
			{"logged signed, attestation removed", signedEvent, fm, nil, ShipmentReconcileOutcomeConflict},
			{"logged signed, attestation changed", signedEvent, fm, &changed, ShipmentReconcileOutcomeConflict},
			{"logged unsigned, attestation added", unsignedEvent, fm, att, ShipmentReconcileOutcomeConflict},
			{"logged unsigned, still unsigned", unsignedEvent, fm, nil, ShipmentReconcileOutcomeNoOp},
			{"prepared signed, same attestation", nil, classifierFrontmatter(string(ShipmentShipped), key, digest, signedEvent, ShipmentReconciledShippedEventDigest(signedEvent)), att, ShipmentReconcileOutcomeNoOp},
			{"prepared signed, attestation removed", nil, classifierFrontmatter(string(ShipmentShipped), key, digest, signedEvent, ShipmentReconciledShippedEventDigest(signedEvent)), nil, ShipmentReconcileOutcomeConflict},
			{"prepared unsigned, attestation added", nil, classifierFrontmatter(string(ShipmentShipped), key, digest, unsignedEvent, ShipmentReconciledShippedEventDigest(unsignedEvent)), att, ShipmentReconcileOutcomeConflict},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := classifyShipmentReconcileState(tc.log, tc.fm, key, digest, itemID, tc.req)
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			})
		}
	})

	t.Run("doctor distinguishes a signed from an unsigned reconcile", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		logsDir := WorkspaceLogsRoot(ws.RootPath)
		require.NoError(t, os.MkdirAll(logsDir, 0o755))

		signedDelta := validReconciledShippedDelta()
		signedDelta.Attestation = reconcileTestAttestation()
		write := func(id string, raw []byte) {
			require.NoError(t, os.WriteFile(events.LogPathForItem(logsDir, id), raw, 0o644))
		}
		write("signed-S", marshalReconciledShippedEvent(t, signedDelta, "signed-S"))
		write("unsigned-S", marshalReconciledShippedEvent(t, validReconciledShippedDelta(), "unsigned-S"))

		kind, readable := classifyReconciledShippedEvent(logsDir, "signed-S")
		assert.True(t, readable)
		assert.Equal(t, reconciledShippedSigned, kind)
		kind, readable = classifyReconciledShippedEvent(logsDir, "unsigned-S")
		assert.True(t, readable)
		assert.Equal(t, reconciledShippedUnsigned, kind)
		kind, readable = classifyReconciledShippedEvent(logsDir, "missing-S")
		assert.True(t, readable)
		assert.Equal(t, reconciledShippedAbsent, kind)

		present, _ := reconciledShippedEventPresence(logsDir, "signed-S")
		assert.True(t, present, "a signed reconcile still suppresses the missing-shipped-event finding")
	})
}
