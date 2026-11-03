package mcp

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/parser"
)

const (
	custodyTestDefaultBranch = "main"
	custodyTestBranch        = "feat/211-s-workflow-custody-handoff-210-f"
	custodyTestStageFile     = "docs/plans/stage.md"
	custodyTestOperatorFile  = "docs/plans/operator.md"
	custodyTestBaseContent   = "placeholder\n"
	custodyTestRevised       = "revised\n"
	custodyTestMaxResponse   = 10 * 1024
	custodyTestReceiptStage  = "rcpt-stage-ship-1"
	custodyTestReceiptAdopt  = "rcpt-adopt-1"
)

// custodyTestRepo is a git repository on a feature branch with two committed
// covered files and two shipments in an initialized canonical workspace.
type custodyTestRepo struct {
	root           string
	base           string
	stageShip      string
	operatorShip   string
	operatorPriv   ed25519.PrivateKey
	operatorAnchor config.TrustAnchor
}

// custodyTestNonceProvider is a test-only in-memory compare-and-consume provider.
// Production wiring has no default provider, so MCP live operator acceptance
// fails closed unless a host injects one through the context.
type custodyTestNonceProvider struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

func (p *custodyTestNonceProvider) CompareAndConsumeNonce(_ context.Context, nonce string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.seen[nonce]; ok {
		return fmt.Errorf("nonce %q: %w", nonce, core.ErrNonceReplayed)
	}
	p.seen[nonce] = struct{}{}
	return nil
}

type custodyTestNonceAuditor struct{}

func (custodyTestNonceAuditor) RecordNonceConsume(core.NonceConsumeEvent) error { return nil }

// custodyTestMutationResponse is the live offer and accept response shape: the
// core result plus the degraded database marker.
type custodyTestMutationResponse struct {
	core.CustodyMutationResult
	DB string `json:"db,omitempty"`
}

type custodyTestErrorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func custodyTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", root,
		"-c", "user.name=backlogit-test",
		"-c", "user.email=backlogit-test@example.invalid",
		"-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).Output()
	require.NoErrorf(t, err, "git %v", args)
	return strings.TrimSpace(string(out))
}

func custodyTestWrite(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

func custodyTestDigest(content string) string {
	return canonical.HashBytes([]byte(content))
}

// custodyTestPatchDigest mirrors the workflow-custody/patch/v1 digest over the
// base, index, and worktree digests of a covered path.
func custodyTestPatchDigest(base, index, worktree string) string {
	return canonical.HashBytes([]byte("workflow-custody/patch/v1\n" + base + "\n" + index + "\n" + worktree))
}

// custodyTestEntry returns an unstaged modified entry whose pins match the
// committed base and the revised worktree content.
func custodyTestEntry(path, contributor, sourceKind string) core.CustodyEntry {
	base := custodyTestDigest(custodyTestBaseContent)
	worktree := custodyTestDigest(custodyTestRevised)
	return core.CustodyEntry{
		Path: path, ChangeKind: "modified", StagedState: "unstaged_only",
		BaseBlobSHA256: base, IndexSHA256: base, WorktreeSHA256: worktree,
		PatchDigestSHA256: custodyTestPatchDigest(base, base, worktree),
		Contributor:       contributor, SourceKind: sourceKind,
	}
}

func custodyTestJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

// custodyTestNewRepo builds the repository, writes the trust anchors into the
// workspace config on disk, and creates two shipments. The workspace handle is
// closed before returning so each test controls the SQLite cache state.
func custodyTestNewRepo(t *testing.T) *custodyTestRepo {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	custodyTestGit(t, root, "init", "-q")
	custodyTestGit(t, root, "symbolic-ref", "HEAD", "refs/heads/"+custodyTestDefaultBranch)

	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	priv, anchor := custodyTestOperatorAnchor(t)
	custodyTestWriteTrustAnchor(t, backlogitDir, anchor)

	custodyTestWrite(t, root, custodyTestStageFile, custodyTestBaseContent)
	custodyTestWrite(t, root, custodyTestOperatorFile, custodyTestBaseContent)
	custodyTestGit(t, root, "add", "-A")
	custodyTestGit(t, root, "commit", "-q", "-m", "chore: seed custody fixture")
	repo := &custodyTestRepo{
		root:           root,
		base:           custodyTestGit(t, root, "rev-parse", "HEAD"),
		operatorPriv:   priv,
		operatorAnchor: anchor,
	}
	custodyTestGit(t, root, "checkout", "-q", "-b", custodyTestBranch)

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	stage, err := core.CreateShipment(ctx, ws, "custody stage shipment", nil)
	require.NoError(t, err)
	operator, err := core.CreateShipment(ctx, ws, "custody operator shipment", nil)
	require.NoError(t, err)
	require.NoError(t, ws.Close())
	repo.stageShip = stage.ID
	repo.operatorShip = operator.ID
	return repo
}

// custodyTestOperatorAnchor generates an Ed25519 operator key and an inline-PEM trust anchor.
func custodyTestOperatorAnchor(t *testing.T) (ed25519.PrivateKey, config.TrustAnchor) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	pemText := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return priv, config.TrustAnchor{
		ID: "op-mcp-1", Role: "operator", Algo: config.TrustAnchorAlgoEd25519,
		PublicKeyRef: string(pemText), Status: config.TrustAnchorStatusActive,
	}
}

// custodyTestWriteTrustAnchor persists the anchor under trust_anchors in config.yaml,
// preserving the rest of the workspace configuration.
func custodyTestWriteTrustAnchor(t *testing.T, backlogitDir string, anchor config.TrustAnchor) {
	t.Helper()
	cfgPath := filepath.Join(backlogitDir, "config.yaml")
	raw, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	doc := map[string]any{}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	doc["trust_anchors"] = []config.TrustAnchor{anchor}
	out, err := yaml.Marshal(doc)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(cfgPath, out, 0o644))
}

// custodyTestCorruptDB replaces the SQLite cache with bytes that cannot be opened.
func custodyTestCorruptDB(t *testing.T, root string) {
	t.Helper()
	dbPath := filepath.Join(root, ".backlogit", "backlogit.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("not a sqlite database\x00\x01\x02"), 0o644))
	custodyTestRemoveSidecars(t, root)
}

// custodyTestRemoveDB deletes the SQLite cache so the workspace is cold.
func custodyTestRemoveDB(t *testing.T, root string) {
	t.Helper()
	require.NoError(t, os.Remove(filepath.Join(root, ".backlogit", "backlogit.db")))
	custodyTestRemoveSidecars(t, root)
}

func custodyTestRemoveSidecars(t *testing.T, root string) {
	t.Helper()
	for _, suffix := range []string{"-wal", "-shm"} {
		err := os.Remove(filepath.Join(root, ".backlogit", "backlogit.db"+suffix))
		if err != nil && !os.IsNotExist(err) {
			require.NoError(t, err)
		}
	}
}

// custodyTestSnapshot records every file outside .git with its content digest
// and modification time, so any write under root changes the snapshot.
func custodyTestSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
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
		out[filepath.ToSlash(rel)] = canonical.HashBytes(data) + "|" + info.ModTime().UTC().Format(time.RFC3339Nano)
		return nil
	}))
	return out
}

// custodyTestInvoke calls a registered tool in-process with the given arguments.
func custodyTestInvoke(t *testing.T, ctx context.Context, srv *Server, tool string, args map[string]any) *mcplib.CallToolResult {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Name = tool
	req.Params.Arguments = args
	result, err := srv.InvokeTool(ctx, tool, req)
	require.NoError(t, err, "invoke %s", tool)
	require.NotNil(t, result, "invoke %s", tool)
	return result
}

func custodyTestText(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "tool result must be text content")
	return text.Text
}

// custodyTestErrorType returns the structured error code of a refused tool call.
func custodyTestErrorType(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.True(t, result.IsError, "expected a structured tool error, got %s", custodyTestText(t, result))
	var body custodyTestErrorBody
	require.NoError(t, json.Unmarshal([]byte(custodyTestText(t, result)), &body))
	return body.Error
}

func custodyTestMutation(t *testing.T, result *mcplib.CallToolResult) custodyTestMutationResponse {
	t.Helper()
	require.False(t, result.IsError, "mutation must succeed: %s", custodyTestText(t, result))
	var resp custodyTestMutationResponse
	require.NoError(t, json.Unmarshal([]byte(custodyTestText(t, result)), &resp))
	return resp
}

// custodyTestPreview runs the zero-write preview and returns the candidate-set digest.
func custodyTestPreview(t *testing.T, srv *Server, shipmentID, contributorPath, contributor, sourceKind string) core.CustodyPreviewResult {
	t.Helper()
	candidates := []core.CustodyCandidateSpec{{
		Path: contributorPath, ChangeKind: "modified", StagedState: "unstaged_only",
		Contributor: contributor, SourceKind: sourceKind,
	}}
	result := custodyTestInvoke(t, context.Background(), srv, "backlogit_custody_preview", map[string]any{
		"shipment_id": shipmentID,
		"branch":      custodyTestBranch,
		"candidates":  custodyTestJSON(t, candidates),
	})
	require.False(t, result.IsError, "preview must succeed: %s", custodyTestText(t, result))
	var preview core.CustodyPreviewResult
	require.NoError(t, json.Unmarshal([]byte(custodyTestText(t, result)), &preview))
	return preview
}

// custodyTestEnvelope reads the workflow-custody envelope from the shipment
// Markdown source of truth, which does not depend on the SQLite cache.
func custodyTestEnvelope(t *testing.T, root, shipmentID string) core.WorkflowCustodyEnvelope {
	t.Helper()
	ctx := context.Background()
	ro, err := core.NewReadOnlyWorkspace(ctx, root)
	require.NoError(t, err)
	defer func() { require.NoError(t, ro.Close()) }()
	path, err := core.FindArtifactPath(ctx, ro, shipmentID)
	require.NoError(t, err)
	art, _, err := parser.ParseMarkdownFile(path)
	require.NoError(t, err)
	raw, err := json.Marshal(art.CustomFields["workflow_custody"])
	require.NoError(t, err)
	var env core.WorkflowCustodyEnvelope
	require.NoError(t, json.Unmarshal(raw, &env))
	return env
}

// custodyTestNonceContext injects a test-only nonce consumer, the only way a live
// Profile A accept can succeed in this package.
func custodyTestNonceContext(t *testing.T, root string) context.Context {
	t.Helper()
	ledger, err := core.OpenNonceLedger(filepath.Join(root, ".backlogit", "nonce-ledger.json"))
	require.NoError(t, err)
	consumer, err := core.NewNonceConsumer(
		&custodyTestNonceProvider{seen: map[string]struct{}{}},
		ledger,
		custodyTestNonceAuditor{},
	)
	require.NoError(t, err)
	return core.WithCustodyNonceConsumer(context.Background(), consumer)
}

func TestU6_MCPCustodyToolsCanonicalDirectAndColdServer(t *testing.T) {
	t.Run("mcp_custody_preview_and_verify_cold_server_zero_write", func(t *testing.T) {
		ctx := context.Background()
		repo := custodyTestNewRepo(t)
		custodyTestRemoveDB(t, repo.root)
		custodyTestWrite(t, repo.root, custodyTestStageFile, custodyTestRevised)

		srv := NewServerForRoot(repo.root)
		require.Nil(t, srv.Workspace, "server must start cold")
		before := custodyTestSnapshot(t, repo.root)

		preview := custodyTestPreview(t, srv, repo.stageShip, custodyTestStageFile, "stage", "stage_artifact")
		assert.Equal(t, "agent_workflow_handoff", preview.RequiredAuthorityProfile)
		assert.NotEmpty(t, preview.CandidateSetDigest)

		verify := custodyTestInvoke(t, ctx, srv, "backlogit_custody_verify", map[string]any{
			"shipment_id": repo.stageShip,
			"branch":      custodyTestBranch,
		})
		require.False(t, verify.IsError, "verify must succeed: %s", custodyTestText(t, verify))
		var verified core.CustodyVerifyResult
		require.NoError(t, json.Unmarshal([]byte(custodyTestText(t, verify)), &verified))
		assert.NotNil(t, verified.Verdicts)

		assert.Nil(t, srv.Workspace, "read-only tools must not initialize the server workspace")
		assert.Equal(t, before, custodyTestSnapshot(t, repo.root), "preview and verify must not write any file")
		_, err := os.Stat(filepath.Join(repo.root, ".backlogit", "backlogit.db"))
		assert.True(t, os.IsNotExist(err), "preview and verify must not create the SQLite cache")
	})

	t.Run("mcp_custody_offer_and_accept_profile_a_and_profile_b", func(t *testing.T) {
		ctx := context.Background()
		repo := custodyTestNewRepo(t)
		custodyTestCorruptDB(t, repo.root)
		custodyTestWrite(t, repo.root, custodyTestStageFile, custodyTestRevised)
		custodyTestWrite(t, repo.root, custodyTestOperatorFile, custodyTestRevised)

		srv := NewServerForRoot(repo.root)

		// Profile B: stage_to_ship_handoff, live on a server whose SQLite cache is unusable.
		stagePreview := custodyTestPreview(t, srv, repo.stageShip, custodyTestStageFile, "stage", "stage_artifact")
		require.Equal(t, "agent_workflow_handoff", stagePreview.RequiredAuthorityProfile)
		stageEntry := custodyTestEntry(custodyTestStageFile, "stage", "stage_artifact")
		stageSpec := custodyTestJSON(t, core.CustodyReceipt{
			ReceiptID: custodyTestReceiptStage, Revision: 1, Kind: "stage_to_ship_handoff",
			AuthorityProfile: "agent_workflow_handoff", Status: "offered",
			ProducerRole: "stage", AcceptingRole: "ship", AllowedAction: "stage_artifact_commit",
			CandidateSetDigest: stagePreview.CandidateSetDigest, Entries: []core.CustodyEntry{stageEntry},
		})
		offerB := custodyTestMutation(t, custodyTestInvoke(t, ctx, srv, "backlogit_custody_offer", map[string]any{
			"shipment_id": repo.stageShip, "spec": stageSpec,
		}))
		assert.Equal(t, "offered", offerB.Status)
		assert.Equal(t, "agent_workflow_handoff", offerB.AuthorityProfile)
		assert.Equal(t, "unavailable", offerB.DB, "a fallback live offer must carry the degraded database marker")

		acceptB := custodyTestMutation(t, custodyTestInvoke(t, ctx, srv, "backlogit_custody_accept", map[string]any{
			"shipment_id": repo.stageShip, "receipt_id": custodyTestReceiptStage, "by": "ship",
		}))
		assert.Equal(t, "accepted", acceptB.Status)
		assert.Equal(t, "unavailable", acceptB.DB)

		// Profile A: preexisting_adoption, operator_verified, live on the same unusable cache.
		operatorPreview := custodyTestPreview(t, srv, repo.operatorShip, custodyTestOperatorFile, "operator", "operator_code_or_doc")
		require.Equal(t, "operator_verified", operatorPreview.RequiredAuthorityProfile)
		operatorEntry := custodyTestEntry(custodyTestOperatorFile, "operator", "operator_code_or_doc")
		adoptSpec := custodyTestJSON(t, core.CustodyReceipt{
			ReceiptID: custodyTestReceiptAdopt, Revision: 1, Kind: "preexisting_adoption",
			AuthorityProfile: "operator_verified", Status: "offered",
			ProducerRole: "operator", AcceptingRole: "operator", AllowedAction: "preexisting_adoption_commit",
			CandidateSetDigest: operatorPreview.CandidateSetDigest, Entries: []core.CustodyEntry{operatorEntry},
		})
		offerA := custodyTestMutation(t, custodyTestInvoke(t, ctx, srv, "backlogit_custody_offer", map[string]any{
			"shipment_id": repo.operatorShip, "spec": adoptSpec,
		}))
		assert.Equal(t, "offered", offerA.Status)
		assert.Equal(t, "operator_verified", offerA.AuthorityProfile)

		// A bare confirmation never authorizes Profile A.
		bare := custodyTestInvoke(t, ctx, srv, "backlogit_custody_accept", map[string]any{
			"shipment_id": repo.operatorShip, "receipt_id": custodyTestReceiptAdopt, "by": "operator", "confirm": true,
		})
		assert.Equal(t, "authorization_required", custodyTestErrorType(t, bare))

		env := custodyTestEnvelope(t, repo.root, repo.operatorShip)
		digest, err := core.CustodyRequestIdentityDigest(env)
		require.NoError(t, err)
		proof := core.CustodyApprovalProof{
			KeyID: repo.operatorAnchor.ID, Algorithm: "ed25519", Principal: "operator:local", AuthorizedRole: "operator",
			WorkflowID: env.WorkflowID, ShipmentID: repo.operatorShip, ScopeBranch: custodyTestBranch,
			BaseCommitSHA: repo.base, CandidateSetDigest: operatorPreview.CandidateSetDigest,
			AllowedAction: "preexisting_adoption_commit", TargetRevision: 1,
			NotBeforeUTC: time.Now().UTC().Add(-time.Minute), NotAfterUTC: time.Now().UTC().Add(5 * time.Minute),
			Nonce: "mcp-custody-nonce-0001", RequestIdentityDigest: digest,
		}
		token, err := core.MintCustodyApprovalTokenWithSigner(proof, repo.operatorPriv)
		require.NoError(t, err)

		// Without an injected nonce provider the live accept fails closed.
		failClosed := custodyTestInvoke(t, ctx, srv, "backlogit_custody_accept", map[string]any{
			"shipment_id": repo.operatorShip, "receipt_id": custodyTestReceiptAdopt, "by": "operator", "auth_token": token,
		})
		assert.Equal(t, "nonce_provider_unavailable", custodyTestErrorType(t, failClosed))

		acceptA := custodyTestMutation(t, custodyTestInvoke(t, custodyTestNonceContext(t, repo.root), srv, "backlogit_custody_accept", map[string]any{
			"shipment_id": repo.operatorShip, "receipt_id": custodyTestReceiptAdopt, "by": "operator", "auth_token": token,
		}))
		assert.Equal(t, "accepted", acceptA.Status)
		assert.Equal(t, "operator_verified", acceptA.AuthorityProfile)
		assert.Equal(t, "unavailable", acceptA.DB)
	})

	t.Run("mcp_custody_error_mapping_and_non_nil_slices", func(t *testing.T) {
		ctx := context.Background()
		repo := custodyTestNewRepo(t)
		custodyTestRemoveDB(t, repo.root)
		custodyTestWrite(t, repo.root, custodyTestStageFile, custodyTestRevised)

		srv := NewServerForRoot(repo.root)

		refused := custodyTestInvoke(t, ctx, srv, "backlogit_custody_preview", map[string]any{
			"shipment_id": repo.stageShip,
			"branch":      custodyTestDefaultBranch,
			"candidates":  custodyTestJSON(t, []core.CustodyCandidateSpec{{Path: custodyTestStageFile, ChangeKind: "modified", Contributor: "stage", SourceKind: "stage_artifact"}}),
		})
		assert.Equal(t, "custody_default_branch_forbidden", custodyTestErrorType(t, refused))

		verify := custodyTestInvoke(t, ctx, srv, "backlogit_custody_verify", map[string]any{
			"shipment_id": repo.stageShip,
			"branch":      custodyTestBranch,
		})
		require.False(t, verify.IsError)
		verifyText := custodyTestText(t, verify)
		assert.Contains(t, verifyText, `"verdicts":[]`)
		assert.NotContains(t, verifyText, `null`)
		assert.Less(t, len(verifyText), custodyTestMaxResponse)

		preview := custodyTestPreview(t, srv, repo.stageShip, custodyTestStageFile, "stage", "stage_artifact")
		entry := custodyTestEntry(custodyTestStageFile, "stage", "stage_artifact")
		spec := custodyTestJSON(t, core.CustodyReceipt{
			ReceiptID: custodyTestReceiptStage, Revision: 1, Kind: "stage_to_ship_handoff",
			AuthorityProfile: "agent_workflow_handoff", Status: "offered",
			ProducerRole: "stage", AcceptingRole: "ship", AllowedAction: "stage_artifact_commit",
			CandidateSetDigest: preview.CandidateSetDigest, Entries: []core.CustodyEntry{entry},
		})

		before := custodyTestSnapshot(t, repo.root)
		dryRun := custodyTestInvoke(t, ctx, srv, "backlogit_custody_offer", map[string]any{
			"shipment_id": repo.stageShip, "spec": spec, "dry_run": true,
		})
		dryText := custodyTestText(t, dryRun)
		require.False(t, dryRun.IsError, dryText)
		assert.Contains(t, dryText, `"verdicts":[]`)
		assert.Contains(t, dryText, `"warnings":[]`)
		assert.NotContains(t, dryText, `null`)
		assert.Less(t, len(dryText), custodyTestMaxResponse)
		assert.Equal(t, before, custodyTestSnapshot(t, repo.root), "dry-run offer must not write")
		assert.Nil(t, srv.Workspace, "dry-run offer must not initialize the server workspace")

		live := custodyTestInvoke(t, ctx, srv, "backlogit_custody_offer", map[string]any{
			"shipment_id": repo.stageShip, "spec": spec,
		})
		liveText := custodyTestText(t, live)
		resp := custodyTestMutation(t, live)
		assert.Equal(t, "offered", resp.Status)
		assert.NotNil(t, resp.Verdicts)
		assert.NotNil(t, resp.Warnings)
		assert.NotContains(t, liveText, `"db":`, "a healthy live offer must not carry the degraded marker")
		assert.NotContains(t, liveText, `null`)
		assert.Less(t, len(liveText), custodyTestMaxResponse)
	})
}
