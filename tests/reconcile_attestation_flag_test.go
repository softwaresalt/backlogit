package tests_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// attestationFlagSigner provisions an ed25519 attestation-signer anchor in the
// fixture workspace config, pins it through the environment, and returns a
// function that writes a signed attestation file for given evidence.
func attestationFlagSigner(t *testing.T, fx *reconcileShippedFixture) func(expected core.AttestationExpectedEvidence) string {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	sum := sha256.Sum256(der)
	fingerprint := "sha256:" + hex.EncodeToString(sum[:])
	pemText := "-----BEGIN PUBLIC KEY-----\n" + base64.StdEncoding.EncodeToString(der) + "\n-----END PUBLIC KEY-----\n"

	cfgPath := filepath.Join(fx.root, ".backlogit", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	var indented strings.Builder
	for _, line := range strings.Split(strings.TrimRight(pemText, "\n"), "\n") {
		indented.WriteString("      " + line + "\n")
	}
	yaml := fmt.Sprintf("\ntrust_anchors:\n  - id: ci-signer\n    role: attestation-signer\n    algo: ed25519\n    public_key_ref: |\n%s    fingerprint: %q\n    status: active\n", indented.String(), fingerprint)
	require.NoError(t, os.WriteFile(cfgPath, append(data, []byte(yaml)...), 0o600))
	t.Setenv("BACKLOGIT_TRUST_PINS", fingerprint+"=attestation-signer")

	return func(expected core.AttestationExpectedEvidence) string {
		t.Helper()
		st := core.AttestationStatement{
			Type:          "https://in-toto.io/Statement/v1",
			Subject:       []core.AttestationSubject{{Name: expected.ShipmentID, Digest: map[string]string{"sha256": "abc"}}},
			PredicateType: "https://backlogit.dev/delivery-evidence/v1",
			Predicate: core.AttestationPredicate{
				ShipmentID: expected.ShipmentID, MergeSHA: expected.MergeSHA, ManifestDigest: expected.ManifestDigest,
				ClosureContentHash: expected.ClosureContentHash, SignerIdentity: "ci@example",
				IssuedAt:  time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
				ExpiresAt: time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
			},
		}
		body, err := json.Marshal(st)
		require.NoError(t, err)
		const payloadType = "application/vnd.in-toto+json"
		pae := fmt.Sprintf("DSSEv1 %d %s %d %s", len(payloadType), payloadType, len(body), body)
		sig := ed25519.Sign(priv, []byte(pae))
		env, err := json.Marshal(core.AttestationEnvelope{
			PayloadType: payloadType,
			Payload:     base64.StdEncoding.EncodeToString(body),
			Signatures:  []core.AttestationSignature{{KeyID: "ci-signer", Sig: base64.StdEncoding.EncodeToString(sig), Algo: "ed25519"}},
		})
		require.NoError(t, err)
		path := filepath.Join(t.TempDir(), "attestation.json")
		require.NoError(t, os.WriteFile(path, env, 0o600))
		return path
	}
}

// discoverExpectedEvidence learns the reconcile-computed evidence by running a
// dry-run through core with a capturing verifier.
func discoverExpectedEvidence(t *testing.T, fx *reconcileShippedFixture, args []string) core.AttestationExpectedEvidence {
	t.Helper()
	var got core.AttestationExpectedEvidence
	req := core.ShipmentShippedReconcileRequest{
		ShipmentID: fx.shipmentID, Reason: "preview", Actor: "operator", IdempotencyKey: "idem-discover",
		MergeSHA: fx.deliverySHA, ClosureEvidence: filepath.ToSlash(fx.closureRel), DryRun: true,
		AttestationVerifier: func(expected core.AttestationExpectedEvidence) error {
			got = expected
			return nil
		},
	}
	_, err := core.ReconcileShipmentToShipped(context.Background(), fx.ws, req)
	require.NoError(t, err)
	require.NotEmpty(t, got.ManifestDigest, "the verifier hook must receive the reconcile-computed evidence")
	require.NotEmpty(t, got.ClosureContentHash)
	require.Equal(t, fx.shipmentID, got.ShipmentID)
	require.Equal(t, fx.deliverySHA, got.MergeSHA)
	_ = args
	return got
}

func TestReconcileAttestationFlagEndToEnd(t *testing.T) {
	t.Run("dry-run prints the verification outcome and writes nothing", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		sign := attestationFlagSigner(t, fx)
		path := sign(discoverExpectedEvidence(t, fx, nil))

		logBefore, _ := snapshotFileOrAbsent(t, fx.itemLogPath())
		args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-att-dry", "operator", "signed repair"), "--dry-run", "--attestation", path)
		stdout, stderr, err := runReconcileShippedCLI(t, fx.root, args...)
		require.NoError(t, err, stderr)
		result := decodeReconcileShippedResult(t, stdout)
		require.Equal(t, "reconciled", result["outcome"])
		att, ok := result["attestation"].(map[string]any)
		require.True(t, ok, "dry-run output must carry the attestation verification outcome: %s", stdout)
		require.Equal(t, "verified", att["outcome"])
		require.Equal(t, "ci-signer", att["key_id"])
		require.Equal(t, "active", fx.readArchivedStatusFromMarkdown(t))
		logAfter, _ := snapshotFileOrAbsent(t, fx.itemLogPath())
		require.Equal(t, logBefore, logAfter, "dry-run must not write the item log")
	})

	t.Run("evidence mismatch is rejected fail-closed", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		sign := attestationFlagSigner(t, fx)
		expected := discoverExpectedEvidence(t, fx, nil)
		expected.MergeSHA = strings.Repeat("0", 40)
		path := sign(expected)

		args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-att-bad", "operator", "signed repair"), "--dry-run", "--attestation", path)
		_, _, err := runReconcileShippedCLI(t, fx.root, args...)
		require.ErrorIs(t, err, core.ErrAttestationEvidenceMismatch)
		require.Equal(t, "active", fx.readArchivedStatusFromMarkdown(t))
	})

	t.Run("absent flag keeps the v1 output shape", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-v1", "operator", "v1 repair"), "--dry-run")
		stdout, stderr, err := runReconcileShippedCLI(t, fx.root, args...)
		require.NoError(t, err, stderr)
		result := decodeReconcileShippedResult(t, stdout)
		require.Equal(t, "reconciled", result["outcome"])
		require.NotContains(t, result, "attestation")
	})

	t.Run("live signed run records the attestation and replay requires the same attestation", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		sign := attestationFlagSigner(t, fx)
		path := sign(discoverExpectedEvidence(t, fx, nil))

		live := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-att-live", "operator", "signed repair"), fx.confirmArg()...)
		stdout, stderr, err := runReconcileShippedCLI(t, fx.root, append(live, "--attestation", path)...)
		require.NoError(t, err, stderr)
		require.Equal(t, "reconciled", decodeReconcileShippedResult(t, stdout)["outcome"])
		require.Equal(t, "shipped", fx.readArchivedStatusFromMarkdown(t))
		raw, err := os.ReadFile(fx.itemLogPath())
		require.NoError(t, err)
		require.Contains(t, string(raw), `"attestation":{"key_id":"ci-signer"`)

		stdout, stderr, err = runReconcileShippedCLI(t, fx.root, append(live, "--attestation", path)...)
		require.NoError(t, err, stderr)
		require.Equal(t, "no_op", decodeReconcileShippedResult(t, stdout)["outcome"])

		_, _, err = runReconcileShippedCLI(t, fx.root, live...)
		require.Error(t, err, "replay without the attestation must conflict")
	})
}
