package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestRubberDuck_F4_MissingShipmentCodeMatchesMCP(t *testing.T) {
	repo := u7NewCustodyRepo(t)
	call := u7VerifyCall("999-S")

	mcpOut := u7Invoke(t, u7ModeMCP, repo.root, call)
	cliOut := u7Invoke(t, u7ModeCLI, repo.root, call)
	require.Equal(t, "not_found", mcpOut.Code, "MCP verify on a missing shipment: %s", mcpOut.Message)
	assert.Equal(t, mcpOut.Code, cliOut.Code, "CLI and MCP must report the same code for a missing shipment")
}

func TestRubberDuck_F5_RevokedPinnedAnchorReportsRevokedOnBothSurfaces(t *testing.T) {
	repo := u7NewCustodyRepo(t)
	u7WriteFile(t, repo.root, u7OperatorFile, u7RevisedContent)
	operatorCandidates := u7JSON(t, []core.CustodyCandidateSpec{{
		Path: u7OperatorFile, ChangeKind: "modified", StagedState: "unstaged_only",
		Contributor: "operator", SourceKind: "operator_code_or_doc",
	}})
	preview := u7Invoke(t, u7ModeCLI, repo.root, u7PreviewCall(repo.operatorShip, operatorCandidates))
	require.Empty(t, preview.Code, "preview: %s", preview.Message)
	digest := u7Field(t, preview, "candidate_set_digest")

	adoptSpec := u7JSON(t, core.CustodyReceipt{
		ReceiptID: u7AdoptReceipt, Revision: 1, Kind: "preexisting_adoption",
		AuthorityProfile: "operator_verified", Status: "offered",
		ProducerRole: "operator", AcceptingRole: "operator", AllowedAction: "preexisting_adoption_commit",
		CandidateSetDigest: digest,
		Entries:            []core.CustodyEntry{u7Entry(u7OperatorFile, "operator", "operator_code_or_doc")},
	})
	offer := u7Invoke(t, u7ModeCLI, repo.root, u7OfferCall(repo.operatorShip, adoptSpec))
	require.Empty(t, offer.Code, "offer: %s", offer.Message)
	token := u7ApprovalToken(t, repo, digest)

	cfgPath := filepath.Join(repo.root, ".backlogit", "config.yaml")
	raw, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	revoked := strings.Replace(string(raw), "status: active", "status: revoked", 1)
	require.NotEqual(t, string(raw), revoked, "precondition: the pinned operator anchor must be active in config")
	require.NoError(t, os.WriteFile(cfgPath, []byte(revoked), 0o644))

	for _, mode := range []u7Mode{u7ModeMCP, u7ModeCLI} {
		out := u7Invoke(t, mode, repo.root, u7AcceptCall(repo.operatorShip, u7AdoptReceipt, "operator", token, false))
		assert.Equal(t, "trust_anchor_revoked", out.Code, "accept via %s must report the revoked anchor: %s", mode, out.Message)
	}
}
