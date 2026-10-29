package tests_test

import (
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

// tokenFlagIssuer provisions an ed25519 token-issuer anchor in the fixture
// workspace config, pins it through the environment, and returns a function
// that writes a signed authorization token file.
func tokenFlagIssuer(t *testing.T, fx *reconcileShippedFixture) func(shipmentID, digest, nonce string) string {
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
	yaml := fmt.Sprintf("\ntrust_anchors:\n  - id: op-1\n    role: token-issuer\n    algo: ed25519\n    public_key_ref: |\n%s    fingerprint: %q\n    status: active\n", indented.String(), fingerprint)
	require.NoError(t, os.WriteFile(cfgPath, append(data, []byte(yaml)...), 0o600))
	t.Setenv("BACKLOGIT_TRUST_PINS", fingerprint+"=token-issuer")

	return func(shipmentID, digest, nonce string) string {
		t.Helper()
		now := time.Now().UTC()
		tok := core.AuthorizationToken{
			Header:  core.AuthorizationTokenHeader{Algo: "ed25519", KeyID: "op-1"},
			Claims:  core.AuthorizationTokenClaims{NotBefore: now.Add(-time.Minute), NotAfter: now.Add(10 * time.Minute), Nonce: nonce},
			Binding: core.AuthorizationTokenBinding{ShipmentID: shipmentID, RequestIdentityDigest: digest},
		}
		tok.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(priv, tok.SigningPayload()))
		raw, err := json.Marshal(tok)
		require.NoError(t, err)
		path := filepath.Join(t.TempDir(), "token.json")
		require.NoError(t, os.WriteFile(path, raw, 0o600))
		return path
	}
}

func tokenFlagDigest(t *testing.T, fx *reconcileShippedFixture, key, actor, reason string) string {
	t.Helper()
	digest, err := core.ReconcileRequestIdentityDigest(core.ShipmentShippedReconcileRequest{
		ShipmentID: fx.shipmentID, Reason: reason, Actor: actor, IdempotencyKey: key,
		MergeSHA: fx.deliverySHA, ClosureEvidence: filepath.ToSlash(fx.closureRel),
	})
	require.NoError(t, err)
	return digest
}

func TestReconcileAuthTokenFlagEndToEnd(t *testing.T) {
	t.Run("dry-run prints the authorization verification outcome and writes nothing", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		issue := tokenFlagIssuer(t, fx)
		path := issue(fx.shipmentID, tokenFlagDigest(t, fx, "idem-tok-dry", "operator", "token repair"), "00a1")

		logBefore, _ := snapshotFileOrAbsent(t, fx.itemLogPath())
		args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-tok-dry", "operator", "token repair"), "--dry-run", "--authorization-token", path)
		stdout, stderr, err := runReconcileShippedCLI(t, fx.root, args...)
		require.NoError(t, err, stderr)
		result := decodeReconcileShippedResult(t, stdout)
		require.Equal(t, "reconciled", result["outcome"])
		auth, ok := result["authorization"].(map[string]any)
		require.True(t, ok, "dry-run output must carry the authorization verification outcome: %s", stdout)
		require.Equal(t, "verified", auth["outcome"])
		require.Equal(t, "op-1", auth["key_id"])
		require.Equal(t, "active", fx.readArchivedStatusFromMarkdown(t))
		logAfter, _ := snapshotFileOrAbsent(t, fx.itemLogPath())
		require.Equal(t, logBefore, logAfter, "dry-run must not write the item log")
	})

	t.Run("a token bound to another request identity is rejected fail-closed", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		issue := tokenFlagIssuer(t, fx)
		path := issue(fx.shipmentID, "not-the-digest", "00a2")

		args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-tok-bad", "operator", "token repair"), "--dry-run", "--authorization-token", path)
		_, _, err := runReconcileShippedCLI(t, fx.root, args...)
		require.ErrorIs(t, err, core.ErrApprovalWrongRequestIdentity)
		require.Equal(t, "active", fx.readArchivedStatusFromMarkdown(t))
	})

	t.Run("an unreadable token path fails closed", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		tokenFlagIssuer(t, fx)
		args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-tok-missing", "operator", "token repair"), "--dry-run", "--authorization-token", filepath.Join(t.TempDir(), "absent.json"))
		_, _, err := runReconcileShippedCLI(t, fx.root, args...)
		require.Error(t, err)
		require.Equal(t, "active", fx.readArchivedStatusFromMarkdown(t))
	})

	t.Run("live run without an external nonce provider fails closed before any write", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		issue := tokenFlagIssuer(t, fx)
		path := issue(fx.shipmentID, tokenFlagDigest(t, fx, "idem-tok-live", "operator", "token repair"), "00a3")

		args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-tok-live", "operator", "token repair"), "--authorization-token", path)
		_, _, err := runReconcileShippedCLI(t, fx.root, args...)
		require.ErrorIs(t, err, core.ErrNonceProviderUnavailable)
		require.Equal(t, "active", fx.readArchivedStatusFromMarkdown(t))
	})

	t.Run("absent flag keeps the v1 output shape and confirmation guard", func(t *testing.T) {
		fx := newReconcileShippedFixture(t, "done")
		args := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-tok-v1", "operator", "v1 repair"), "--dry-run")
		stdout, stderr, err := runReconcileShippedCLI(t, fx.root, args...)
		require.NoError(t, err, stderr)
		result := decodeReconcileShippedResult(t, stdout)
		require.Equal(t, "reconciled", result["outcome"])
		require.NotContains(t, result, "authorization")

		live := append(fx.args(fx.deliverySHA, fx.closureRel, "idem-tok-v1", "operator", "v1 repair"), fx.confirmArg()...)
		stdout, stderr, err = runReconcileShippedCLI(t, fx.root, live...)
		require.NoError(t, err, stderr)
		require.Equal(t, "reconciled", decodeReconcileShippedResult(t, stdout)["outcome"])
		require.Equal(t, "shipped", fx.readArchivedStatusFromMarkdown(t))
	})
}
