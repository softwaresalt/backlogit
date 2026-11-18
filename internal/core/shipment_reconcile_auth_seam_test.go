package core

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// seamNonceProvider is an in-memory external compare-and-consume authority.
type seamNonceProvider struct {
	mu       sync.Mutex
	consumed map[string]int
}

func (p *seamNonceProvider) CompareAndConsumeNonce(_ context.Context, nonce string, _ time.Time) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.consumed == nil {
		p.consumed = map[string]int{}
	}
	if p.consumed[nonce] > 0 {
		return ErrNonceReplayed
	}
	p.consumed[nonce]++
	return nil
}

func (p *seamNonceProvider) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.consumed)
}

type seamConsumeAuditor struct{}

func (seamConsumeAuditor) RecordNonceConsume(NonceConsumeEvent) error { return nil }

type seamRejectionAuditor struct{ events []TokenRejectionEvent }

func (a *seamRejectionAuditor) RecordTokenRejection(ev TokenRejectionEvent) error {
	a.events = append(a.events, ev)
	return nil
}

// authSeamFixture is a reconcilable archived shipment with an operator key.
type authSeamFixture struct {
	ws         *Workspace
	shipmentID string
	req        ShipmentShippedReconcileRequest
	digest     string
	priv       ed25519.PrivateKey
	key        ResolvedKey
	pub        *VerificationKey
	provider   *seamNonceProvider
	rejections *seamRejectionAuditor
	nonces     *NonceConsumer
}

var authSeamSeed struct {
	once           sync.Once
	root           string
	shipmentID     string
	memberID       string
	closureRel     string
	evidenceRefRel string
	req            ShipmentShippedReconcileRequest
	digest         string
	priv           ed25519.PrivateKey
	key            ResolvedKey
	pub            *VerificationKey
	err            error
}

func buildAuthSeamSeed(ctx context.Context) (err error) {
	shipmentReconcileEvidenceSeed.once.Do(func() {
		shipmentReconcileEvidenceSeed.root, shipmentReconcileEvidenceSeed.fixture,
			shipmentReconcileEvidenceSeed.err = buildShipmentReconcileEvidenceSeed()
	})
	if shipmentReconcileEvidenceSeed.err != nil {
		return fmt.Errorf("initialize auth seam git seed: %w", shipmentReconcileEvidenceSeed.err)
	}

	shipmentWorkspaceSeed.once.Do(func() {
		shipmentWorkspaceSeed.root, shipmentWorkspaceSeed.err = buildShipmentWorkspaceSeed(
			ctx, "backlogit-shipment-workspace-seed-*", nil)
	})
	if shipmentWorkspaceSeed.err != nil {
		return fmt.Errorf("initialize auth seam workspace seed: %w", shipmentWorkspaceSeed.err)
	}

	root, err := os.MkdirTemp(os.TempDir(), "backlogit-auth-seam-seed-*")
	if err != nil {
		return fmt.Errorf("create auth seam seed dir: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(root)
		}
	}()

	if err := copyFixtureGitSeed(shipmentReconcileEvidenceSeed.root, root); err != nil {
		return fmt.Errorf("copy auth seam git seed: %w", err)
	}

	storage := filepath.Join(root, ".backlogit")
	if err := os.MkdirAll(storage, 0o755); err != nil {
		return fmt.Errorf("create auth seam seed storage: %w", err)
	}
	if err := copyFixtureGitSeed(filepath.Join(shipmentWorkspaceSeed.root, ".backlogit"), storage); err != nil {
		return fmt.Errorf("copy auth seam workspace seed: %w", err)
	}

	ws, err := NewDiagnosticWorkspace(ctx, root)
	if err != nil {
		return fmt.Errorf("open auth seam seed workspace: %w", err)
	}
	defer func() {
		if closeErr := ws.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close auth seam seed workspace: %w", closeErr))
		}
	}()
	ws.GateBroker = nil
	ws.Config.Reconcile = &config.ReconcileConfig{}

	feature, err := CreateArtifact(ctx, ws, "167.014-T member feature", "feature")
	if err != nil {
		return fmt.Errorf("create auth seam member feature: %w", err)
	}
	member, err := CreateArtifact(ctx, ws, "167.014-T member task", "task", WithParent(feature.ID))
	if err != nil {
		return fmt.Errorf("create auth seam member task: %w", err)
	}
	if _, err := setArtifactStatus(ctx, ws, member.ID, models.StatusDone, "167.014-T test fixture"); err != nil {
		return fmt.Errorf("mark auth seam member done: %w", err)
	}

	shipment, err := CreateShipment(ctx, ws, "167.014-T archived shipment", []string{member.ID})
	if err != nil {
		return fmt.Errorf("create auth seam shipment: %w", err)
	}
	if _, err := ClaimShipment(ctx, ws, shipment.ID); err != nil {
		return fmt.Errorf("claim auth seam shipment: %w", err)
	}
	if _, err := ArchiveItem(ctx, ws.DB, ws, shipment.ID); err != nil {
		return fmt.Errorf("archive auth seam shipment: %w", err)
	}
	archived, err := findArtifact(ctx, ws, shipment.ID)
	if err != nil {
		return fmt.Errorf("reload archived auth seam shipment: %w", err)
	}

	evFixture := shipmentReconcileEvidenceSeed.fixture
	evidenceRefRel := filepath.Join("docs", "evidence", "approval.txt")
	evidenceAbs := filepath.Join(root, filepath.FromSlash(filepath.ToSlash(evidenceRefRel)))
	if err := os.MkdirAll(filepath.Dir(evidenceAbs), 0o755); err != nil {
		return fmt.Errorf("create auth seam evidence dir: %w", err)
	}
	if err := os.WriteFile(evidenceAbs, []byte("approved\n"), 0o644); err != nil {
		return fmt.Errorf("write auth seam evidence ref: %w", err)
	}

	closureRel := "docs/closure/auth-seam.md"
	closureAbs := filepath.Join(root, filepath.FromSlash(filepath.ToSlash(closureRel)))
	if err := os.MkdirAll(filepath.Dir(closureAbs), 0o755); err != nil {
		return fmt.Errorf("create auth seam closure dir: %w", err)
	}
	closureContent := strings.TrimSpace(`---
shipments:
  - `+archived.ID+`
---

# Closure

### `+archived.ID+`
Merged as PR #101 (feature), commit `+shortSHA(evFixture.merge048)+`, and PR #102 (post-merge closure), commit `+shortSHA(evFixture.closure048)+`.
`) + "\n"
	if err := os.WriteFile(closureAbs, []byte(closureContent), 0o644); err != nil {
		return fmt.Errorf("write auth seam closure file: %w", err)
	}

	req := ShipmentShippedReconcileRequest{
		ShipmentID:      archived.ID,
		Reason:          "governed repair",
		Actor:           "operator",
		SecondApprover:  "auditor",
		IdempotencyKey:  "idem-167-001",
		MergeSHA:        evFixture.merge048,
		ClosureEvidence: closureRel,
		EvidenceRefs:    []string{evidenceRefRel},
	}
	digest, err := shipmentReconcileRequestIdentityDigest(req)
	if err != nil {
		return fmt.Errorf("compute auth seam request digest: %w", err)
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("generate auth seam keypair: %w", err)
	}
	anchors := []config.TrustAnchor{{
		ID: "op-1", Role: TokenIssuerRole, Algo: "ed25519", Fingerprint: "sha256:t1",
		Status: config.TrustAnchorStatusActive,
	}}
	key, err := ResolveTokenIssuerKey(anchors, "op-1", time.Now(), NewBoundExternalPinSet(PinBinding{Fingerprint: "sha256:t1", Role: TokenIssuerRole}))
	if err != nil {
		return fmt.Errorf("resolve auth seam issuer key: %w", err)
	}

	if _, err := ws.DB.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return fmt.Errorf("checkpoint auth seam seed: %w", err)
	}

	authSeamSeed.root = root
	authSeamSeed.shipmentID = archived.ID
	authSeamSeed.memberID = member.ID
	authSeamSeed.closureRel = closureRel
	authSeamSeed.evidenceRefRel = evidenceRefRel
	authSeamSeed.req = req
	authSeamSeed.digest = digest
	authSeamSeed.priv = priv
	authSeamSeed.key = key
	authSeamSeed.pub = &VerificationKey{Algo: "ed25519", Public: pub, Fingerprint: "sha256:t1"}
	return nil
}

func cloneAuthSeamKey(src ResolvedKey) ResolvedKey {
	cloned := src
	if src.Anchor.NotBefore != nil {
		nb := *src.Anchor.NotBefore
		cloned.Anchor.NotBefore = &nb
	}
	if src.Anchor.NotAfter != nil {
		na := *src.Anchor.NotAfter
		cloned.Anchor.NotAfter = &na
	}
	return cloned
}

func newAuthSeamFixture(t *testing.T) authSeamFixture {
	t.Helper()
	requireGit(t)
	ctx := context.Background()
	authSeamSeed.once.Do(func() {
		authSeamSeed.err = buildAuthSeamSeed(ctx)
	})
	require.NoError(t, authSeamSeed.err, "initialize auth seam seed")

	root := t.TempDir()
	require.NoError(t, copyFixtureGitSeed(authSeamSeed.root, root))

	ws, err := NewDiagnosticWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, ws.Close()) })
	disableExecGateForTest(t, ws)
	for range 4 {
		// Best-effort speedup for disposable test databases only.
		_, _ = ws.DB.ExecContext(ctx, "PRAGMA synchronous = OFF")
	}
	ws.Config.Reconcile = &config.ReconcileConfig{}

	projection, err := OpenNonceLedger(filepath.Join(t.TempDir(), "nonces.jsonl"))
	require.NoError(t, err)
	provider := &seamNonceProvider{}
	nonces, err := NewNonceConsumer(provider, projection, seamConsumeAuditor{})
	require.NoError(t, err)

	privCopy := append(ed25519.PrivateKey(nil), authSeamSeed.priv...)
	seedPub, ok := authSeamSeed.pub.Public.(ed25519.PublicKey)
	require.True(t, ok, "auth seam seed public key must be ed25519.PublicKey")
	pubCopy := append(ed25519.PublicKey(nil), seedPub...)

	reqCopy := authSeamSeed.req
	reqCopy.ClosureEvidence = strings.Clone(authSeamSeed.closureRel)
	reqCopy.EvidenceRefs = append([]string(nil), authSeamSeed.evidenceRefRel)

	return authSeamFixture{
		ws:         ws,
		shipmentID: strings.Clone(authSeamSeed.shipmentID),
		req:        reqCopy,
		digest:     strings.Clone(authSeamSeed.digest),
		priv:       privCopy,
		key:        cloneAuthSeamKey(authSeamSeed.key),
		pub: &VerificationKey{
			Algo:        authSeamSeed.pub.Algo,
			Public:      pubCopy,
			Fingerprint: authSeamSeed.pub.Fingerprint,
		},
		provider:   provider,
		rejections: &seamRejectionAuditor{},
		nonces:     nonces,
	}
}

func (f authSeamFixture) token(shipmentID, digest, nonce string) AuthorizationToken {
	now := time.Now()
	tok := AuthorizationToken{
		Header: AuthorizationTokenHeader{Algo: "ed25519", KeyID: "op-1"},
		Claims: AuthorizationTokenClaims{NotBefore: now.Add(-time.Minute), NotAfter: now.Add(5 * time.Minute), Nonce: nonce},
		Binding: AuthorizationTokenBinding{
			ShipmentID: shipmentID, RequestIdentityDigest: digest,
		},
	}
	tok.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(f.priv, tok.SigningPayload()))
	return tok
}

func (f authSeamFixture) authorized(tok AuthorizationToken) ShipmentShippedReconcileRequest {
	req := f.req
	req.AuthEnforcement = AuthEnforcementAuthenticated
	req.Authorization = &ReconcileAuthorization{
		Token: tok, Key: f.key, PublicKey: f.pub, Nonces: f.nonces, Auditor: f.rejections,
	}
	return req
}

func (f authSeamFixture) logText(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(events.LogPathForItem(WorkspaceLogsRoot(f.ws.RootPath), f.shipmentID))
	if os.IsNotExist(err) {
		return ""
	}
	require.NoError(t, err)
	return string(raw)
}

func (f authSeamFixture) archivedStatus(t *testing.T) string {
	t.Helper()
	art, err := findArtifact(context.Background(), f.ws, f.shipmentID)
	require.NoError(t, err)
	return art.ArchivedStatus
}

func TestReconcileAuthTokenSeam(t *testing.T) {
	ctx := context.Background()

	t.Run("authenticated: verified token is consumed and approver persists in the event", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		res, err := ReconcileShipmentToShipped(ctx, f.ws, f.authorized(f.token(f.shipmentID, f.digest, "00aa")))
		require.NoError(t, err)
		assert.Equal(t, ShipmentReconcileOutcomeReconciled, res.Outcome)
		assert.Equal(t, AuthorizationOutcomeConsumed, res.Authorization)
		assert.Equal(t, 1, f.provider.count(), "nonce consumed via the atomic owner")
		assert.Equal(t, string(ShipmentShipped), f.archivedStatus(t))
		log := f.logText(t)
		assert.Contains(t, log, `"approver":{"algo":"ed25519","key_id":"op-1","nonce":"00aa"}`)
	})

	t.Run("authenticated: missing token fails closed with no write", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		req := f.req
		req.AuthEnforcement = AuthEnforcementAuthenticated
		_, err := ReconcileShipmentToShipped(ctx, f.ws, req)
		require.ErrorIs(t, err, ErrAuthorizationRequired)
		assert.Equal(t, string(ShipmentActive), f.archivedStatus(t))
		assert.NotContains(t, f.logText(t), EventShipmentReconciledShipped)
		assert.Zero(t, f.provider.count())
	})

	t.Run("authenticated: rejected token fails closed with no write and no consume", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		tok := f.token(f.shipmentID, f.digest, "00bb")
		tok.Binding.RequestIdentityDigest = "tampered"
		_, err := ReconcileShipmentToShipped(ctx, f.ws, f.authorized(tok))
		require.ErrorIs(t, err, ErrAuthorizationRejected)
		assert.Equal(t, string(ShipmentActive), f.archivedStatus(t))
		assert.NotContains(t, f.logText(t), EventShipmentReconciledShipped)
		assert.Zero(t, f.provider.count())
		assert.NotEmpty(t, f.rejections.events, "rejection is audited")
	})

	t.Run("authenticated dry run verifies without consuming or writing", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		req := f.authorized(f.token(f.shipmentID, f.digest, "00cc"))
		req.DryRun = true
		res, err := ReconcileShipmentToShipped(ctx, f.ws, req)
		require.NoError(t, err)
		assert.Equal(t, AuthorizationOutcomeVerified, res.Authorization)
		assert.Zero(t, f.provider.count())
		assert.Equal(t, string(ShipmentActive), f.archivedStatus(t))
	})

	t.Run("unconfigured: v1 confirmation path needs no token and records no approver", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		res, err := ReconcileShipmentToShipped(ctx, f.ws, f.req)
		require.NoError(t, err)
		assert.Equal(t, ShipmentReconcileOutcomeReconciled, res.Outcome)
		assert.Equal(t, AuthorizationOutcomeNotRequired, res.Authorization)
		assert.Zero(t, f.provider.count())
		assert.NotContains(t, f.logText(t), `"approver"`)
	})

	t.Run("unknown enforcement mode is denied fail closed", func(t *testing.T) {
		f := newAuthSeamFixture(t)
		req := f.req
		req.AuthEnforcement = AuthEnforcement("bogus")
		_, err := ReconcileShipmentToShipped(ctx, f.ws, req)
		require.ErrorIs(t, err, ErrAuthPolicyDenied)
		assert.Equal(t, string(ShipmentActive), f.archivedStatus(t))
	})
}

func TestU6_AuthSeamFixtureUsesSharedSeedAndUR1BSubprocessIsBounded(t *testing.T) {
	t.Run("shared_auth_seam_seed_preserves_isolated_roots_and_shared_keypair", func(t *testing.T) {
		first := newAuthSeamFixture(t)
		second := newAuthSeamFixture(t)

		require.NotEqual(t, first.ws.RootPath, second.ws.RootPath)
		require.Equal(t, first.priv, second.priv)
		require.Equal(t, first.pub.Public, second.pub.Public)
	})

	t.Run("ur1b_child_subprocess_uses_command_context_and_wait_delay_ast", func(t *testing.T) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "shipment_blocked_lifecycle_harness_test.go", nil, 0)
		require.NoError(t, err)

		var targetFn *ast.FuncDecl
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.Name == "TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot" {
				targetFn = fn
				break
			}
		}
		require.NotNil(t, targetFn, "TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot declaration not found")
		require.NotNil(t, targetFn.Body)

		var unboundedCommands []string
		hasCommandContext := false
		hasWaitDelayAssign := false

		ast.Inspect(targetFn.Body, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				sel, ok := node.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, isIdent := sel.X.(*ast.Ident)
				if !isIdent || pkg.Name != "exec" {
					return true
				}
				switch sel.Sel.Name {
				case "Command":
					unboundedCommands = append(unboundedCommands, fset.Position(node.Pos()).String())
				case "CommandContext":
					hasCommandContext = true
				}
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == "WaitDelay" {
						hasWaitDelayAssign = true
					}
				}
			}
			return true
		})

		require.Empty(t, unboundedCommands, "TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot must not invoke unbounded exec.Command")
		require.True(t, hasCommandContext, "TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot must invoke exec.CommandContext")
		require.True(t, hasWaitDelayAssign, "TestUR1B_FailClosedValidationAndAuthoritativeActiveSlot must assign WaitDelay on *exec.Cmd")
	})
}
