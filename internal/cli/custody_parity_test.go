package cli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/canonical"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
	"github.com/softwaresalt/backlogit/internal/parser"
)

const (
	u7DefaultBranch  = "main"
	u7Branch         = "feat/211-s-workflow-custody-handoff-210-f"
	u7StageFile      = "docs/plans/stage.md"
	u7OperatorFile   = "docs/plans/operator.md"
	u7BaseContent    = "placeholder\n"
	u7RevisedContent = "revised\n"
	u7StageReceipt   = "rcpt-stage-ship-1"
	u7AdoptReceipt   = "rcpt-adopt-1"
	u7NonceID        = "cli-custody-nonce-0001"
	u7OperatorKeyID  = "op-cli-1"
)

// u7Mode selects the surface that drives one custody flow.
type u7Mode string

const (
	u7ModeMCP     u7Mode = "mcp"
	u7ModeCLI     u7Mode = "cli --json"
	u7ModeJSONRPC u7Mode = "cli --jsonrpc"
)

// u7Call is one custody invocation expressed for both surfaces: cli holds the
// arguments after the custody group, tool and mcpArgs the matching MCP call, and
// nonce whether the live accept runs with an injected nonce consumer.
type u7Call struct {
	cli     []string
	tool    string
	mcpArgs map[string]any
	nonce   bool
}

// u7Outcome is one normalized custody result: the success body, or the refusal
// code and message.
type u7Outcome struct {
	Body    any
	Code    string
	Message string
}

// u7CustodyRepo is a git repository on a feature branch with two committed covered
// files and two shipments in an initialized workspace.
type u7CustodyRepo struct {
	root         string
	base         string
	stageShip    string
	operatorShip string
	operatorPriv ed25519.PrivateKey
}

type u7NonceProvider struct {
	seen map[string]struct{}
}

func (p *u7NonceProvider) CompareAndConsumeNonce(_ context.Context, nonce string, _ time.Time) error {
	if _, ok := p.seen[nonce]; ok {
		return fmt.Errorf("nonce %q: %w", nonce, core.ErrNonceReplayed)
	}
	p.seen[nonce] = struct{}{}
	return nil
}

type u7NonceAuditor struct{}

func (u7NonceAuditor) RecordNonceConsume(core.NonceConsumeEvent) error { return nil }

func u7Git(t *testing.T, root string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", root,
		"-c", "user.name=backlogit-test",
		"-c", "user.email=backlogit-test@example.invalid",
		"-c", "commit.gpgsign=false"}, args...)
	out, err := exec.Command("git", full...).Output()
	require.NoErrorf(t, err, "git %v", args)
	return strings.TrimSpace(string(out))
}

func u7WriteFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

func u7JSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return string(raw)
}

// u7OperatorAnchor generates an Ed25519 operator key and an inline-PEM trust anchor.
func u7OperatorAnchor(t *testing.T) (ed25519.PrivateKey, config.TrustAnchor) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKIXPublicKey(pub)
	require.NoError(t, err)
	pemText := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})
	return priv, config.TrustAnchor{
		ID: u7OperatorKeyID, Role: "operator", Algo: config.TrustAnchorAlgoEd25519,
		PublicKeyRef: string(pemText), Status: config.TrustAnchorStatusActive,
	}
}

// u7WriteTrustAnchor persists the anchor under trust_anchors in config.yaml.
func u7WriteTrustAnchor(t *testing.T, backlogitDir string, anchor config.TrustAnchor) {
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

// u7NewCustodyRepo builds a fresh repository with its own workspace and shipments.
func u7NewCustodyRepo(t *testing.T) *u7CustodyRepo {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	u7Git(t, root, "init", "-q")
	u7Git(t, root, "symbolic-ref", "HEAD", "refs/heads/"+u7DefaultBranch)

	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	priv, anchor := u7OperatorAnchor(t)
	u7WriteTrustAnchor(t, backlogitDir, anchor)

	u7WriteFile(t, root, u7StageFile, u7BaseContent)
	u7WriteFile(t, root, u7OperatorFile, u7BaseContent)
	u7Git(t, root, "add", "-A")
	u7Git(t, root, "commit", "-q", "-m", "chore: seed custody fixture")
	repo := &u7CustodyRepo{
		root:         root,
		base:         u7Git(t, root, "rev-parse", "HEAD"),
		operatorPriv: priv,
	}
	u7Git(t, root, "checkout", "-q", "-b", u7Branch)

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

// u7Entry pins a modified, unstaged covered path to its committed and revised content.
func u7Entry(path, contributor, sourceKind string) core.CustodyEntry {
	base := canonical.HashBytes([]byte(u7BaseContent))
	worktree := canonical.HashBytes([]byte(u7RevisedContent))
	patch := canonical.HashBytes([]byte("workflow-custody/patch/v1\n" + base + "\n" + base + "\n" + worktree))
	return core.CustodyEntry{
		Path: path, ChangeKind: "modified", StagedState: "unstaged_only",
		BaseBlobSHA256: base, IndexSHA256: base, WorktreeSHA256: worktree,
		PatchDigestSHA256: patch, Contributor: contributor, SourceKind: sourceKind,
	}
}

func u7PreviewCall(shipment, candidates string) u7Call {
	return u7Call{
		cli:     []string{"preview", shipment, "--candidates", candidates, "--branch", u7Branch},
		tool:    "backlogit_custody_preview",
		mcpArgs: map[string]any{"shipment_id": shipment, "candidates": candidates, "branch": u7Branch},
	}
}

func u7OfferCall(shipment, spec string) u7Call {
	return u7Call{
		cli:     []string{"offer", shipment, "--spec", spec},
		tool:    "backlogit_custody_offer",
		mcpArgs: map[string]any{"shipment_id": shipment, "spec": spec},
	}
}

func u7AcceptCall(shipment, receipt, by, token string, nonce bool) u7Call {
	cli := []string{"accept", shipment, "--receipt-id", receipt, "--by", by}
	args := map[string]any{"shipment_id": shipment, "receipt_id": receipt, "by": by}
	if token != "" {
		cli = append(cli, "--auth-token", token)
		args["auth_token"] = token
	}
	return u7Call{cli: cli, tool: "backlogit_custody_accept", mcpArgs: args, nonce: nonce}
}

func u7VerifyCall(shipment string) u7Call {
	return u7Call{
		cli:     []string{"verify", shipment, "--branch", u7Branch},
		tool:    "backlogit_custody_verify",
		mcpArgs: map[string]any{"shipment_id": shipment, "branch": u7Branch},
	}
}

// u7NonceContext injects a test-only nonce consumer, the only way a live
// operator_verified accept can succeed in this package.
func u7NonceContext(t *testing.T, root string) context.Context {
	t.Helper()
	ledger, err := core.OpenNonceLedger(filepath.Join(root, ".backlogit", "nonce-ledger.json"))
	require.NoError(t, err)
	consumer, err := core.NewNonceConsumer(&u7NonceProvider{seen: map[string]struct{}{}}, ledger, u7NonceAuditor{})
	require.NoError(t, err)
	return core.WithCustodyNonceConsumer(context.Background(), consumer)
}

// u7Envelope reads the workflow custody envelope from the shipment Markdown.
func u7Envelope(t *testing.T, root, shipmentID string) core.WorkflowCustodyEnvelope {
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

// u7ApprovalToken mints a signed operator approval bound to the offered receipt.
func u7ApprovalToken(t *testing.T, repo *u7CustodyRepo, candidateDigest string) string {
	t.Helper()
	env := u7Envelope(t, repo.root, repo.operatorShip)
	digest, err := core.CustodyRequestIdentityDigest(env)
	require.NoError(t, err)
	now := time.Now().UTC()
	proof := core.CustodyApprovalProof{
		KeyID: u7OperatorKeyID, Algorithm: "ed25519", Principal: "operator:local", AuthorizedRole: "operator",
		WorkflowID: env.WorkflowID, ShipmentID: repo.operatorShip, ScopeBranch: u7Branch,
		BaseCommitSHA: repo.base, CandidateSetDigest: candidateDigest,
		AllowedAction: "preexisting_adoption_commit", TargetRevision: 1,
		NotBeforeUTC: now.Add(-time.Minute), NotAfterUTC: now.Add(5 * time.Minute),
		Nonce: u7NonceID, RequestIdentityDigest: digest,
	}
	token, err := core.MintCustodyApprovalTokenWithSigner(proof, repo.operatorPriv)
	require.NoError(t, err)
	return token
}

// u7CommitSHA matches a full Git commit SHA.
var u7CommitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// u7NormalizeString replaces the temporary workspace root, which differs between
// the independent repositories of one parity comparison.
func u7NormalizeString(root, s string) string {
	s = strings.ReplaceAll(s, root, "<root>")
	return strings.ReplaceAll(s, filepath.ToSlash(root), "<root>")
}

// u7Normalize replaces only the fields that are nondeterministic across the two
// independent repositories: timestamps (keys ending in _at) and commit SHAs, which
// embed the wall-clock time of the seed commit. The temporary root is also replaced.
// Every other field must match exactly.
func u7Normalize(root string, v any, key string) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, inner := range val {
			out[k] = u7Normalize(root, inner, k)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, inner := range val {
			out[i] = u7Normalize(root, inner, key)
		}
		return out
	case string:
		if strings.HasSuffix(key, "_at") {
			return "<timestamp>"
		}
		if u7CommitSHA.MatchString(val) {
			return "<commit-sha>"
		}
		return u7NormalizeString(root, val)
	default:
		return v
	}
}

func u7Success(root string, body any) u7Outcome {
	return u7Outcome{Body: u7Normalize(root, body, "")}
}

func u7Refusal(root, code, message string) u7Outcome {
	return u7Outcome{Code: code, Message: u7NormalizeString(root, message)}
}

// u7RefusalFromError splits a CLI refusal rendered as "<code>: <message>".
func u7RefusalFromError(root string, err error) u7Outcome {
	code, message, found := strings.Cut(err.Error(), ": ")
	if !found {
		return u7Refusal(root, err.Error(), "")
	}
	return u7Refusal(root, code, message)
}

// u7ExecuteStdout runs the package-level Execute() with os.Args set to args and
// captures what it writes to os.Stdout. This is the production JSON-RPC entry
// point, which cannot carry a host context.
func u7ExecuteStdout(t *testing.T, args ...string) (string, error) {
	t.Helper()
	outFile, err := os.CreateTemp(t.TempDir(), "u7-stdout-*.txt")
	require.NoError(t, err)
	oldArgs, oldStdout := os.Args, os.Stdout
	defer func() { os.Args, os.Stdout = oldArgs, oldStdout }()
	os.Args = append([]string{"backlogit"}, args...)
	os.Stdout = outFile
	execErr := Execute()
	require.NoError(t, outFile.Close())
	raw, readErr := os.ReadFile(outFile.Name())
	require.NoError(t, readErr)
	return string(raw), execErr
}

// u7EnvelopeOutcome decodes a JSON-RPC 2.0 envelope into a normalized outcome.
// Refusals carry the structured code and message in error.data.
func u7EnvelopeOutcome(t *testing.T, root string, raw []byte) u7Outcome {
	t.Helper()
	var env map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(raw), &env), "JSON-RPC envelope: %s", raw)
	if rpcErr, ok := env["error"].(map[string]any); ok {
		message, _ := rpcErr["message"].(string)
		code := ""
		if data, ok := rpcErr["data"].(map[string]any); ok {
			code, _ = data["error"].(string)
			message, _ = data["message"].(string)
		}
		return u7Refusal(root, code, message)
	}
	return u7Success(root, env["result"])
}

// u7Invoke drives one custody call through the requested surface.
func u7Invoke(t *testing.T, mode u7Mode, root string, c u7Call) u7Outcome {
	t.Helper()
	ctx := context.Background()
	if c.nonce {
		ctx = u7NonceContext(t, root)
	}
	switch mode {
	case u7ModeMCP:
		return u7MCPInvoke(t, ctx, root, c)
	case u7ModeCLI:
		return u7CLIInvoke(t, ctx, root, c)
	default:
		return u7JSONRPCInvoke(t, ctx, root, c)
	}
}

func u7MCPInvoke(t *testing.T, ctx context.Context, root string, c u7Call) u7Outcome {
	t.Helper()
	srv := mcpinternal.NewServerForRoot(root)
	req := mcplib.CallToolRequest{}
	req.Params.Name = c.tool
	req.Params.Arguments = c.mcpArgs
	result, err := srv.InvokeTool(ctx, c.tool, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	if srv.Workspace != nil {
		require.NoError(t, srv.Workspace.Close())
	}
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "MCP result must be text content")
	if result.IsError {
		var body struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		require.NoError(t, json.Unmarshal([]byte(text.Text), &body))
		return u7Refusal(root, body.Error, body.Message)
	}
	var body any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &body))
	return u7Success(root, body)
}

func u7CLIInvoke(t *testing.T, ctx context.Context, root string, c u7Call) u7Outcome {
	t.Helper()
	args := []string{"--cwd", root, "custody"}
	args = append(args, c.cli...)
	args = append(args, "--json")
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(ctx); err != nil {
		return u7RefusalFromError(root, err)
	}
	var body any
	require.NoError(t, json.Unmarshal(out.Bytes(), &body), "CLI stdout: %s", out.String())
	return u7Success(root, body)
}

// u7JSONRPCInvoke drives the --jsonrpc mode. Execute() cannot carry a host nonce
// consumer, so nonce-bearing calls run the root command with its context; the
// success envelope is the same PostRunE wrap that Execute() emits.
func u7JSONRPCInvoke(t *testing.T, ctx context.Context, root string, c u7Call) u7Outcome {
	t.Helper()
	args := []string{"--jsonrpc", "--cwd", root, "custody"}
	args = append(args, c.cli...)
	if c.nonce {
		cmd := NewRootCommand()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(args)
		if err := cmd.ExecuteContext(ctx); err != nil {
			return u7RefusalFromError(root, err)
		}
		return u7EnvelopeOutcome(t, root, out.Bytes())
	}
	raw, execErr := u7ExecuteStdout(t, args...)
	require.NotEmptyf(t, raw, "Execute wrote no JSON-RPC envelope: %v", execErr)
	return u7EnvelopeOutcome(t, root, []byte(raw))
}

// u7RunFlow drives the custody lifecycle through one surface and returns the
// normalized outcome of every step. Each surface gets its own repository.
func u7RunFlow(t *testing.T, mode u7Mode) map[string]u7Outcome {
	t.Helper()
	repo := u7NewCustodyRepo(t)
	u7WriteFile(t, repo.root, u7StageFile, u7RevisedContent)
	u7WriteFile(t, repo.root, u7OperatorFile, u7RevisedContent)

	outcomes := map[string]u7Outcome{}
	step := func(name, wantCode string, c u7Call) u7Outcome {
		t.Helper()
		out := u7Invoke(t, mode, repo.root, c)
		require.Equalf(t, wantCode, out.Code, "%s via %s: refusal %q: %s", name, mode, out.Code, out.Message)
		outcomes[name] = out
		return out
	}

	stageCandidates := u7JSON(t, []core.CustodyCandidateSpec{{
		Path: u7StageFile, ChangeKind: "modified", StagedState: "unstaged_only",
		Contributor: "stage", SourceKind: "stage_artifact",
	}})
	operatorCandidates := u7JSON(t, []core.CustodyCandidateSpec{{
		Path: u7OperatorFile, ChangeKind: "modified", StagedState: "unstaged_only",
		Contributor: "operator", SourceKind: "operator_code_or_doc",
	}})

	previewStage := step("preview_stage", "", u7PreviewCall(repo.stageShip, stageCandidates))
	stageSpec := u7JSON(t, core.CustodyReceipt{
		ReceiptID: u7StageReceipt, Revision: 1, Kind: "stage_to_ship_handoff",
		AuthorityProfile: "agent_workflow_handoff", Status: "offered",
		ProducerRole: "stage", AcceptingRole: "ship", AllowedAction: "stage_artifact_commit",
		CandidateSetDigest: u7Field(t, previewStage, "candidate_set_digest"),
		Entries:            []core.CustodyEntry{u7Entry(u7StageFile, "stage", "stage_artifact")},
	})
	step("offer_stage", "", u7OfferCall(repo.stageShip, stageSpec))
	step("accept_stage", "", u7AcceptCall(repo.stageShip, u7StageReceipt, "ship", "", false))
	step("verify_stage", "", u7VerifyCall(repo.stageShip))

	previewOperator := step("preview_operator", "", u7PreviewCall(repo.operatorShip, operatorCandidates))
	operatorDigest := u7Field(t, previewOperator, "candidate_set_digest")
	adoptSpec := u7JSON(t, core.CustodyReceipt{
		ReceiptID: u7AdoptReceipt, Revision: 1, Kind: "preexisting_adoption",
		AuthorityProfile: "operator_verified", Status: "offered",
		ProducerRole: "operator", AcceptingRole: "operator", AllowedAction: "preexisting_adoption_commit",
		CandidateSetDigest: operatorDigest,
		Entries:            []core.CustodyEntry{u7Entry(u7OperatorFile, "operator", "operator_code_or_doc")},
	})
	step("offer_operator", "", u7OfferCall(repo.operatorShip, adoptSpec))
	step("accept_operator_without_approval", "authorization_required",
		u7AcceptCall(repo.operatorShip, u7AdoptReceipt, "operator", "", false))

	token := u7ApprovalToken(t, repo, operatorDigest)
	step("accept_operator_without_nonce_provider", "nonce_provider_unavailable",
		u7AcceptCall(repo.operatorShip, u7AdoptReceipt, "operator", token, false))
	step("accept_operator", "", u7AcceptCall(repo.operatorShip, u7AdoptReceipt, "operator", token, true))
	step("verify_operator", "", u7VerifyCall(repo.operatorShip))
	return outcomes
}

// u7Field returns a non-empty string field of a successful outcome body.
func u7Field(t *testing.T, out u7Outcome, key string) string {
	t.Helper()
	body, ok := out.Body.(map[string]any)
	require.Truef(t, ok, "outcome has no JSON object body: code=%q message=%q", out.Code, out.Message)
	value, _ := body[key].(string)
	require.NotEmptyf(t, value, "outcome body lacks %q", key)
	return value
}

// u7RunCLI runs one root command invocation against root and returns its stdout.
func u7RunCLI(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := NewRootCommand()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append([]string{"--cwd", root}, args...))
	require.NoErrorf(t, cmd.ExecuteContext(context.Background()), "backlogit %v: %s", args, errOut.String())
	return out.String()
}

// u7RequireReceiptRow asserts the query output contains the accepted receipt row.
func u7RequireReceiptRow(t *testing.T, raw, shipmentID string) {
	t.Helper()
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &rows), "query output: %s", raw)
	var found map[string]any
	for _, row := range rows {
		if row["receipt_id"] == u7StageReceipt {
			found = row
		}
	}
	require.NotNilf(t, found, "query must return the accepted receipt row: %s", raw)
	assert.Equal(t, shipmentID, found["shipment_id"])
	assert.Equal(t, "agent_workflow_handoff", found["authority_profile"])
	assert.Equal(t, "accepted", found["status"])
}

// TestU7_CustodyCLIMCPParityAndRegistryContract pins the backlogit custody CLI
// (210.007-T) against the MCP custody tools: envelope and error-code parity across
// the standard and JSON-RPC modes, promotion of the registry rows to CLI commands,
// and projection of an accepted receipt into the SQLite cache.
func TestU7_CustodyCLIMCPParityAndRegistryContract(t *testing.T) {
	t.Run("cli_custody_preview_offer_accept_verify_and_jsonrpc_parity", func(t *testing.T) {
		mcpFlow := u7RunFlow(t, u7ModeMCP)
		cliFlow := u7RunFlow(t, u7ModeCLI)
		rpcFlow := u7RunFlow(t, u7ModeJSONRPC)

		assert.Equal(t, "operator_verified", u7Field(t, mcpFlow["preview_operator"], "required_authority_profile"))
		assert.Equal(t, "accepted", u7Field(t, mcpFlow["accept_operator"], "status"))
		for name, want := range mcpFlow {
			assert.Equalf(t, want, cliFlow[name], "%s: CLI --json outcome must match MCP", name)
			assert.Equalf(t, want, rpcFlow[name], "%s: CLI --jsonrpc outcome must match MCP", name)
		}
	})

	t.Run("registry_parity_and_authoritative_dispatch", func(t *testing.T) {
		ops := loadRegistryOperations(t)
		root := NewRootCommand()
		for _, name := range []string{"preview_custody", "offer_custody", "accept_custody", "verify_custody"} {
			t.Run(name, func(t *testing.T) {
				op, ok := ops[name]
				require.True(t, ok, "registry must declare %s", name)
				require.NotEmptyf(t, op.CLICommand, "%s must expose a cli_command", name)
				require.Falsef(t, op.MCPOnly, "%s must not remain mcp_only", name)
				path := resolveCLIPath(op.CLICommand)
				cmd := findCommandByPath(root, path)
				require.NotNilf(t, cmd, "cli_command %q must dispatch to a registered command", op.CLICommand)
				assert.Equal(t, path, cmd.CommandPath())
			})
		}
	})

	t.Run("sql_query_projection_after_custody_accept", func(t *testing.T) {
		repo := u7NewCustodyRepo(t)
		u7WriteFile(t, repo.root, u7StageFile, u7RevisedContent)
		candidates := u7JSON(t, []core.CustodyCandidateSpec{{
			Path: u7StageFile, ChangeKind: "modified", StagedState: "unstaged_only",
			Contributor: "stage", SourceKind: "stage_artifact",
		}})
		preview := u7Invoke(t, u7ModeCLI, repo.root, u7PreviewCall(repo.stageShip, candidates))
		require.Empty(t, preview.Code, "preview: %s", preview.Message)
		spec := u7JSON(t, core.CustodyReceipt{
			ReceiptID: u7StageReceipt, Revision: 1, Kind: "stage_to_ship_handoff",
			AuthorityProfile: "agent_workflow_handoff", Status: "offered",
			ProducerRole: "stage", AcceptingRole: "ship", AllowedAction: "stage_artifact_commit",
			CandidateSetDigest: u7Field(t, preview, "candidate_set_digest"),
			Entries:            []core.CustodyEntry{u7Entry(u7StageFile, "stage", "stage_artifact")},
		})
		offer := u7Invoke(t, u7ModeCLI, repo.root, u7OfferCall(repo.stageShip, spec))
		require.Empty(t, offer.Code, "offer: %s", offer.Message)
		accept := u7Invoke(t, u7ModeCLI, repo.root, u7AcceptCall(repo.stageShip, u7StageReceipt, "ship", "", false))
		require.Empty(t, accept.Code, "accept: %s", accept.Message)

		const query = "SELECT shipment_id, receipt_id, authority_profile, status FROM custody_receipts"
		u7RequireReceiptRow(t, u7RunCLI(t, repo.root, "query", query), repo.stageShip)
		u7RunCLI(t, repo.root, "sync")
		u7RequireReceiptRow(t, u7RunCLI(t, repo.root, "query", query), repo.stageShip)
	})
}
