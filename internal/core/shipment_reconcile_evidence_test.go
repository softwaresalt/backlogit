package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

type shipmentReconcileEvidenceFixture struct {
	ws             *Workspace
	merge047       string
	closure047     string
	merge048       string
	closure048     string
	nonMergeHead   string
	hiddenMerge    string
	staleTip       string
	trustedAlias   string
	closureRel     string
	evidenceRefRel string
	outsidePath    string
}

var shipmentReconcileEvidenceSeed struct {
	once    sync.Once
	root    string
	fixture shipmentReconcileEvidenceFixture
	err     error
}

func TestShipmentReconcileDigestHex_DomainSeparatedFieldsPreventBoundaryShiftCollisions(t *testing.T) {
	left := shipmentReconcileDigestHex(
		"shipment-reconcile/test/v1",
		shipmentReconcileScalarField("left", "ab"),
		shipmentReconcileScalarField("right", "c"),
	)
	right := shipmentReconcileDigestHex(
		"shipment-reconcile/test/v1",
		shipmentReconcileScalarField("left", "a"),
		shipmentReconcileScalarField("right", "bc"),
	)

	require.Equal(t, "abc", "ab"+"c")
	require.Equal(t, "abc", "a"+"bc")
	assert.NotEqual(t, left, right, "field-tagged length-prefix framing must distinguish naive concatenation collisions")
}

func TestShipmentReconcileRequestIdentityDigest_RejectsInvalidFields(t *testing.T) {
	validSHA := strings.Repeat("a", 40)
	baseReq := ShipmentShippedReconcileRequest{
		ShipmentID:      "048-S",
		Reason:          "governed repair",
		Actor:           "operator",
		SecondApprover:  "auditor",
		IdempotencyKey:  "idem-167-001",
		MergeSHA:        validSHA,
		ClosureEvidence: filepath.Join("docs", "closure", "missing.md"),
		EvidenceRefs:    []string{filepath.Join("docs", "evidence", "approval.txt")},
	}

	cases := []struct {
		name   string
		mutate func(*ShipmentShippedReconcileRequest)
	}{
		{"blank_reason", func(req *ShipmentShippedReconcileRequest) { req.Reason = " \t " }},
		{"blank_actor", func(req *ShipmentShippedReconcileRequest) { req.Actor = "\n" }},
		{"blank_idempotency_key", func(req *ShipmentShippedReconcileRequest) { req.IdempotencyKey = "   " }},
		{"blank_merge_sha", func(req *ShipmentShippedReconcileRequest) { req.MergeSHA = "   " }},
		{"invalid_merge_sha_shape", func(req *ShipmentShippedReconcileRequest) { req.MergeSHA = "not-a-sha" }},
		{"blank_closure_path", func(req *ShipmentShippedReconcileRequest) { req.ClosureEvidence = "  " }},
		{"blank_second_approver", func(req *ShipmentShippedReconcileRequest) { req.SecondApprover = "  " }},
		{"same_second_approver", func(req *ShipmentShippedReconcileRequest) { req.SecondApprover = req.Actor }},
		{"blank_evidence_ref", func(req *ShipmentShippedReconcileRequest) { req.EvidenceRefs = []string{"ok", "   "} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := baseReq
			tc.mutate(&req)

			_, err := shipmentReconcileRequestIdentityDigest(req)
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileEvidence)
		})
	}
}

func TestShipmentReconcileRequestIdentityDigest_DoesNotReadClosureFile(t *testing.T) {
	req := ShipmentShippedReconcileRequest{
		ShipmentID:      "048-S",
		Reason:          "governed repair",
		Actor:           "operator",
		IdempotencyKey:  "idem-167-001",
		MergeSHA:        strings.Repeat("a", 40),
		ClosureEvidence: filepath.Join("docs", "closure", "does-not-exist.md"),
		EvidenceRefs:    []string{filepath.Join("docs", "evidence", "also-missing.txt")},
	}

	digest, err := shipmentReconcileRequestIdentityDigest(req)
	require.NoError(t, err, "phase-A request identity materialization must not read the closure path")
	assert.Len(t, digest, 64)
}

func TestShipmentReconcileEvidenceDigest_DeterministicAndSensitive(t *testing.T) {
	base := shipmentReconcileEvidenceDigestInput{
		ShipmentID:         "048-S",
		MergeSHA:           strings.Repeat("a", 40),
		TrustedRefName:     "main",
		TrustedRefTip:      strings.Repeat("b", 40),
		ManifestDigest:     "manifest-001",
		ClosureContentHash: strings.Repeat("c", 64),
		EvidenceRefs:       []string{"docs/evidence/approval.txt", "docs/evidence/checklist.txt"},
	}

	baseline := shipmentReconcileEvidenceDigest(base)
	assert.Equal(t, baseline, shipmentReconcileEvidenceDigest(base), "same inputs must hash identically")

	cases := []struct {
		name   string
		mutate func(*shipmentReconcileEvidenceDigestInput)
	}{
		{"shipment_id", func(in *shipmentReconcileEvidenceDigestInput) { in.ShipmentID = "049-S" }},
		{"merge_sha", func(in *shipmentReconcileEvidenceDigestInput) { in.MergeSHA = strings.Repeat("d", 40) }},
		{"trusted_ref_name", func(in *shipmentReconcileEvidenceDigestInput) { in.TrustedRefName = "release/trusted" }},
		{"trusted_ref_tip", func(in *shipmentReconcileEvidenceDigestInput) { in.TrustedRefTip = strings.Repeat("e", 40) }},
		{"manifest_digest", func(in *shipmentReconcileEvidenceDigestInput) { in.ManifestDigest = "manifest-002" }},
		{"closure_content_hash", func(in *shipmentReconcileEvidenceDigestInput) { in.ClosureContentHash = strings.Repeat("f", 64) }},
		{"evidence_ref_value", func(in *shipmentReconcileEvidenceDigestInput) { in.EvidenceRefs[1] = "docs/evidence/other.txt" }},
		{"evidence_ref_order", func(in *shipmentReconcileEvidenceDigestInput) {
			in.EvidenceRefs[0], in.EvidenceRefs[1] = in.EvidenceRefs[1], in.EvidenceRefs[0]
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := base
			mutated.EvidenceRefs = append([]string(nil), base.EvidenceRefs...)
			tc.mutate(&mutated)
			assert.NotEqual(t, baseline, shipmentReconcileEvidenceDigest(mutated), "changing %s must change the digest", tc.name)
		})
	}
}

func TestPrepareShipmentReconcileEvidence_HappyPathUsesDefaultTrustedRefAndBuildsCanonicalEvent(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}

	result, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, fixture.request("048-S", fixture.merge048), fixedShipmentReconcileEvidenceInput())
	require.NoError(t, err)
	assert.Len(t, result.RequestIdentityDigest, 64)
	assert.Len(t, result.ClosureContentHash, 64)
	assert.Len(t, result.EvidenceDigest, 64)
	assert.Len(t, result.EventDigest, 64)
	assert.Equal(t, "main", result.TrustedRefName)
	assert.Equal(t, fixture.nonMergeHead, result.TrustedRefTip)

	require.NoError(t, ValidateShipmentReconciledShippedEvent(result.EventBytes, result.EventBytes, result.EventDigest, ""))
	assert.Equal(t, result.EventDigest, ShipmentReconciledShippedEventDigest(result.EventBytes))

	var wrapper events.Event
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(result.EventBytes), &wrapper))
	assert.Equal(t, "backlogit", wrapper.Actor)
	assert.Equal(t, "048-S", wrapper.ItemID)
	assert.Equal(t, EventShipmentReconciledShipped, wrapper.EventType)
	assert.Equal(t, fixture.merge048, wrapper.CommitSHA)

	delta, err := decodeShipmentReconciledShippedDelta(wrapper.Delta)
	require.NoError(t, err)
	assert.Equal(t, string(ShipmentActive), delta.Before.ArchivedStatus)
	assert.Equal(t, string(ShipmentShipped), delta.After.ArchivedStatus)
	assert.Equal(t, result.RequestIdentityDigest, delta.RequestIdentityDigest)
	assert.Equal(t, result.TrustedRefName, delta.TrustedRefName)
	assert.Equal(t, result.TrustedRefTip, delta.TrustedRefTip)
	assert.Equal(t, filepath.ToSlash(filepath.Clean(fixture.closureRel)), delta.Evidence.ClosureEvidence)
	assert.Equal(t, []string{filepath.ToSlash(filepath.Clean(fixture.evidenceRefRel))}, delta.Evidence.EvidenceRefs)
	assert.Equal(t, result.ClosureContentHash, delta.Evidence.ClosureContentHash)
	assert.Equal(t, result.EvidenceDigest, delta.Evidence.EvidenceDigest)
	assert.Equal(t, []string{"049-F", "049.005-T"}, delta.Evidence.MemberTerminalIDs)
}

func TestPrepareShipmentReconcileEvidence_RejectsWrongDeliveryMergeRoleOrShipment(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}

	cases := []struct {
		name       string
		shipmentID string
		mergeSHA   string
	}{
		{name: "post_merge_closure_sha_rejected", shipmentID: "048-S", mergeSHA: fixture.closure048},
		{name: "other_shipment_feature_sha_rejected", shipmentID: "048-S", mergeSHA: fixture.merge047},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, fixture.request(tc.shipmentID, tc.mergeSHA), fixedShipmentReconcileEvidenceInput())
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileEvidence)
		})
	}
}

func TestPrepareShipmentReconcileEvidence_RejectsReachableNonMergeCommit(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}
	closureRel := fixture.writeClosureFile(t, "docs/closure/non-merge-048.md", fixture.syntheticClosureContent(fixture.nonMergeHead, fixture.closure048))
	req := fixture.request("048-S", fixture.nonMergeHead)
	req.ClosureEvidence = closureRel

	_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileEvidence)
}

func TestPrepareShipmentReconcileEvidence_RejectsUntrustedOrUnreachableTrustedRefTips(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)

	cases := []struct {
		name        string
		trustedRefs []string
		mergeSHA    string
		closureRel  string
	}{
		{
			name:        "merge_only_reachable_from_untrusted_branch",
			trustedRefs: []string{"main"},
			mergeSHA:    fixture.hiddenMerge,
			closureRel:  fixture.writeClosureFile(t, "docs/closure/hidden-048.md", fixture.syntheticClosureContent(fixture.hiddenMerge, fixture.closure048)),
		},
		{
			name:        "merge_not_reachable_from_pinned_trusted_tip",
			trustedRefs: []string{"stale-tip"},
			mergeSHA:    fixture.merge048,
			closureRel:  fixture.closureRel,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture.ws.Config.Reconcile = &config.ReconcileConfig{TrustedRefs: tc.trustedRefs}
			req := fixture.request("048-S", tc.mergeSHA)
			req.ClosureEvidence = tc.closureRel

			_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileEvidence)
		})
	}
}

func TestPrepareShipmentReconcileEvidence_RejectsClosurePathTraversalAndSymlinkEscape(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}

	outsideClosure := fixture.writeOutsideClosureFile(t, fixture.syntheticClosureContent(fixture.merge048, fixture.closure048))

	t.Run("path_traversal_escape", func(t *testing.T) {
		req := fixture.request("048-S", fixture.merge048)
		req.ClosureEvidence = filepath.Join("..", filepath.Base(outsideClosure))

		_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileEvidence)
	})

	t.Run("symlink_escape", func(t *testing.T) {
		linkRel := filepath.Join("docs", "closure", "escape-link.md")
		linkPath := filepath.Join(fixture.ws.RootPath, filepath.FromSlash(filepath.ToSlash(linkRel)))
		require.NoError(t, os.MkdirAll(filepath.Dir(linkPath), 0o755))
		if err := os.Symlink(outsideClosure, linkPath); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}

		_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, fixture.request("048-S", fixture.merge048, linkRel), fixedShipmentReconcileEvidenceInput())
		require.Error(t, err)
		assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileEvidence)
	})
}

func TestPrepareShipmentReconcileEvidence_RejectsInvalidEvidenceRefs(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}

	cases := []struct {
		name string
		refs []string
	}{
		{name: "blank_ref", refs: []string{fixture.evidenceRefRel, "  "}},
		{name: "path_escape", refs: []string{filepath.Join("..", filepath.Base(fixture.outsidePath))}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := fixture.request("048-S", fixture.merge048)
			req.EvidenceRefs = tc.refs

			_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
			require.Error(t, err)
			assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileEvidence)
		})
	}
}

// TestPrepareShipmentReconcileEvidence_RejectsAmbiguousDuplicateShipmentSections
// covers PR #440 review round 2, finding 2: a closure document with TWO
// sections whose headings both exactly match the same shipment ID (one with
// a legitimate merge reference, one with a different/conflicting one) must
// fail closed rather than silently accept whichever section is found first.
func TestPrepareShipmentReconcileEvidence_RejectsAmbiguousDuplicateShipmentSections(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}

	duplicateContent := strings.TrimSpace(`---
shipments:
  - 048-S
---

# Combined closure summary

### 048-S
Merged as PR #101 (feature), commit `+shortSHA(fixture.merge048)+`, and PR #102 (post-merge closure), commit `+shortSHA(fixture.closure048)+`.

### 048-S
Merged as PR #201 (feature), commit `+shortSHA(fixture.hiddenMerge)+`, and PR #202 (post-merge closure), commit `+shortSHA(fixture.closure048)+`.
`) + "\n"
	closureRel := fixture.writeClosureFile(t, "docs/closure/duplicate-048.md", duplicateContent)
	req := fixture.request("048-S", fixture.merge048)
	req.ClosureEvidence = closureRel

	_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, req, fixedShipmentReconcileEvidenceInput())
	require.Error(t, err, "a closure document with two conflicting exact-match sections for the same shipment must fail closed")
	assert.ErrorIs(t, err, blerrors.ErrShipmentReconcileEvidence)
	assert.Contains(t, err.Error(), "ambiguous", "the error must identify the ambiguous-section cause, not a generic parse failure")
}

func setupShipmentReconcileEvidenceFixture(t *testing.T) shipmentReconcileEvidenceFixture {
	t.Helper()

	ws := setupShipmentWorkspace(t)
	requireGit(t)
	shipmentReconcileEvidenceSeed.once.Do(func() {
		shipmentReconcileEvidenceSeed.root, shipmentReconcileEvidenceSeed.fixture,
			shipmentReconcileEvidenceSeed.err = buildShipmentReconcileEvidenceSeed()
	})
	require.NoError(t, shipmentReconcileEvidenceSeed.err, "initialize reconcile git seed")
	require.NoError(t, copyFixtureGitSeed(shipmentReconcileEvidenceSeed.root, ws.RootPath))

	fixture := shipmentReconcileEvidenceSeed.fixture
	fixture.ws = ws
	fixture.closureRel = filepath.Join("docs", "closure", "2026-09-01-047-s-048-s-closure-summary.md")
	fixture.evidenceRefRel = filepath.Join("docs", "evidence", "approval.txt")
	fixture.outsidePath = filepath.Join(filepath.Dir(ws.RootPath), "outside-evidence.txt")

	fixture.writeClosureFile(t, fixture.closureRel, fixture.syntheticClosureContent(fixture.merge048, fixture.closure048))
	evidenceAbs := filepath.Join(ws.RootPath, filepath.FromSlash(filepath.ToSlash(fixture.evidenceRefRel)))
	require.NoError(t, os.MkdirAll(filepath.Dir(evidenceAbs), 0o755))
	require.NoError(t, os.WriteFile(evidenceAbs, []byte("approved\n"), 0o644))
	require.NoError(t, os.WriteFile(fixture.outsidePath, []byte("outside\n"), 0o644))

	return fixture
}

func buildShipmentReconcileEvidenceSeed() (string, shipmentReconcileEvidenceFixture, error) {
	seed := newFixtureGitSeed("backlogit-reconcile-git-seed-*")
	// Match initEvidenceGitRepo without allowing FailNow to poison sync.Once.
	seed.git("-c", "init.defaultBranch=main", "init")
	seed.git("config", "user.email", "backlogit-tests@example.invalid")
	seed.git("config", "user.name", "Backlogit Tests")
	writeCommit := func(rel, content, msg string) string {
		seed.write(rel, content)
		seed.git("add", filepath.ToSlash(rel))
		seed.git("commit", "-m", msg)
		return seed.git("rev-parse", "HEAD")
	}
	mergeBranch := func(branch, msg string) string {
		seed.git("merge", "--no-ff", branch, "-m", msg)
		return seed.git("rev-parse", "HEAD")
	}

	base := writeCommit("README.md", "base\n", "base")

	seed.git("checkout", "-b", "feature-047")
	_ = writeCommit("shipments/047.txt", "feature 047\n", "feature 047 work")
	seed.git("checkout", "main")
	merge047 := mergeBranch("feature-047", "merge 047 feature")
	seed.git("branch", "stale-tip", merge047)

	seed.git("checkout", "-b", "closure-047")
	_ = writeCommit("closures/047.txt", "closure 047\n", "closure 047 work")
	seed.git("checkout", "main")
	closure047 := mergeBranch("closure-047", "merge 047 closure")

	seed.git("checkout", "-b", "feature-048")
	_ = writeCommit("shipments/048.txt", "feature 048\n", "feature 048 work")
	seed.git("checkout", "main")
	merge048 := mergeBranch("feature-048", "merge 048 feature")

	seed.git("checkout", "-b", "closure-048")
	_ = writeCommit("closures/048.txt", "closure 048\n", "closure 048 work")
	seed.git("checkout", "main")
	closure048 := mergeBranch("closure-048", "merge 048 closure")

	nonMergeHead := writeCommit("post.txt", "post merge plain commit\n", "post merge plain commit")
	seed.git("branch", "release/trusted", nonMergeHead)

	seed.git("checkout", "-b", "hidden-main", base)
	_ = writeCommit("hidden/main.txt", "hidden main\n", "hidden main commit")
	seed.git("checkout", "-b", "hidden-feature")
	_ = writeCommit("hidden/feature.txt", "hidden feature\n", "hidden feature commit")
	seed.git("checkout", "hidden-main")
	hiddenMerge := mergeBranch("hidden-feature", "merge hidden feature")
	seed.git("checkout", "main")

	fixture := shipmentReconcileEvidenceFixture{
		merge047:     merge047,
		closure047:   closure047,
		merge048:     merge048,
		closure048:   closure048,
		nonMergeHead: nonMergeHead,
		hiddenMerge:  hiddenMerge,
		staleTip:     merge047,
		trustedAlias: nonMergeHead,
	}
	return seed.root, fixture, seed.err
}

func TestShipmentReconcileEvidenceSeedIsolation(t *testing.T) {
	first := setupShipmentReconcileEvidenceFixture(t)
	originalReadme, err := os.ReadFile(filepath.Join(first.ws.RootPath, "README.md"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(first.ws.RootPath, "README.md"), []byte("private mutation\n"), 0o644))
	runGit(t, first.ws.RootPath, "branch", "-f", "stale-tip", first.nonMergeHead)
	runGit(t, first.ws.RootPath, "config", "user.name", "Private Fixture")

	second := setupShipmentReconcileEvidenceFixture(t)
	assert.Equal(t, first.merge047, second.merge047)
	assert.Equal(t, first.closure047, second.closure047)
	assert.Equal(t, first.merge048, second.merge048)
	assert.Equal(t, first.closure048, second.closure048)
	assert.Equal(t, first.nonMergeHead, second.nonMergeHead)
	assert.Equal(t, first.hiddenMerge, second.hiddenMerge)
	assert.Equal(t, first.staleTip, second.staleTip)
	assert.Equal(t, second.staleTip, strings.TrimSpace(runGit(t, second.ws.RootPath, "rev-parse", "stale-tip")))
	assert.Equal(t, second.nonMergeHead, strings.TrimSpace(runGit(t, second.ws.RootPath, "rev-parse", "HEAD")))
	assert.Equal(t, "main", strings.TrimSpace(runGit(t, second.ws.RootPath, "branch", "--show-current")))
	assert.Equal(t, "Backlogit Tests", strings.TrimSpace(runGit(t, second.ws.RootPath, "config", "user.name")))
	for _, rel := range []string{
		"README.md",
		"shipments/047.txt",
		"shipments/048.txt",
		"closures/047.txt",
		"closures/048.txt",
		"post.txt",
	} {
		// Checkout may apply ambient core.autocrlf. The copy must preserve the
		// exact checked-out seed bytes and modes, not assume LF on Windows.
		seedPath := filepath.Join(shipmentReconcileEvidenceSeed.root, filepath.FromSlash(rel))
		copyPath := filepath.Join(second.ws.RootPath, filepath.FromSlash(rel))
		want, err := os.ReadFile(seedPath)
		require.NoError(t, err)
		data, err := os.ReadFile(copyPath)
		require.NoError(t, err)
		assert.Equal(t, want, data, rel)
		seedInfo, err := os.Stat(seedPath)
		require.NoError(t, err)
		copyInfo, err := os.Stat(copyPath)
		require.NoError(t, err)
		assert.Equal(t, seedInfo.Mode(), copyInfo.Mode(), rel)
		if rel == "README.md" {
			assert.Equal(t, originalReadme, data, "worktree mutation must not affect the seed")
		}
	}
	_, err = os.Stat(filepath.Join(second.ws.RootPath, "hidden"))
	assert.ErrorIs(t, err, os.ErrNotExist, "main must not contain hidden branch worktree files")
}

func fixedShipmentReconcileEvidenceInput() shipmentReconcileEvidenceInput {
	return shipmentReconcileEvidenceInput{
		ManifestDigest:       "manifest-001",
		MemberTerminalIDs:    []string{"049-F", "049.005-T"},
		BeforeArchivedStatus: string(ShipmentActive),
		Timestamp:            time.Date(2026, 9, 12, 21, 28, 0, 0, time.UTC),
	}
}

func (f shipmentReconcileEvidenceFixture) request(shipmentID, mergeSHA string, closureRelOverride ...string) ShipmentShippedReconcileRequest {
	closureRel := f.closureRel
	if len(closureRelOverride) > 0 {
		closureRel = closureRelOverride[0]
	}
	return ShipmentShippedReconcileRequest{
		ShipmentID:      shipmentID,
		Reason:          "governed repair",
		Actor:           "operator",
		SecondApprover:  "auditor",
		IdempotencyKey:  "idem-167-001",
		MergeSHA:        mergeSHA,
		ClosureEvidence: closureRel,
		EvidenceRefs:    []string{f.evidenceRefRel},
	}
}

func (f shipmentReconcileEvidenceFixture) syntheticClosureContent(deliveryMergeSHA, closureMergeSHA string) string {
	return strings.TrimSpace(`---
shipments:
  - 047-S
  - 048-S
---

# Combined closure summary

### 047-S
Merged as PR #83 (feature), commit `+shortSHA(f.merge047)+`, and PR #84 (post-merge closure), commit `+shortSHA(f.closure047)+`.

### 048-S
Merged as PR #101 (feature), commit `+shortSHA(deliveryMergeSHA)+`, and PR #102 (post-merge closure), commit `+shortSHA(closureMergeSHA)+`.
`) + "\n"
}

func (f shipmentReconcileEvidenceFixture) writeClosureFile(t *testing.T, rel, content string) string {
	t.Helper()
	abs := filepath.Join(f.ws.RootPath, filepath.FromSlash(filepath.ToSlash(rel)))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
	return rel
}

func (f shipmentReconcileEvidenceFixture) writeOutsideClosureFile(t *testing.T, content string) string {
	t.Helper()
	outside := filepath.Join(filepath.Dir(f.ws.RootPath), "outside-closure.md")
	require.NoError(t, os.WriteFile(outside, []byte(content), 0o644))
	return outside
}

func shortSHA(full string) string {
	if len(full) < 7 {
		return full
	}
	return full[:7]
}

func closureContentHash(t *testing.T, root, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(filepath.ToSlash(rel))))
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func TestPrepareShipmentReconcileEvidence_PersistsExpectedClosureHash(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{}

	result, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, fixture.request("048-S", fixture.merge048), fixedShipmentReconcileEvidenceInput())
	require.NoError(t, err)
	assert.Equal(t, closureContentHash(t, fixture.ws.RootPath, fixture.closureRel), result.ClosureContentHash)
}

func TestErrShipmentReconcileEvidence_SentinelWrapping(t *testing.T) {
	fixture := setupShipmentReconcileEvidenceFixture(t)
	fixture.ws.Config.Reconcile = &config.ReconcileConfig{TrustedRefs: []string{"stale-tip"}}

	_, err := prepareShipmentReconcileEvidence(context.Background(), fixture.ws, fixture.request("048-S", fixture.merge048), fixedShipmentReconcileEvidenceInput())
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrShipmentReconcileEvidence))
}
