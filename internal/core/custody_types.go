package core

import (
	"errors"
	"time"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// WorkflowCustodySchemaVersion identifies the workflow-custody payload schema.
const WorkflowCustodySchemaVersion = "workflow-custody/v1"

// CustodyRequestIdentityDomain domain-separates custody request identity digests.
const CustodyRequestIdentityDomain = "backlogit/workflow-custody/request-identity/v1"

// DefaultCustodyReceiptExpiry is the default lifetime of an accepted custody receipt.
const DefaultCustodyReceiptExpiry = 168 * time.Hour

// MaxCustodyApprovalTokenWindow bounds the validity window of a custody approval token.
const MaxCustodyApprovalTokenWindow = 15 * time.Minute

// Custody event types appended to shipment item logs.
const (
	EventWorkflowCustodyBound      = "workflow_custody_bound"
	EventCustodyHandoffOffered     = "custody_handoff_offered"
	EventCustodyHandoffAccepted    = "custody_handoff_accepted"
	EventCustodyAdoptionRecorded   = "custody_adoption_recorded"
	EventCustodyReceiptSuperseded  = "custody_receipt_superseded"
	EventCustodyReceiptInvalidated = "custody_receipt_invalidated"
)

// Sentinel errors for workflow-custody refusals.
var (
	ErrCustodyDefaultBranchForbidden = errors.New("custody: default branch cannot be a custody scope")
	ErrPartialRenameAdoptionRejected = errors.New("custody: partial rename selection rejected")
	ErrUnsupportedCustodyChangeKind  = errors.New("custody: unsupported change kind")
	ErrOperatorApprovalRequired      = errors.New("custody: operator approval required")
	ErrApprovalChannelUnavailable    = errors.New("custody: approval channel unavailable")
	ErrMixedIndexWorktreeDivergence  = errors.New("custody: index and worktree diverge")
	ErrCustodyStaleDigest            = errors.New("custody: stale digest")
	ErrCustodyWrongBranch            = errors.New("custody: wrong branch")
)

// WorkflowCustodyEnvelope is the workflow-custody/v1 payload stored on a shipment.
type WorkflowCustodyEnvelope struct{}

// CustodyReceipt records an accepted custody handoff.
type CustodyReceipt struct{}

// CustodyEntry describes one covered path within a custody receipt.
type CustodyEntry struct{}

// CustodyApprovalProof carries the operator approval claims for a custody mutation.
type CustodyApprovalProof struct{}

// CustodyCandidateSpec describes one candidate path considered for custody.
type CustodyCandidateSpec struct{}

// CustodyPreviewOptions configures a zero-write custody preview.
type CustodyPreviewOptions struct{}

// CustodyPreviewResult reports the outcome of a custody preview.
type CustodyPreviewResult struct{}

// CustodyOfferOptions configures a custody handoff offer.
type CustodyOfferOptions struct{}

// CustodyAcceptOptions configures a custody handoff acceptance.
type CustodyAcceptOptions struct{}

// CustodyMutationResult reports the outcome of a custody offer or acceptance.
type CustodyMutationResult struct{}

// CustodyVerifyOptions configures a custody state verification.
type CustodyVerifyOptions struct{}

// CustodyPathVerdict reports the verification outcome for one custody path.
type CustodyPathVerdict struct{}

// CustodyVerifyResult reports the outcome of a custody state verification.
type CustodyVerifyResult struct{}

// ValidateCustodyEnvelope validates a workflow-custody/v1 envelope.
func ValidateCustodyEnvelope(env WorkflowCustodyEnvelope) error {
	return blerrors.ErrNotImplemented
}

// ValidateCustodyEntryPath validates that entryPath is a safe path under ws.
func ValidateCustodyEntryPath(ws *Workspace, entryPath string) error {
	return blerrors.ErrNotImplemented
}

// CustodyRequestIdentityDigest returns the domain-separated digest identifying a custody request.
func CustodyRequestIdentityDigest(env WorkflowCustodyEnvelope) (string, error) {
	return "", blerrors.ErrNotImplemented
}

// MintCustodyApprovalToken issues a signed approval token for proof.
func MintCustodyApprovalToken(proof CustodyApprovalProof) (string, error) {
	return "", blerrors.ErrNotImplemented
}

// VerifyCustodyApprovalToken verifies token against env at time now and returns its proof.
func VerifyCustodyApprovalToken(token string, env WorkflowCustodyEnvelope, now time.Time) (CustodyApprovalProof, error) {
	return CustodyApprovalProof{}, blerrors.ErrNotImplemented
}
