package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// extAuditor is a no-op consume auditor for store-level tests.
type extAuditor struct{}

func (extAuditor) RecordNonceConsume(NonceConsumeEvent) error { return nil }

// extConsumer builds a consumer over provider and a workspace-style projection at projectionPath.
func extConsumer(t *testing.T, provider NonceConsumeProvider, projectionPath string) *NonceConsumer {
	t.Helper()
	projection, err := OpenNonceLedger(projectionPath)
	require.NoError(t, err)
	consumer, err := NewNonceConsumer(provider, projection, extAuditor{})
	require.NoError(t, err)
	return consumer
}

// extWriteFile writes body to name inside dir and returns the path.
func extWriteFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

type extPinEntry struct {
	Fingerprint string `yaml:"fingerprint"`
	Role        string `yaml:"role"`
}

type extPinsDoc struct {
	TrustAnchors []config.TrustAnchor `yaml:"trust_anchors,omitempty"`
	Pins         []extPinEntry        `yaml:"pins,omitempty"`
}

// extPinsFile marshals doc into a pins file in dir.
func extPinsFile(t *testing.T, dir string, doc extPinsDoc) string {
	t.Helper()
	raw, err := yaml.Marshal(doc)
	require.NoError(t, err)
	return extWriteFile(t, dir, "pins.yaml", string(raw))
}

func TestExternal_PathContainment(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "ws")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".backlogit"), 0o755))
	inside := extWriteFile(t, filepath.Join(root, ".backlogit"), "pins.yaml", "pins: []\n")
	sibling := filepath.Join(parent, "ws-evil")
	require.NoError(t, os.MkdirAll(sibling, 0o755))
	outside := extWriteFile(t, sibling, "pins.yaml", "pins: []\n")

	t.Run("pins_inside_workspace_refused", func(t *testing.T) {
		_, err := resolveCustodyPinsPath(root, inside)
		assert.ErrorIs(t, err, ErrCustodyExternalPathInWorkspace)
		assert.ErrorIs(t, err, blerrors.ErrValidation)
	})
	t.Run("pins_outside_workspace_including_prefix_sibling_accepted", func(t *testing.T) {
		got, err := resolveCustodyPinsPath(root, outside)
		require.NoError(t, err)
		assert.True(t, filepath.IsAbs(got), "the resolved path must be absolute")
	})
	t.Run("missing_pins_file_fails_closed", func(t *testing.T) {
		_, err := resolveCustodyPinsPath(root, filepath.Join(sibling, "missing.yaml"))
		assert.Error(t, err)
	})
	t.Run("nonce_store_inside_workspace_refused", func(t *testing.T) {
		_, err := resolveCustodyNonceStorePath(root, filepath.Join(root, ".backlogit", "nonces.jsonl"))
		assert.ErrorIs(t, err, ErrCustodyExternalPathInWorkspace)
	})
	t.Run("nonce_store_outside_may_not_exist_yet", func(t *testing.T) {
		got, err := resolveCustodyNonceStorePath(root, filepath.Join(sibling, "new-store.jsonl"))
		require.NoError(t, err)
		assert.Equal(t, "new-store.jsonl", filepath.Base(got))
	})
	t.Run("nonce_store_missing_directory_fails_closed", func(t *testing.T) {
		_, err := resolveCustodyNonceStorePath(root, filepath.Join(sibling, "no-such-dir", "nonces.jsonl"))
		assert.Error(t, err)
	})
}

func TestExternal_WithinRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Repo")
	cases := []struct {
		name string
		path string
		fold bool
		want bool
	}{
		{"root_itself", root, false, true},
		{"child", filepath.Join(root, "x.yaml"), false, true},
		{"upper_case_child_folded", strings.ToUpper(root) + string(filepath.Separator) + "x.yaml", true, true},
		{"lower_case_child_folded", strings.ToLower(root) + string(filepath.Separator) + "x.yaml", true, true},
		{"prefix_sibling_is_outside", root + "-evil" + string(filepath.Separator) + "x.yaml", true, false},
		{"unrelated_is_outside", filepath.Join(filepath.Dir(root), "other", "x.yaml"), true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, custodyWithinRoot(root, tc.path, tc.fold))
		})
	}
}

func TestExternal_OperatorPinsFile(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "ws")
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".backlogit"), 0o755))
	ext := filepath.Join(parent, "external")
	require.NoError(t, os.MkdirAll(ext, 0o755))

	t.Run("inside_workspace_refused_without_read", func(t *testing.T) {
		inside := extWriteFile(t, filepath.Join(root, ".backlogit"), "pins.yaml", "pins:\n  - fingerprint: fp-1\n    role: operator\n")
		_, _, err := loadCustodyOperatorPins(root, inside)
		assert.ErrorIs(t, err, ErrCustodyExternalPathInWorkspace)
	})
	t.Run("missing_file_fails_closed", func(t *testing.T) {
		_, _, err := loadCustodyOperatorPins(root, filepath.Join(ext, "missing.yaml"))
		assert.Error(t, err)
	})

	cases := []struct {
		name string
		body string
	}{
		{"unknown_top_level_key_rejected", "pins:\n  - fingerprint: fp-1\n    role: operator\nextra: true\n"},
		{"unknown_pin_key_rejected", "pins:\n  - fingerprint: fp-1\n    role: operator\n    tombstone: true\n"},
		{"empty_file_rejected", ""},
		{"no_entries_rejected", "trust_anchors: []\npins: []\n"},
		{"empty_fingerprint_rejected", "pins:\n  - fingerprint: \"\"\n    role: operator\n"},
		{"second_document_rejected", "pins:\n  - fingerprint: fp-1\n    role: operator\n---\npins: []\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := extWriteFile(t, ext, "bad-"+strings.ReplaceAll(tc.name, "_", "-")+".yaml", tc.body)
			_, _, err := loadCustodyOperatorPins(root, path)
			assert.ErrorIs(t, err, ErrCustodyOperatorPinsInvalid)
		})
	}

	t.Run("pins_only_file_is_accepted", func(t *testing.T) {
		path := extWriteFile(t, ext, "pins-only.yaml", "pins:\n  - fingerprint: fp-1\n    role: operator\n")
		anchors, pins, err := loadCustodyOperatorPins(root, path)
		require.NoError(t, err)
		assert.Empty(t, anchors)
		require.NotNil(t, pins)
		assert.True(t, pins.Contains("fp-1"))
		binding, ok := pins.Binding("fp-1")
		require.True(t, ok)
		assert.Equal(t, "operator", binding.Role)
	})
}

func TestExternal_OperatorPinsZeroFallback(t *testing.T) {
	ctx := context.Background()
	const anchorFP = "fp-op-1"

	t.Run("workspace_anchor_not_consulted_when_pins_file_set", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		anchor := fx.anchor
		anchor.Fingerprint = anchorFP
		fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{anchor}
		token := fx.token(t, "ext-zero-1")
		pins := extPinsFile(t, t.TempDir(), extPinsDoc{Pins: []extPinEntry{{Fingerprint: "fp-other", Role: "operator"}}})
		opts := fx.acceptOpts(token)
		opts.DryRun = true
		opts.OperatorPinsPath = pins
		_, err := AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
		require.Error(t, err, "with a pins file set, a workspace anchor that the file does not pin must not verify the token")
	})

	t.Run("file_pinned_anchor_verifies_without_workspace_anchors", func(t *testing.T) {
		fx := custodyReviewOfferAdoption(t)
		anchor := fx.anchor
		anchor.Fingerprint = anchorFP
		fx.repo.ws.Config.TrustAnchors = nil
		token := fx.token(t, "ext-zero-2")
		pins := extPinsFile(t, t.TempDir(), extPinsDoc{
			TrustAnchors: []config.TrustAnchor{anchor},
			Pins:         []extPinEntry{{Fingerprint: anchorFP, Role: "operator"}},
		})
		opts := fx.acceptOpts(token)
		opts.DryRun = true
		opts.OperatorPinsPath = pins
		_, err := AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
		require.NoError(t, err, "an anchor pinned and bound by the operator pins file must verify the token")
	})
}

func TestExternal_NonceStore(t *testing.T) {
	ctx := context.Background()
	future := time.Now().UTC().Add(time.Hour)

	t.Run("replay_refused_across_store_instances", func(t *testing.T) {
		dir := t.TempDir()
		storePath := filepath.Join(dir, "nonces.jsonl")
		first := extConsumer(t, newCustodyFileNonceStore(storePath), filepath.Join(dir, "projection-a.json"))
		require.NoError(t, first.ConsumeBound(ctx, "ext-nonce-1", future, "digest-1"))

		// A second store over the same file, with an empty projection, models another process.
		second := extConsumer(t, newCustodyFileNonceStore(storePath), filepath.Join(dir, "projection-b.json"))
		err := second.ConsumeBound(ctx, "ext-nonce-1", future, "digest-1")
		assert.ErrorIs(t, err, ErrNonceReplayed)
	})

	t.Run("expired_nonce_refused_and_not_recorded", func(t *testing.T) {
		dir := t.TempDir()
		storePath := filepath.Join(dir, "nonces.jsonl")
		consumer := extConsumer(t, newCustodyFileNonceStore(storePath), filepath.Join(dir, "projection.json"))
		err := consumer.ConsumeBound(ctx, "ext-nonce-expired", time.Now().UTC().Add(-time.Minute), "digest-x")
		assert.ErrorIs(t, err, ErrNonceExpired)
		assert.NoFileExists(t, storePath, "an expired nonce must not be recorded")
	})

	t.Run("record_holds_nonce_not_after_consume_time_and_digest", func(t *testing.T) {
		dir := t.TempDir()
		storePath := filepath.Join(dir, "nonces.jsonl")
		consumer := extConsumer(t, newCustodyFileNonceStore(storePath), filepath.Join(dir, "projection.json"))
		require.NoError(t, consumer.ConsumeBound(ctx, "ext-nonce-fields", future, "digest-fields"))

		raw, err := os.ReadFile(storePath)
		require.NoError(t, err)
		var rec struct {
			Nonce      string     `json:"nonce"`
			NotAfter   time.Time  `json:"not_after"`
			ConsumedAt *time.Time `json:"consumed_at"`
			Digest     string     `json:"request_identity_digest"`
		}
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(string(raw))), &rec))
		assert.Equal(t, "ext-nonce-fields", rec.Nonce)
		assert.WithinDuration(t, future, rec.NotAfter, time.Second)
		require.NotNil(t, rec.ConsumedAt, "consumed_at must be recorded")
		assert.WithinDuration(t, time.Now().UTC(), *rec.ConsumedAt, time.Minute)
		assert.Equal(t, "digest-fields", rec.Digest)
	})

	t.Run("write_failure_is_fail_closed_and_not_recorded", func(t *testing.T) {
		dir := t.TempDir()
		storePath := filepath.Join(dir, "nonces.jsonl")
		store := newCustodyFileNonceStore(storePath)
		store.rename = func(string, string) error { return os.ErrPermission }
		consumer := extConsumer(t, store, filepath.Join(dir, "projection.json"))
		err := consumer.ConsumeBound(ctx, "ext-nonce-write", future, "digest-w")
		require.Error(t, err, "a commit that does not persist must not report success")
		assert.ErrorIs(t, err, ErrNonceProviderUnavailable)

		store.rename = os.Rename
		retry := extConsumer(t, store, filepath.Join(dir, "projection-retry.json"))
		assert.NoError(t, retry.ConsumeBound(ctx, "ext-nonce-write", future, "digest-w"),
			"nothing was recorded, so the nonce is still unconsumed")
	})

	t.Run("held_lock_fails_closed", func(t *testing.T) {
		dir := t.TempDir()
		storePath := filepath.Join(dir, "nonces.jsonl")
		store := newCustodyFileNonceStore(storePath)
		store.lockTimeout = 50 * time.Millisecond
		extWriteFile(t, dir, "nonces.jsonl.lock", "")
		consumer := extConsumer(t, store, filepath.Join(dir, "projection.json"))
		err := consumer.ConsumeBound(ctx, "ext-nonce-lock", future, "digest-l")
		assert.ErrorIs(t, err, ErrNonceProviderUnavailable, "a lock that cannot be acquired must fail closed")

		require.NoError(t, os.Remove(storePath+".lock"))
		assert.NoError(t, consumer.ConsumeBound(ctx, "ext-nonce-lock", future, "digest-l"))
	})

	t.Run("corrupt_store_fails_closed_and_is_not_overwritten", func(t *testing.T) {
		dir := t.TempDir()
		storePath := extWriteFile(t, dir, "nonces.jsonl", "{not json\n")
		consumer := extConsumer(t, newCustodyFileNonceStore(storePath), filepath.Join(dir, "projection.json"))
		err := consumer.ConsumeBound(ctx, "ext-nonce-corrupt", future, "digest-c")
		assert.ErrorIs(t, err, ErrNonceLedgerCorrupt)
		raw, readErr := os.ReadFile(storePath)
		require.NoError(t, readErr)
		assert.Equal(t, "{not json\n", string(raw))
	})
}

func TestExternal_RedemptionFromExternalStore(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	window := func(nonce, digest string, from, to time.Time) CustodyApprovalProof {
		return CustodyApprovalProof{Nonce: nonce, RequestIdentityDigest: digest, NotBeforeUTC: from, NotAfterUTC: to}
	}

	t.Run("external_store_alone_satisfies_redemption", func(t *testing.T) {
		repo := custExecNewRepo(t)
		storePath := filepath.Join(t.TempDir(), "nonces.jsonl")
		consumer := extConsumer(t, newCustodyFileNonceStore(storePath), filepath.Join(t.TempDir(), "projection.json"))
		require.NoError(t, consumer.ConsumeBound(ctx, "ext-redeem-1", now.Add(time.Hour), "digest-r1"))
		proof := window("ext-redeem-1", "digest-r1", now.Add(-time.Hour), now.Add(time.Hour))

		redeemed, err := custodyRedeemedWithStore(repo.ws, repo.shipmentID, "rcpt-r", proof, storePath)
		require.NoError(t, err)
		assert.True(t, redeemed, "a redemption recorded only in the external store must satisfy verify")
		assert.False(t, custodyRedeemed(repo.ws, repo.shipmentID, "rcpt-r", proof),
			"control: the workspace sources alone do not record the redemption")
	})

	t.Run("external_record_outside_token_window_is_not_redemption", func(t *testing.T) {
		repo := custExecNewRepo(t)
		storePath := filepath.Join(t.TempDir(), "nonces.jsonl")
		consumer := extConsumer(t, newCustodyFileNonceStore(storePath), filepath.Join(t.TempDir(), "projection.json"))
		require.NoError(t, consumer.ConsumeBound(ctx, "ext-redeem-2", now.Add(time.Hour), "digest-r2"))
		proof := window("ext-redeem-2", "digest-r2", now.Add(-2*time.Hour), now.Add(-time.Hour))

		redeemed, err := custodyRedeemedWithStore(repo.ws, repo.shipmentID, "rcpt-r", proof, storePath)
		require.NoError(t, err)
		assert.False(t, redeemed)
	})

	t.Run("unreadable_store_fails_closed_even_when_ledger_matches", func(t *testing.T) {
		repo := custExecNewRepo(t)
		ledger, err := OpenNonceLedger(custodyNonceLedgerPath(repo.root))
		require.NoError(t, err)
		require.NoError(t, ledger.RecordBound("ext-redeem-3", now.Add(time.Hour), "digest-r3"))
		storePath := extWriteFile(t, t.TempDir(), "nonces.jsonl", "garbage\n")
		proof := window("ext-redeem-3", "digest-r3", now.Add(-time.Hour), now.Add(time.Hour))

		redeemed, err := custodyRedeemedWithStore(repo.ws, repo.shipmentID, "rcpt-r", proof, storePath)
		assert.Error(t, err, "a configured store that cannot be read is UNVERIFIED, not skipped")
		assert.False(t, redeemed)
	})
}

func TestExternal_LiveAcceptAndVerifyUseExternalStore(t *testing.T) {
	ctx := context.Background()
	storePath := filepath.Join(t.TempDir(), "nonces.jsonl")

	fx := custodyReviewOfferAdoption(t)
	fx.repo.ws.Config.TrustAnchors = []config.TrustAnchor{fx.anchor}
	opts := fx.acceptOpts(fx.token(t, "ext-live-1"))
	opts.NonceStorePath = storePath
	_, err := AcceptCustodyHandoff(ctx, fx.repo.ws, opts)
	require.NoError(t, err, "a live operator accept consumes through the external nonce store")
	assert.FileExists(t, storePath)

	// Remove the workspace-writable redemption records. Only the external store still holds the nonce.
	require.NoError(t, os.Remove(custodyNonceLedgerPath(fx.repo.root)))
	require.NoError(t, os.Remove(events.LogPathForItem(WorkspaceLogsRoot(fx.repo.root), fx.repo.shipmentID)))

	without, err := VerifyCustodyState(ctx, fx.repo.ws, CustodyVerifyOptions{ShipmentID: fx.repo.shipmentID, Branch: custExecFeatureBranch})
	require.NoError(t, err)
	assert.Equal(t, "UNVERIFIED_OPERATOR_PROVENANCE", custExecVerdict(without, custExecCovered),
		"without the external store, the removed workspace records leave no redemption")

	with, err := VerifyCustodyState(ctx, fx.repo.ws, CustodyVerifyOptions{
		ShipmentID: fx.repo.shipmentID, Branch: custExecFeatureBranch, NonceStorePath: storePath,
	})
	require.NoError(t, err)
	assert.Equal(t, "ADOPTED_OPERATOR_VERIFIED", custExecVerdict(with, custExecCovered),
		"the external store alone must satisfy verify")

	other := custodyReviewOfferAdoption(t)
	other.repo.ws.Config.TrustAnchors = []config.TrustAnchor{other.anchor}
	replay := other.acceptOpts(other.token(t, "ext-live-1"))
	replay.NonceStorePath = storePath
	_, err = AcceptCustodyHandoff(ctx, other.repo.ws, replay)
	assert.ErrorIs(t, err, ErrNonceReplayed, "the external store refuses a nonce already consumed in another workspace")
}

func TestExternal_VerifyRefusesInsidePathBeforeShipmentLookup(t *testing.T) {
	ctx := context.Background()
	repo := custExecNewRepo(t)
	inside := extWriteFile(t, repo.root, "pins.yaml", "pins: []\n")

	_, err := VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: "no-such-shipment", OperatorPinsPath: inside})
	assert.ErrorIs(t, err, ErrCustodyExternalPathInWorkspace, "a bad pins path must be refused before the shipment is looked up")

	_, err = VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{
		ShipmentID: "no-such-shipment", NonceStorePath: filepath.Join(repo.root, "nonces.jsonl"),
	})
	assert.ErrorIs(t, err, ErrCustodyExternalPathInWorkspace, "a bad nonce store path must be refused before the shipment is looked up")
}
