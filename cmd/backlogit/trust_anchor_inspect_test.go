package main

import (
	"bytes"
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
)

const trustAnchorYAML = `
trust_anchors:
  - id: live-signer
    role: attestation-signer
    algo: ed25519
    public_key_ref: keys/live.pem
    fingerprint: "sha256:live0001"
    status: active
  - id: future-signer
    role: token-issuer
    algo: ecdsa-p256
    public_key_ref: keys/future.pem
    fingerprint: "sha256:future01"
    status: active
    not_before: 2999-01-01T00:00:00Z
  - id: old-signer
    role: legacy
    algo: ed25519
    public_key_ref: keys/old.pem
    fingerprint: "sha256:old00001"
    status: active
    not_after: 2020-01-01T00:00:00Z
`

func trustAnchorWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(ws, 0o755))
	require.NoError(t, config.WriteDefaults(ws))
	cfgPath := filepath.Join(ws, "config.yaml")
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, append(data, []byte(trustAnchorYAML)...), 0o600))
	return root
}

func runTrustAnchorCLI(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := cli.NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--cwd", root, "--no-update-check"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

// snapshotTree hashes every file (name + mtime + content) under root.
func snapshotTree(t *testing.T, root string) []string {
	t.Helper()
	var entries []string
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			entries = append(entries, "d "+rel)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		entries = append(entries, rel+" "+info.ModTime().String()+" "+string(sum[:]))
		return nil
	}))
	sort.Strings(entries)
	return entries
}

func TestTrustAnchorInspectCLI(t *testing.T) {
	root := trustAnchorWorkspace(t)
	before := snapshotTree(t, root)

	t.Run("list prints resolver output", func(t *testing.T) {
		out, err := runTrustAnchorCLI(t, root, "trust-anchor", "list")
		require.NoError(t, err)
		for _, want := range []string{"live-signer", "future-signer", "old-signer", "attestation-signer", "sha256:live0001"} {
			require.Contains(t, out, want)
		}
		lines := map[string]string{}
		for _, line := range strings.Split(out, "\n") {
			for _, id := range []string{"live-signer", "future-signer", "old-signer"} {
				if strings.HasPrefix(strings.TrimSpace(line), id) {
					lines[id] = line
				}
			}
		}
		require.Contains(t, lines["live-signer"], "valid")
		require.Contains(t, lines["future-signer"], "not-yet-valid")
		require.Contains(t, lines["old-signer"], "expired")
	})

	t.Run("show prints resolver output", func(t *testing.T) {
		out, err := runTrustAnchorCLI(t, root, "trust-anchor", "show", "old-signer")
		require.NoError(t, err)
		require.Contains(t, out, "id: old-signer")
		require.Contains(t, out, "role: legacy")
		require.Contains(t, out, "algo: ed25519")
		require.Contains(t, out, "fingerprint: sha256:old00001")
		require.Contains(t, out, "validity: expired")

		_, err = runTrustAnchorCLI(t, root, "trust-anchor", "show", "missing")
		require.Error(t, err)
	})

	t.Run("commands perform no writes", func(t *testing.T) {
		_, _ = runTrustAnchorCLI(t, root, "trust-anchor", "list")
		_, _ = runTrustAnchorCLI(t, root, "trust-anchor", "show", "live-signer")
		require.Equal(t, before, snapshotTree(t, root))
	})
}
