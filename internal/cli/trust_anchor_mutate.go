package cli

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// Environment variables carrying the operator approver public key. The key is
// held by the operator outside the workspace; approval tokens are signed with
// the matching private key and verified by the core.
const (
	// envTrustApproverKey is a base64 (std) raw Ed25519 public key.
	envTrustApproverKey = "BACKLOGIT_TRUST_APPROVER_KEY"
	// envTrustApproverKeyFile names a file containing that base64 key.
	envTrustApproverKeyFile = "BACKLOGIT_TRUST_APPROVER_KEY_FILE"
)

// trustAnchorFlags are the new-anchor declaration flags for add and rotate.
type trustAnchorFlags struct {
	id, role, algo, publicKeyRef, fingerprint string
	notBefore, notAfter                       string
	approvalFile                              string
}

func (f *trustAnchorFlags) bindAnchor(cmd *cobra.Command) {
	fl := cmd.Flags()
	fl.StringVar(&f.id, "id", "", "id of the new trust anchor")
	fl.StringVar(&f.role, "role", "", "role of the new trust anchor")
	fl.StringVar(&f.algo, "algo", "", "signature algorithm (ed25519 or ecdsa-p256)")
	fl.StringVar(&f.publicKeyRef, "public-key-ref", "", "inline PEM or workspace-relative path to the public key")
	fl.StringVar(&f.fingerprint, "fingerprint", "", "externally pinned fingerprint of the key")
	fl.StringVar(&f.notBefore, "not-before", "", "optional RFC3339 start of validity")
	fl.StringVar(&f.notAfter, "not-after", "", "optional RFC3339 end of validity")
}

func (f *trustAnchorFlags) bindApproval(cmd *cobra.Command) {
	cmd.Flags().StringVar(&f.approvalFile, "approval", "", "operator approval token file (signed JSON evidence); required for rotate and revoke")
}

func (f *trustAnchorFlags) anchor() (config.TrustAnchor, error) {
	a := config.TrustAnchor{ID: f.id, Role: f.role, Algo: f.algo, PublicKeyRef: f.publicKeyRef, Fingerprint: f.fingerprint}
	var err error
	if a.NotBefore, err = parseOptionalTime("not-before", f.notBefore); err != nil {
		return a, err
	}
	if a.NotAfter, err = parseOptionalTime("not-after", f.notAfter); err != nil {
		return a, err
	}
	return a, nil
}

func parseOptionalTime(name, v string) (*time.Time, error) {
	if v == "" {
		return nil, nil //nolint:nilnil // an absent optional bound is represented by nil
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return nil, fmt.Errorf("--%s must be RFC3339: %w", name, err)
	}
	return &t, nil
}

// addTrustAnchorMutationCommands registers add, rotate and revoke. They only
// parse input and delegate to core.TrustAnchorMutator; pin-gate, approval
// verification and audit all live in the core.
func addTrustAnchorMutationCommands(parent *cobra.Command, cwd *string) {
	var addF trustAnchorFlags
	add := &cobra.Command{
		Use:   "add",
		Short: "Add a trust anchor (fingerprint must be externally pinned)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			anchor, err := addF.anchor()
			if err != nil {
				return err
			}
			return runTrustAnchorMutation(cmd, *cwd, core.TrustAnchorMutation{Action: core.TrustAnchorActionAdd, Anchor: anchor}, "")
		},
	}
	addF.bindAnchor(add)

	var rotF trustAnchorFlags
	rotate := &cobra.Command{
		Use:   "rotate <id>",
		Short: "Rotate a trust anchor to a new key (requires operator approval)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			anchor, err := rotF.anchor()
			if err != nil {
				return err
			}
			return runTrustAnchorMutation(cmd, *cwd,
				core.TrustAnchorMutation{Action: core.TrustAnchorActionRotate, AnchorID: args[0], Anchor: anchor}, rotF.approvalFile)
		},
	}
	rotF.bindAnchor(rotate)
	rotF.bindApproval(rotate)

	var revF trustAnchorFlags
	revoke := &cobra.Command{
		Use:   "revoke <id>",
		Short: "Revoke a trust anchor (requires operator approval)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrustAnchorMutation(cmd, *cwd,
				core.TrustAnchorMutation{Action: core.TrustAnchorActionRevoke, AnchorID: args[0]}, revF.approvalFile)
		},
	}
	revF.bindApproval(revoke)
	parent.AddCommand(add, rotate, revoke)
}

// runTrustAnchorMutation obtains operator approval evidence for destructive
// actions and delegates to the core. A destructive action without approval is
// refused before the core is constructed or invoked.
func runTrustAnchorMutation(cmd *cobra.Command, cwd string, req core.TrustAnchorMutation, approvalFile string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	var approver core.ApproverVerifier
	if req.Action != core.TrustAnchorActionAdd {
		if approvalFile == "" {
			return fmt.Errorf("trust anchor %s %q: %w: pass --approval <token file>", req.Action, req.AnchorID, core.ErrApprovalRequired)
		}
		ev, err := readApprovalEvidence(approvalFile)
		if err != nil {
			return fmt.Errorf("trust anchor %s %q: %w", req.Action, req.AnchorID, err)
		}
		key, err := loadApproverKey()
		if err != nil {
			return fmt.Errorf("trust anchor %s %q: %w", req.Action, req.AnchorID, err)
		}
		req.Approval = &ev
		approver = core.NewEd25519ApproverVerifier(key)
	}
	storageRoot, err := core.ResolveStorageRoot(cwd)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	m := core.NewFileTrustAnchorMutator(storageRoot, envPinProvider{}, approver)
	out, err := m.Apply(ctx, req)
	if err != nil {
		return err
	}
	_, werr := fmt.Fprintf(cmd.OutOrStdout(), "trust anchor %s applied: id=%s config_written=%t status_written=%t audit_persisted=%t\n",
		req.Action, mutationTargetID(req), out.ConfigWritten, out.StatusWritten, out.AuditPersisted)
	return werr
}

func mutationTargetID(req core.TrustAnchorMutation) string {
	if req.Action == core.TrustAnchorActionAdd {
		return req.Anchor.ID
	}
	return req.AnchorID
}

// approvalFileDoc is the on-disk shape of an operator approval token.
type approvalFileDoc struct {
	Action    string `json:"action"`
	AnchorID  string `json:"anchor_id"`
	Approver  string `json:"approver"`
	Expiry    string `json:"expiry"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

// readApprovalEvidence parses a signed approval token. It only decodes; the
// core verifies signature, binding and expiry.
func readApprovalEvidence(path string) (core.ApprovalEvidence, error) {
	data, err := os.ReadFile(path) //nolint:gosec // operator-supplied approval token path
	if err != nil {
		return core.ApprovalEvidence{}, fmt.Errorf("read approval token: %w", err)
	}
	var doc approvalFileDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return core.ApprovalEvidence{}, fmt.Errorf("%w: parse approval token: %w", core.ErrApprovalInvalid, err)
	}
	expiry, err := time.Parse(time.RFC3339Nano, doc.Expiry)
	if err != nil {
		return core.ApprovalEvidence{}, fmt.Errorf("%w: approval expiry: %w", core.ErrApprovalInvalid, err)
	}
	sig, err := base64.StdEncoding.DecodeString(doc.Signature)
	if err != nil {
		return core.ApprovalEvidence{}, fmt.Errorf("%w: approval signature: %w", core.ErrApprovalInvalid, err)
	}
	return core.ApprovalEvidence{Action: core.TrustAnchorAction(doc.Action), AnchorID: doc.AnchorID,
		Approver: doc.Approver, Expiry: expiry, Nonce: doc.Nonce, Signature: sig}, nil
}

// loadApproverKey reads the operator approver public key from the environment.
func loadApproverKey() (ed25519.PublicKey, error) {
	raw := strings.TrimSpace(os.Getenv(envTrustApproverKey))
	if raw == "" {
		if p := os.Getenv(envTrustApproverKeyFile); p != "" {
			data, err := os.ReadFile(p) //nolint:gosec // operator-supplied approver key path
			if err != nil {
				return nil, fmt.Errorf("%w: read %s: %w", core.ErrApprovalInvalid, envTrustApproverKeyFile, err)
			}
			raw = strings.TrimSpace(string(data))
		}
	}
	if raw == "" {
		return nil, fmt.Errorf("%w: operator approver key not provisioned (%s)", core.ErrApprovalInvalid, envTrustApproverKey)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return nil, errors.Join(core.ErrApprovalInvalid, errors.New("operator approver key must be a base64 Ed25519 public key"))
	}
	return ed25519.PublicKey(key), nil
}
