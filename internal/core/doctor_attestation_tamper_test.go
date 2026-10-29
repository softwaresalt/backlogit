package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// TestDoctorAttestationTamperFailClosed covers 169.014-T: swapped and tampered
// persisted attestations fail closed with typed errors, and the legacy
// presence-only unsigned branch is gone from internal/core.
func TestDoctorAttestationTamperFailClosed(t *testing.T) {
	signer := newAttestationTestSigner(t, "ci-signer")
	d := validReconciledShippedDelta()
	const shipmentID = "048-S"
	rv := &AttestationReverifier{Anchors: []config.TrustAnchor{signer.anchor}, Pins: signer.pins}

	reverify := func(env []byte) error {
		claim := attestationTestClaim(t, env, signer.id)
		claim.Envelope = string(env)
		_, err := rv.Reverify(context.Background(), AttestationReverifyInput{ShipmentID: shipmentID, At: reconcileEventTime, Attestation: claim, Evidence: d.Evidence})
		return err
	}

	t.Run("swapped attestation reference fails closed with a typed binding error", func(t *testing.T) {
		st := attestationTestStatement(expectedFromDelta(shipmentID, d), reconcileEventTime)
		st.Subject[0].Name = "049-S"
		err := reverify(signer.signEnvelope(t, signer.id, st))
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAttestationBindingMismatch)
		var ve *AttestationVerifyError
		require.ErrorAs(t, err, &ve)
		assert.Equal(t, "binding", ve.Field)
	})

	t.Run("byte-tampered attestation fails closed with a typed signature error", func(t *testing.T) {
		env := signer.signEnvelope(t, signer.id, attestationTestStatement(expectedFromDelta(shipmentID, d), reconcileEventTime))
		var parsed AttestationEnvelope
		require.NoError(t, json.Unmarshal(env, &parsed))
		payload, err := base64.StdEncoding.DecodeString(parsed.Payload)
		require.NoError(t, err)
		payload = []byte(strings.Replace(string(payload), "ci@example", "ci@evil000", 1))
		parsed.Payload = base64.StdEncoding.EncodeToString(payload)
		tampered, err := json.Marshal(parsed)
		require.NoError(t, err)

		err = reverify(tampered)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrAttestationSignatureInvalid)
		var ve *AttestationVerifyError
		require.ErrorAs(t, err, &ve)
		assert.Equal(t, "signature", ve.Field)
	})

	t.Run("doctor rejects an envelope signed for another shipment", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ws.Config.TrustAnchors = []config.TrustAnchor{signer.anchor}
		seedReconcileEventShipment(t, ws, func(_ string, dd ShipmentReconciledShippedDelta) *ShipmentReconciledShippedAttestation {
			return signedAttestationFor(t, signer, "999-S", dd)
		})
		assert.Len(t, findingsOfType(doctorShippedAudit(t, ws, signer.pins), FindingMissingShippedEvent), 1)
	})

	t.Run("legacy unsigned core path is absent", func(t *testing.T) {
		banned := map[string]bool{
			"classifyReconciledShippedEvent": true, "reconciledShippedLineSigned": true,
			"reconciledShippedEventKind": true, "reconciledShippedAbsent": true,
			"reconciledShippedUnsigned": true, "reconciledShippedSigned": true,
		}
		files, err := filepath.Glob("*.go")
		require.NoError(t, err)
		fset := token.NewFileSet()
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, name, nil, 0)
			require.NoError(t, err)
			ast.Inspect(file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && banned[id.Name] {
					t.Errorf("%s: legacy presence-only attestation path %q still referenced", fset.Position(id.Pos()), id.Name)
				}
				return true
			})
		}
	})
}
