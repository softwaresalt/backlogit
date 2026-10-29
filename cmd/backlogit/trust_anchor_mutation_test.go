package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// mutationEnv provisions a workspace, an external pin set and an approver key.
func mutationEnv(t *testing.T) (root string, priv ed25519.PrivateKey) {
	t.Helper()
	root = trustAnchorWorkspace(t)
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	t.Setenv("BACKLOGIT_TRUST_APPROVER_KEY", base64.StdEncoding.EncodeToString(pub))
	t.Setenv("BACKLOGIT_TRUST_PINS", "sha256:live0001,sha256:future01,sha256:old00001,sha256:new00001,sha256:rot00001")
	return root, priv
}

// writeApproval signs an approval token and writes it as an operator file.
func writeApproval(t *testing.T, priv ed25519.PrivateKey, action core.TrustAnchorAction, id string, expiry time.Time, repl ...config.TrustAnchor) string {
	t.Helper()
	ev := core.ApprovalEvidence{Action: action, AnchorID: id, Approver: "operator", Expiry: expiry, Nonce: "nonce-" + id}
	if len(repl) > 0 {
		ev.Replacement = &repl[0]
	}
	ev.Signature = ed25519.Sign(priv, ev.SigningPayload())
	data, err := json.Marshal(map[string]string{
		"action": string(ev.Action), "anchor_id": ev.AnchorID, "approver": ev.Approver,
		"expiry": ev.Expiry.UTC().Format(time.RFC3339Nano), "nonce": ev.Nonce,
		"signature": base64.StdEncoding.EncodeToString(ev.Signature),
	})
	require.NoError(t, err)
	p := filepath.Join(t.TempDir(), "approval.json")
	require.NoError(t, os.WriteFile(p, data, 0o600))
	return p
}

func loadAnchors(t *testing.T, root string) map[string]config.TrustAnchor {
	t.Helper()
	cfg, err := config.Load(context.Background(), filepath.Join(root, ".backlogit"))
	require.NoError(t, err)
	out := map[string]config.TrustAnchor{}
	for _, a := range cfg.TrustAnchors {
		out[a.ID] = a
	}
	return out
}

var newAnchorFlags = []string{"--id", "new-signer", "--role", "attestation-signer", "--algo", "ed25519",
	"--public-key-ref", "keys/new.pem", "--fingerprint", "sha256:new00001"}

func TestTrustAnchorMutationCLI(t *testing.T) {
	soon := time.Now().Add(time.Hour)

	t.Run("add delegates to core and surfaces the result", func(t *testing.T) {
		root, _ := mutationEnv(t)
		out, err := runTrustAnchorCLI(t, root, append([]string{"trust-anchor", "add"}, newAnchorFlags...)...)
		require.NoError(t, err, out)
		require.Contains(t, out, "new-signer")
		got := loadAnchors(t, root)
		require.Equal(t, config.TrustAnchorStatusActive, got["new-signer"].Status)
		require.Equal(t, "sha256:new00001", got["new-signer"].Fingerprint)
		require.FileExists(t, filepath.Join(root, ".backlogit", "trust_anchor_audit.jsonl"))
	})

	t.Run("add of an unpinned fingerprint is rejected with no partial write", func(t *testing.T) {
		root, _ := mutationEnv(t)
		before, err := os.ReadFile(filepath.Join(root, ".backlogit", "config.yaml"))
		require.NoError(t, err)
		args := append([]string{"trust-anchor", "add"}, newAnchorFlags...)
		args[len(args)-1] = "sha256:notpinned"
		_, err = runTrustAnchorCLI(t, root, args...)
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned)
		after, err := os.ReadFile(filepath.Join(root, ".backlogit", "config.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(before), string(after))
	})

	t.Run("revoke without approval is refused and mutates nothing", func(t *testing.T) {
		root, _ := mutationEnv(t)
		before := snapshotTree(t, root)
		_, err := runTrustAnchorCLI(t, root, "trust-anchor", "revoke", "live-signer")
		require.Error(t, err)
		require.ErrorIs(t, err, core.ErrApprovalRequired)
		require.Equal(t, before, snapshotTree(t, root), "no core mutation, config or audit write may occur")
	})

	t.Run("rotate without approval is refused and mutates nothing", func(t *testing.T) {
		root, _ := mutationEnv(t)
		before := snapshotTree(t, root)
		args := []string{"trust-anchor", "rotate", "live-signer", "--id", "rot-signer", "--role", "attestation-signer",
			"--algo", "ed25519", "--public-key-ref", "keys/rot.pem", "--fingerprint", "sha256:rot00001"}
		_, err := runTrustAnchorCLI(t, root, args...)
		require.ErrorIs(t, err, core.ErrApprovalRequired)
		require.Equal(t, before, snapshotTree(t, root))
	})

	// 168.009-T: the CLI wires no external revocation authority, so a revoke
	// that could only flip advisory workspace status fails closed.
	t.Run("revoke with verifiable approval fails closed without an external authority", func(t *testing.T) {
		root, priv := mutationEnv(t)
		appr := writeApproval(t, priv, core.TrustAnchorActionRevoke, "live-signer", soon)
		_, err := runTrustAnchorCLI(t, root, "trust-anchor", "revoke", "live-signer", "--approval", appr)
		require.ErrorIs(t, err, core.ErrRevocationAuthorityRequired)
		require.Contains(t, err.Error(), "sha256:live0001", "the required external operation is surfaced")
		require.Equal(t, config.TrustAnchorStatusActive, loadAnchors(t, root)["live-signer"].Status, "no advisory flip")
	})

	// Rotate must retire the old key authoritatively; the CLI has no authority,
	// so it fails closed with no workspace change.
	t.Run("rotate with verifiable approval fails closed without an external authority", func(t *testing.T) {
		root, priv := mutationEnv(t)
		repl := config.TrustAnchor{ID: "rot-signer", Role: "attestation-signer", Algo: "ed25519", PublicKeyRef: "keys/rot.pem", Fingerprint: "sha256:rot00001"}
		appr := writeApproval(t, priv, core.TrustAnchorActionRotate, "live-signer", soon, repl)
		args := []string{"trust-anchor", "rotate", "live-signer", "--approval", appr, "--id", "rot-signer", "--role", "attestation-signer",
			"--algo", "ed25519", "--public-key-ref", "keys/rot.pem", "--fingerprint", "sha256:rot00001"}
		_, err := runTrustAnchorCLI(t, root, args...)
		require.ErrorIs(t, err, core.ErrRevocationAuthorityRequired)
		require.Contains(t, err.Error(), "sha256:live0001")
		got := loadAnchors(t, root)
		require.Equal(t, config.TrustAnchorStatusActive, got["live-signer"].Status)
		require.NotContains(t, got, "rot-signer")
	})

	t.Run("forged approval is rejected with no partial write", func(t *testing.T) {
		root, _ := mutationEnv(t)
		_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)
		appr := writeApproval(t, otherPriv, core.TrustAnchorActionRevoke, "live-signer", soon)
		before, err := os.ReadFile(filepath.Join(root, ".backlogit", "config.yaml"))
		require.NoError(t, err)
		_, err = runTrustAnchorCLI(t, root, "trust-anchor", "revoke", "live-signer", "--approval", appr)
		require.ErrorIs(t, err, core.ErrApprovalInvalid)
		after, err := os.ReadFile(filepath.Join(root, ".backlogit", "config.yaml"))
		require.NoError(t, err)
		require.Equal(t, string(before), string(after))
	})

	t.Run("approval without an operator approver key is refused", func(t *testing.T) {
		root, priv := mutationEnv(t)
		appr := writeApproval(t, priv, core.TrustAnchorActionRevoke, "live-signer", soon)
		t.Setenv("BACKLOGIT_TRUST_APPROVER_KEY", "")
		before := snapshotTree(t, root)
		_, err := runTrustAnchorCLI(t, root, "trust-anchor", "revoke", "live-signer", "--approval", appr)
		require.Error(t, err)
		require.Equal(t, before, snapshotTree(t, root))
	})

	t.Run("rejected core mutation (duplicate id) exits non-zero without writing", func(t *testing.T) {
		root, _ := mutationEnv(t)
		before := snapshotTree(t, root)
		args := append([]string{"trust-anchor", "add"}, newAnchorFlags...)
		args[3] = "live-signer" // --id value collides with the existing anchor
		_, err := runTrustAnchorCLI(t, root, args...)
		require.ErrorIs(t, err, core.ErrTrustAnchorMutationInvalid)
		require.Equal(t, before, snapshotTree(t, root))
	})
}
