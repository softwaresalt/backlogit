package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

const (
	custExecDefaultBranch = "main"
	custExecFeatureBranch = "feat/211-s-workflow-custody-handoff-210-f"
	custExecCovered       = "docs/plans/plan.md"
	custExecBase          = "placeholder\n"
	custExecRevised       = "revised\n"
	custExecCorrelationA  = "0123456789abcdef0123456789abcdef"
	custExecCorrelationB  = "fedcba9876543210fedcba9876543210"
)

// custExecRepo is a git repository on a feature branch with a canonical
// workspace and one shipment. ws is replaced on reopen, so cleanup closes the
// current handle.
type custExecRepo struct {
	root       string
	base       string
	shipmentID string
	ws         *Workspace
}

var custExecSeed struct {
	once    sync.Once
	root    string
	baseSHA string
	err     error
}

// TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery is the RED harness for the
// canonical Markdown/JSONL custody preview, offer, accept, verify, and journal
// recovery behavior (210.004-T). Every behavioral subtest calls the U2 stubs
// first, so each fails on the ErrNotImplemented stub before any later assertion.
func TestU4_CustodyPreviewOfferAcceptVerifyAndRecovery(t *testing.T) {
	t.Run("preview_and_verify_zero_write_and_default_branch_refusal", func(t *testing.T) {
		ctx := context.Background()
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, custExecCovered, custExecRevised)

		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
		require.NoError(t, err)
		assert.Equal(t, custExecFeatureBranch, preview.Branch)
		assert.Equal(t, "agent_workflow_handoff", preview.RequiredAuthorityProfile)
		require.NotEmpty(t, preview.CandidateSetDigest)

		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
			ShipmentID: repo.shipmentID,
			Spec:       custExecStageReceipt(preview.CandidateSetDigest),
		})
		require.NoError(t, err)

		ro, err := NewReadOnlyWorkspace(ctx, repo.root)
		require.NoError(t, err)
		before := custExecSnapshot(t, repo.root)

		roPreview, err := PreviewCustodyCandidates(ctx, ro, custExecPreviewOpts(repo, custExecStageCandidate()))
		require.NoError(t, err)
		assert.Equal(t, preview.CandidateSetDigest, roPreview.CandidateSetDigest)

		verify, err := VerifyCustodyState(ctx, ro, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.NotEqual(t, "WRONG_BRANCH", custExecVerdict(verify, custExecCovered))

		cross, err := VerifyCustodyState(ctx, ro, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: "feat/other-branch"})
		require.NoError(t, err)
		assert.Equal(t, "WRONG_BRANCH", custExecVerdict(cross, custExecCovered))

		_, err = PreviewCustodyCandidates(ctx, ro, CustodyPreviewOptions{
			ShipmentID: repo.shipmentID,
			Branch:     custExecDefaultBranch,
			Candidates: []CustodyCandidateSpec{custExecStageCandidate()},
		})
		assert.ErrorIs(t, err, ErrCustodyDefaultBranchForbidden)

		assert.Equal(t, before, custExecSnapshot(t, repo.root), "read-only preview and verify must not write any file")

		runGit(t, repo.root, "checkout", custExecDefaultBranch)
		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
			ShipmentID: repo.shipmentID,
			Spec:       custExecStageReceipt(preview.CandidateSetDigest),
		})
		assert.ErrorIs(t, err, ErrCustodyDefaultBranchForbidden, "offer on the default branch must fail closed")
	})

	t.Run("profile_b_stage_to_ship_handoff_and_stale_digest_invalidation", func(t *testing.T) {
		ctx := context.Background()
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, custExecCovered, custExecRevised)

		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
		require.NoError(t, err)

		offer, err := OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
			ShipmentID: repo.shipmentID,
			Spec:       custExecStageReceipt(preview.CandidateSetDigest),
		})
		require.NoError(t, err)
		assert.Equal(t, "offered", offer.Status)
		assert.Equal(t, "agent_workflow_handoff", offer.AuthorityProfile)

		art := custExecShipment(t, repo.ws, repo.shipmentID)
		assert.NotEmpty(t, art.CustomFields["workflow_id"])
		assert.Equal(t, custExecFeatureBranch, art.CustomFields["branch"])
		env := custExecEnvelope(t, art)
		require.Len(t, env.Receipts, 1)
		assert.Equal(t, "offered", env.Receipts[0].Status)
		assert.Contains(t, custExecEventTypes(t, repo.root, repo.shipmentID), EventCustodyHandoffOffered)

		accept, err := AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{
			ShipmentID: repo.shipmentID,
			ReceiptID:  "rcpt-stage-ship-1",
			By:         "ship",
		})
		require.NoError(t, err)
		assert.Equal(t, "accepted", accept.Status)

		env = custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
		assert.Equal(t, "accepted", env.Receipts[0].Status)
		assert.Equal(t, "rcpt-stage-ship-1", env.ActiveReceiptID)
		assert.Equal(t, 1, env.CustodyRevision)
		assert.Contains(t, custExecEventTypes(t, repo.root, repo.shipmentID), EventCustodyHandoffAccepted)

		verify, err := VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "HANDOFF_AGENT_VERIFIED", custExecVerdict(verify, custExecCovered))

		custExecWrite(t, repo.root, custExecCovered, custExecRevised+"x")
		stale, err := VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "STALE_DIGEST", custExecVerdict(stale, custExecCovered))

		// A case-only rename must match the on-disk directory entry casing exactly.
		// Comparing the directory listing rather than os.Stat keeps this check
		// correct on case-insensitive filesystems such as Windows NTFS.
		caseOnlyEnv := func(path string) WorkflowCustodyEnvelope {
			return WorkflowCustodyEnvelope{Receipts: []CustodyReceipt{{ReceiptID: "rcpt-case-only", Entries: []CustodyEntry{{
				Path: path, OldPath: "docs/plans/PLAN.md", ChangeKind: "renamed", CaseOnlyRename: true,
			}}}}}
		}
		require.NoError(t, ValidateCustodyRenameCasing(repo.ws, caseOnlyEnv(custExecCovered)),
			"case-only rename whose path matches the on-disk casing must pass")
		err = ValidateCustodyRenameCasing(repo.ws, caseOnlyEnv("docs/plans/Plan.md"))
		assert.ErrorIs(t, err, ErrPartialRenameAdoptionRejected,
			"case-only rename whose casing differs from the on-disk entry must fail closed")
	})

	t.Run("profile_a_operator_adoption_nonce_replay_and_unverified_rejection", func(t *testing.T) {
		ctx := context.Background()
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, custExecCovered, custExecRevised)
		priv, anchor := u3OperatorFixture(t)

		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecOperatorCandidate()))
		require.NoError(t, err)
		assert.Equal(t, "operator_verified", preview.RequiredAuthorityProfile)

		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
			ShipmentID: repo.shipmentID,
			Spec:       custExecAdoptionReceipt(preview.CandidateSetDigest),
		})
		require.NoError(t, err)

		env := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
		digest, err := CustodyRequestIdentityDigest(env)
		require.NoError(t, err)
		token, err := MintCustodyApprovalTokenWithSigner(custExecProof(repo, env, preview.CandidateSetDigest, digest, anchor.ID), priv)
		require.NoError(t, err)

		_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-adopt-1", By: "operator"})
		assert.ErrorIs(t, err, ErrAuthorizationRequired, "a bare confirmation must never adopt operator custody")

		_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-adopt-1", By: "operator", AuthToken: token})
		assert.ErrorIs(t, err, ErrApprovalChannelUnavailable, "a token without a configured trust anchor must fail closed")

		require.NotNil(t, repo.ws.Config)
		repo.ws.Config.TrustAnchors = []config.TrustAnchor{anchor}

		_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-adopt-1", By: "operator", AuthToken: token, DryRun: true})
		require.NoError(t, err, "dry-run verification must succeed without consuming a nonce")
		assert.Equal(t, "offered", custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID)).Receipts[0].Status)

		_, err = AcceptCustodyHandoff(ctx, repo.ws, CustodyAcceptOptions{ShipmentID: repo.shipmentID, ReceiptID: "rcpt-adopt-1", By: "operator", AuthToken: token})
		assert.ErrorIs(t, err, ErrNonceProviderUnavailable, "a live accept without the external nonce provider must fail closed")
		assert.Equal(t, "offered", custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID)).Receipts[0].Status)

		foreign := custExecReceipt("rcpt-foreign-1", "stage_to_ship_handoff", "agent_workflow_handoff", "stage", "ship", "stage_artifact_commit", preview.CandidateSetDigest, custExecOperatorEntry())
		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{ShipmentID: repo.shipmentID, Spec: foreign})
		assert.ErrorIs(t, err, ErrOperatorApprovalRequired, "profile B must not cover operator-contributed ForeignPaths")

		accepted := custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID))
		accepted.ActiveReceiptID = "rcpt-adopt-1"
		accepted.Receipts[0].Status = "accepted"
		accepted.Receipts[0].AcceptedAtUTC = time.Now().UTC()
		accepted.Receipts[0].ApprovalProof = &CustodyApprovalProof{
			KeyID: anchor.ID, Algorithm: "ed25519", Principal: "operator:local", AuthorizedRole: "operator",
			WorkflowID: env.WorkflowID, ShipmentID: repo.shipmentID, ScopeBranch: custExecFeatureBranch,
			BaseCommitSHA: repo.base, CandidateSetDigest: preview.CandidateSetDigest,
			AllowedAction: "preexisting_adoption_commit", TargetRevision: 1,
			NotBeforeUTC: time.Now().UTC().Add(-time.Minute), NotAfterUTC: time.Now().UTC().Add(5 * time.Minute),
			Nonce: "u4-nonce-0002", RequestIdentityDigest: digest, Signature: "bogus-signature",
		}
		custExecWriteCustody(t, repo.ws, accepted)

		verify, err := VerifyCustodyState(ctx, repo.ws, CustodyVerifyOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch})
		require.NoError(t, err)
		assert.Equal(t, "UNVERIFIED_OPERATOR_PROVENANCE", custExecVerdict(verify, custExecCovered))
	})

	t.Run("concurrent_locks_crash_journal_rollback_and_indeterminate_write_contract", func(t *testing.T) {
		ctx := context.Background()
		repo := custExecNewRepo(t)
		custExecWrite(t, repo.root, custExecCovered, custExecRevised)

		preview, err := PreviewCustodyCandidates(ctx, repo.ws, custExecPreviewOpts(repo, custExecStageCandidate()))
		require.NoError(t, err)
		pre := custExecShipment(t, repo.ws, repo.shipmentID)

		_, err = OfferCustodyHandoff(ctx, repo.ws, CustodyOfferOptions{
			ShipmentID: repo.shipmentID,
			Spec:       custExecStageReceipt(preview.CandidateSetDigest),
		})
		require.NoError(t, err)

		journal := custExecJournal(repo.shipmentID, custExecCorrelationA, "intent", "custody_offer", "offered", pre)
		require.NoError(t, validateShipmentLifecycleJournalRecord(journal, custExecCorrelationA),
			"custody_offer must be a registered shipment-operation/v1 lifecycle operation")
		accepted := journal
		accepted.Operation = "custody_accept"
		accepted.Target = "accepted"
		require.NoError(t, validateShipmentLifecycleJournalRecord(accepted, custExecCorrelationA),
			"custody_accept must be a registered shipment-operation/v1 lifecycle operation")

		// Crash after the artifact rewrite, before the commit point: recovery must roll back.
		custExecWriteCustody(t, repo.ws, custExecEnvelope(t, custExecShipment(t, repo.ws, repo.shipmentID)))
		journalPath, err := writeShipmentLifecycleJournalForWorkspace(repo.ws, "shipment-operation-"+custExecCorrelationA+".json", journal)
		require.NoError(t, err)
		repo.reopen(t)
		_, statErr := os.Stat(journalPath)
		assert.True(t, errors.Is(statErr, fs.ErrNotExist), "pending intent journal must be reconciled on reopen")
		_, present := custExecShipment(t, repo.ws, repo.shipmentID).CustomFields["workflow_custody"]
		assert.False(t, present, "pre-commit custody write must be rolled back to the preimage")

		// Crash after the commit point: recovery must never roll the applied write back.
		committed := custExecJournal(repo.shipmentID, custExecCorrelationB, "committed", "custody_offer", "offered", pre)
		custExecWriteCustody(t, repo.ws, WorkflowCustodyEnvelope{
			SchemaVersion:   WorkflowCustodySchemaVersion,
			WorkflowID:      "u4-committed-workflow",
			ShipmentID:      repo.shipmentID,
			ScopeBranch:     custExecFeatureBranch,
			BaseCommitSHA:   repo.base,
			CustodyRevision: 1,
			Receipts:        []CustodyReceipt{custExecStageReceipt(preview.CandidateSetDigest)},
		})
		_, err = writeShipmentLifecycleJournalForWorkspace(repo.ws, "shipment-operation-"+custExecCorrelationB+".json", committed)
		require.NoError(t, err)
		repo.reopen(t)
		_, present = custExecShipment(t, repo.ws, repo.shipmentID).CustomFields["workflow_custody"]
		assert.True(t, present, "a committed custody write must not be rolled back on recovery")

		// Ungoverned mutation of custody state is refused at both write boundaries.
		art := custExecShipment(t, repo.ws, repo.shipmentID)
		tampered := custExecEnvelope(t, art)
		tampered.WorkflowID = "tampered-workflow"
		custExecSetCustody(t, art, tampered)
		_, err = UpdateArtifact(ctx, repo.ws, repo.shipmentID, map[string]any{"custom_fields": art.CustomFields})
		assert.Error(t, err, "generic UpdateArtifact must refuse custom_fields.workflow_custody edits")
		path, err := FindArtifactPath(ctx, repo.ws, repo.shipmentID)
		require.NoError(t, err)
		err = writeArtifactFileGoverned(art, path, path, false, artifactWriteEnvelope{}, nil)
		assert.Error(t, err, "writeArtifactFileGoverned must refuse custody edits without a custody envelope")

		file := parseCustodySource(t, "custody_exec.go")
		require.NotNil(t, file)
		chain := custExecFuncCalling(file, "lockShipmentLifecycleGlobal")
		require.NotNil(t, chain, "custody_exec.go must acquire the custody lock chain in one function")
		for _, entry := range []string{"OfferCustodyHandoff", "AcceptCustodyHandoff"} {
			require.True(t, custExecFuncCalls(custExecFuncNamed(file, entry), chain.Name.Name),
				"%s must acquire the custody lock chain through %s", entry, chain.Name.Name)
		}
		order := custExecFirstCallPositions(chain.Body)
		names := []string{"lockShipmentLifecycleGlobal", "lockShipmentMembership", "LockItemLogCrossProcess", "lockArtifactMutations"}
		for _, name := range names {
			require.Contains(t, order, name, "%s must call %s", chain.Name.Name, name)
		}
		for i := 1; i < len(names); i++ {
			assert.Less(t, order[names[i-1]], order[names[i]], "%s must precede %s", names[i-1], names[i])
		}
	})
}

// custExecNewRepo builds a repository with one committed covered file, a
// feature branch checked out, and a shipment in a read-write workspace.
func custExecNewRepo(t *testing.T) *custExecRepo {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	requireGit(t)
	custExecSeed.once.Do(func() {
		custExecSeed.root, custExecSeed.baseSHA, custExecSeed.err = buildCustExecSeed()
	})
	require.NoError(t, custExecSeed.err, "initialize custody git seed")
	require.NoError(t, copyFixtureGitSeed(custExecSeed.root, root))
	repo := &custExecRepo{root: root, base: custExecSeed.baseSHA}

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	for range 4 {
		// Best-effort speedup for disposable test databases only.
		_, _ = ws.DB.ExecContext(ctx, "PRAGMA synchronous = OFF")
	}
	repo.ws = ws
	disableExecGateForTest(t, ws)
	t.Cleanup(func() {
		if repo.ws != nil {
			_ = repo.ws.Close()
		}
	})
	shipment, err := CreateShipment(ctx, ws, "custody exec shipment", nil)
	require.NoError(t, err)
	repo.shipmentID = shipment.ID
	return repo
}

func buildCustExecSeed() (string, string, error) {
	seed := newFixtureGitSeed("backlogit-custody-git-seed-*")
	// Match initGitRepo without calling testing.T.FailNow inside sync.Once.
	seed.git("init")
	seed.git("config", "user.email", "backlogit-tests@example.invalid")
	seed.git("config", "user.name", "Backlogit Tests")
	seed.git("symbolic-ref", "HEAD", "refs/heads/"+custExecDefaultBranch)
	if seed.err == nil {
		backlogitDir := filepath.Join(seed.root, ".backlogit")
		if err := os.MkdirAll(backlogitDir, 0o755); err != nil {
			seed.err = fmt.Errorf("create custody seed config directory: %w", err)
		} else if err := config.WriteDefaults(backlogitDir); err != nil {
			seed.err = fmt.Errorf("write custody seed defaults: %w", err)
		}
	}
	seed.write(custExecCovered, custExecBase)
	seed.git("add", "-A")
	seed.git("commit", "-m", "chore: seed custody fixture")
	baseSHA := seed.git("rev-parse", "HEAD")
	seed.git("checkout", "-b", custExecFeatureBranch)
	return seed.root, baseSHA, seed.err
}

// fixtureGitSeed latches the first initialization error and stops all further
// I/O. Callers must check err before publishing or copying the immutable seed.
// No testing.T is retained: every consumer can report an initialization failure.
type fixtureGitSeed struct {
	root string
	err  error
}

func newFixtureGitSeed(pattern string) *fixtureGitSeed {
	root, err := os.MkdirTemp(os.TempDir(), pattern)
	if err != nil {
		return &fixtureGitSeed{err: fmt.Errorf("create git seed %s: %w", pattern, err)}
	}
	return &fixtureGitSeed{root: root}
}

func (s *fixtureGitSeed) git(args ...string) string {
	if s.err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", s.root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s.err = fmt.Errorf("git seed %s: %w: %s", strings.Join(args, " "), err, out)
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (s *fixtureGitSeed) write(rel, content string) {
	if s.err != nil {
		return
	}
	path := filepath.Join(s.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		s.err = fmt.Errorf("create git seed parent for %s: %w", rel, err)
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		s.err = fmt.Errorf("write git seed %s: %w", rel, err)
	}
}

// copyFixtureGitSeed copies bytes rather than linking files, including .git, so
// worktree, index, config and ref mutations cannot affect another test or seed.
func copyFixtureGitSeed(source, target string) error {
	err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk git seed: %w", walkErr)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return fmt.Errorf("relativize git seed path: %w", err)
		}
		if rel == "." {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat git seed %s: %w", rel, err)
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			if err := os.MkdirAll(dest, info.Mode().Perm()); err != nil {
				return fmt.Errorf("create git seed copy directory %s: %w", rel, err)
			}
			if err := os.Chmod(dest, info.Mode().Perm()); err != nil {
				return fmt.Errorf("preserve git seed copy directory mode %s: %w", rel, err)
			}
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("copy git seed %s: %w", rel, fs.ErrInvalid)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read git seed %s: %w", rel, err)
		}
		if err := os.WriteFile(dest, data, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write git seed copy %s: %w", rel, err)
		}
		if err := os.Chmod(dest, info.Mode().Perm()); err != nil {
			return fmt.Errorf("preserve git seed copy mode %s: %w", rel, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("copy git fixture seed: %w", err)
	}
	return nil
}

func TestCustodyExecRepoSeedIsolation(t *testing.T) {
	first := custExecNewRepo(t)
	custExecWrite(t, first.root, custExecCovered, "private mutation\n")
	runGit(t, first.root, "config", "user.name", "Private Fixture")
	runGit(t, first.root, "add", custExecCovered)
	runGit(t, first.root, "commit", "-m", "private fixture mutation")
	require.NotEqual(t, first.base, strings.TrimSpace(runGit(t, first.root, "rev-parse", "HEAD")))

	second := custExecNewRepo(t)
	assert.Equal(t, first.base, second.base)
	assert.Equal(t, second.base, strings.TrimSpace(runGit(t, second.root, "rev-parse", "HEAD")))
	assert.Equal(t, custExecFeatureBranch, strings.TrimSpace(runGit(t, second.root, "branch", "--show-current")))
	assert.Equal(t, "Backlogit Tests", strings.TrimSpace(runGit(t, second.root, "config", "user.name")))
	data, err := os.ReadFile(filepath.Join(second.root, filepath.FromSlash(custExecCovered)))
	require.NoError(t, err)
	assert.Equal(t, custExecBase, string(data))
	assert.NotSame(t, first.ws, second.ws)
}

// reopen closes the current workspace handle and opens a fresh one, which runs
// the pending-journal recovery barrier.
func (r *custExecRepo) reopen(t *testing.T) {
	t.Helper()
	require.NoError(t, r.ws.Close())
	ws, err := NewWorkspace(context.Background(), r.root)
	require.NoError(t, err)
	disableExecGateForTest(t, ws)
	r.ws = ws
}

func custExecWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

// custExecDigest is the pinned content digest: SHA-256 hex over LF-normalized bytes.
func custExecDigest(content string) string {
	return canonical.HashBytes([]byte(strings.ReplaceAll(content, "\r\n", "\n")))
}

// custExecPatchDigest pins the adopted patch as a digest over the base, index,
// and worktree digests of the covered path.
func custExecPatchDigest(base, index, worktree string) string {
	return canonical.HashBytes([]byte("workflow-custody/patch/v1\n" + base + "\n" + index + "\n" + worktree))
}

// custExecSnapshot records every file under root with its content digest, size,
// and modification time, so any write under root changes the snapshot.
func custExecSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		out[filepath.ToSlash(rel)] = custExecDigest(string(data)) + "|" + info.ModTime().UTC().Format(time.RFC3339Nano)
		return nil
	}))
	return out
}

func custExecStageCandidate() CustodyCandidateSpec {
	return CustodyCandidateSpec{
		Path: custExecCovered, ChangeKind: "modified", StagedState: "unstaged_only",
		Contributor: "stage", SourceKind: "stage_artifact",
	}
}

func custExecOperatorCandidate() CustodyCandidateSpec {
	return CustodyCandidateSpec{
		Path: custExecCovered, ChangeKind: "modified", StagedState: "unstaged_only",
		Contributor: "operator", SourceKind: "operator_code_or_doc",
	}
}

func custExecPreviewOpts(repo *custExecRepo, candidates ...CustodyCandidateSpec) CustodyPreviewOptions {
	return CustodyPreviewOptions{ShipmentID: repo.shipmentID, Branch: custExecFeatureBranch, Candidates: candidates}
}

func custExecEntry(contributor, sourceKind string) CustodyEntry {
	return CustodyEntry{
		Path:              custExecCovered,
		ChangeKind:        "modified",
		StagedState:       "unstaged_only",
		BaseBlobSHA256:    custExecDigest(custExecBase),
		IndexSHA256:       custExecDigest(custExecBase),
		WorktreeSHA256:    custExecDigest(custExecRevised),
		PatchDigestSHA256: custExecPatchDigest(custExecDigest(custExecBase), custExecDigest(custExecBase), custExecDigest(custExecRevised)),
		Contributor:       contributor,
		SourceKind:        sourceKind,
	}
}

func custExecStageEntry() CustodyEntry { return custExecEntry("stage", "stage_artifact") }

func custExecOperatorEntry() CustodyEntry {
	return custExecEntry("operator", "operator_code_or_doc")
}

func custExecReceipt(id, kind, profile, producer, accepting, action, digest string, entries ...CustodyEntry) CustodyReceipt {
	return CustodyReceipt{
		ReceiptID: id, Revision: 1, Kind: kind, AuthorityProfile: profile, Status: "offered",
		ProducerRole: producer, AcceptingRole: accepting, AllowedAction: action,
		CandidateSetDigest: digest, Entries: entries,
	}
}

func custExecStageReceipt(digest string) CustodyReceipt {
	return custExecReceipt("rcpt-stage-ship-1", "stage_to_ship_handoff", "agent_workflow_handoff", "stage", "ship",
		"stage_artifact_commit", digest, custExecStageEntry())
}

func custExecAdoptionReceipt(digest string) CustodyReceipt {
	return custExecReceipt("rcpt-adopt-1", "preexisting_adoption", "operator_verified", "operator", "operator",
		"preexisting_adoption_commit", digest, custExecOperatorEntry())
}

// custExecProof returns an operator approval proof bound to env and digest, valid for five minutes around now.
func custExecProof(repo *custExecRepo, env WorkflowCustodyEnvelope, candidateDigest, requestDigest, keyID string) CustodyApprovalProof {
	now := time.Now().UTC()
	return CustodyApprovalProof{
		KeyID: keyID, Algorithm: "ed25519", Principal: "operator:local", AuthorizedRole: "operator",
		WorkflowID: env.WorkflowID, ShipmentID: repo.shipmentID, ScopeBranch: custExecFeatureBranch,
		BaseCommitSHA: repo.base, CandidateSetDigest: candidateDigest,
		AllowedAction: "preexisting_adoption_commit", TargetRevision: 1,
		NotBeforeUTC: now.Add(-time.Minute), NotAfterUTC: now.Add(5 * time.Minute),
		Nonce: "u4-nonce-0001", RequestIdentityDigest: requestDigest,
	}
}

func custExecShipment(t *testing.T, ws *Workspace, id string) *models.Artifact {
	t.Helper()
	art, err := findArtifact(context.Background(), ws, id)
	require.NoError(t, err)
	return art
}

func custExecEnvelope(t *testing.T, art *models.Artifact) WorkflowCustodyEnvelope {
	t.Helper()
	raw, err := json.Marshal(art.CustomFields["workflow_custody"])
	require.NoError(t, err)
	env, err := DecodeCustodyEnvelopeJSON(raw)
	require.NoError(t, err)
	return env
}

func custExecSetCustody(t *testing.T, art *models.Artifact, env WorkflowCustodyEnvelope) {
	t.Helper()
	raw, err := json.Marshal(env)
	require.NoError(t, err)
	var value map[string]any
	require.NoError(t, json.Unmarshal(raw, &value))
	if art.CustomFields == nil {
		art.CustomFields = map[string]any{}
	}
	art.CustomFields["workflow_custody"] = value
}

// custExecWriteCustody rewrites the shipment Markdown with env as its custody
// envelope, simulating an artifact rewrite that landed on disk.
func custExecWriteCustody(t *testing.T, ws *Workspace, env WorkflowCustodyEnvelope) {
	t.Helper()
	ctx := context.Background()
	art := custExecShipment(t, ws, env.ShipmentID)
	custExecSetCustody(t, art, env)
	path, err := FindArtifactPath(ctx, ws, env.ShipmentID)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(models.SerializeFrontmatter(art.ToFrontmatterMap(), art.Description)), 0o644))
}

func custExecEventTypes(t *testing.T, root, shipmentID string) []string {
	t.Helper()
	data, err := os.ReadFile(events.LogPathForItem(WorkspaceLogsRoot(root), shipmentID))
	require.NoError(t, err)
	var types []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var ev events.Event
		require.NoError(t, json.Unmarshal([]byte(line), &ev))
		types = append(types, ev.EventType)
	}
	return types
}

func custExecVerdict(res CustodyVerifyResult, path string) string {
	for _, v := range res.Verdicts {
		if v.Path == path {
			return v.Verdict
		}
	}
	return ""
}

func custExecJournal(shipmentID, correlationID, phase, operation, target string, pre *models.Artifact) shipmentLifecycleJournal {
	return shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  correlationID,
		Phase:          phase,
		Operation:      operation,
		RecoveryPolicy: "rollback",
		ShipmentID:     shipmentID,
		Target:         target,
		Preimage:       shipmentLifecyclePreimage{Shipment: pre},
	}
}

// custExecFirstCallPositions maps each called identifier under node to the source
// offset of its first call, so lock acquisition order can be compared.
func custExecFirstCallPositions(node ast.Node) map[string]int {
	positions := map[string]int{}
	ast.Inspect(node, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name := custExecCalleeName(call.Fun); name != "" {
			if _, seen := positions[name]; !seen {
				positions[name] = int(call.Pos())
			}
		}
		return true
	})
	return positions
}

func custExecCalleeName(fun ast.Expr) string {
	switch fn := fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

func custExecFuncNamed(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	return nil
}

// custExecFuncCalling returns the first function in file whose body calls callee.
func custExecFuncCalling(file *ast.File, callee string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && custExecFuncCalls(fn, callee) {
			return fn
		}
	}
	return nil
}

func custExecFuncCalls(fn *ast.FuncDecl, callee string) bool {
	if fn == nil || fn.Body == nil {
		return false
	}
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && custExecCalleeName(call.Fun) == callee {
			found = true
		}
		return !found
	})
	return found
}
