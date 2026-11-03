package core

import (
	"errors"
	"time"

	"github.com/softwaresalt/backlogit/internal/config"
)

// WorkflowCustodySchemaVersion identifies the workflow-custody payload schema.
const WorkflowCustodySchemaVersion = "workflow-custody/v1"

// CustodyRequestIdentityDomain domain-separates custody request identity digests.
const CustodyRequestIdentityDomain = "backlogit/workflow-custody/request-identity/v1"

// DefaultCustodyReceiptExpiry is the default lifetime of an accepted custody receipt.
const DefaultCustodyReceiptExpiry = 168 * time.Hour

// DefaultCustodyOfferExpiry is the default lifetime of an offered custody receipt
// that has not yet been accepted.
const DefaultCustodyOfferExpiry = 168 * time.Hour

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
	ErrCustodyReceiptExpired         = errors.New("custody: receipt expired or has no expiry")
	ErrCustodyIdempotencyConflict    = errors.New("custody: idempotency key reused with a different request")
	ErrCustodyStaleRevision          = errors.New("custody: receipt revision is not the next custody revision")
)

// WorkflowCustodyEnvelope is the workflow-custody/v1 payload stored on a shipment
// under custom_fields.workflow_custody.
type WorkflowCustodyEnvelope struct {
	// SchemaVersion must equal WorkflowCustodySchemaVersion.
	SchemaVersion string `json:"schema_version" yaml:"schema_version"`
	// WorkflowID is the stable identifier scoping the workflow across sessions and branches.
	WorkflowID string `json:"workflow_id" yaml:"workflow_id"`
	// ShipmentID is the shipment the envelope is bound to.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// ScopeBranch is the non-default work branch that custody mutations are scoped to.
	ScopeBranch string `json:"scope_branch" yaml:"scope_branch"`
	// BaseCommitSHA is the commit that custody change sets are measured against.
	BaseCommitSHA string `json:"base_commit_sha" yaml:"base_commit_sha"`
	// CustodyRevision is the revision of the envelope, incremented on each accepted receipt.
	CustodyRevision int `json:"custody_revision" yaml:"custody_revision"`
	// ActiveReceiptID names the receipt currently in force, when one exists.
	ActiveReceiptID string `json:"active_receipt_id,omitempty" yaml:"active_receipt_id,omitempty"`
	// ActiveReceiptDigest is the digest of the active receipt.
	ActiveReceiptDigest string `json:"active_receipt_digest,omitempty" yaml:"active_receipt_digest,omitempty"`
	// Receipts lists the custody receipts recorded for the shipment.
	Receipts []CustodyReceipt `json:"receipts" yaml:"receipts"`
}

// CustodyReceipt records one custody offer or accepted custody handoff.
type CustodyReceipt struct {
	// ReceiptID uniquely identifies the receipt within the shipment.
	ReceiptID string `json:"receipt_id" yaml:"receipt_id"`
	// Revision is the custody revision this receipt was recorded at.
	Revision int `json:"revision" yaml:"revision"`
	// SupersedesReceiptID names the receipt this one replaces, when it supersedes another.
	SupersedesReceiptID string `json:"supersedes_receipt_id,omitempty" yaml:"supersedes_receipt_id,omitempty"`
	// Kind is the custody scenario: preexisting_adoption, stage_to_ship_handoff, or ship_task_custody.
	Kind string `json:"kind" yaml:"kind"`
	// AuthorityProfile is operator_verified (Profile A) or agent_workflow_handoff (Profile B).
	AuthorityProfile string `json:"authority_profile" yaml:"authority_profile"`
	// Status is offered, accepted, rejected, superseded, or invalidated.
	Status string `json:"status" yaml:"status"`
	// ProducerRole is the role that produced the covered change set.
	ProducerRole string `json:"producer_role" yaml:"producer_role"`
	// AcceptingRole is the role that accepts custody of the covered change set.
	AcceptingRole string `json:"accepting_role" yaml:"accepting_role"`
	// AllowedAction is the single mutation the receipt authorizes.
	AllowedAction string `json:"allowed_action" yaml:"allowed_action"`
	// DecisionRef is a context link to the decision that produced the receipt. It is never authority by itself.
	DecisionRef string `json:"decision_ref,omitempty" yaml:"decision_ref,omitempty"`
	// CandidateSetDigest is the digest over the exact candidate change set.
	CandidateSetDigest string `json:"candidate_set_digest" yaml:"candidate_set_digest"`
	// ApprovalProof is the operator approval token required for operator_verified receipts.
	ApprovalProof *CustodyApprovalProof `json:"approval_proof,omitempty" yaml:"approval_proof,omitempty"`
	// IdempotencyKey deduplicates retried offer and accept requests.
	IdempotencyKey string `json:"idempotency_key,omitempty" yaml:"idempotency_key,omitempty"`
	// IdempotencyRequestDigest pins the accept request that recorded IdempotencyKey, so a retry is replayed only for an identical request.
	IdempotencyRequestDigest string `json:"idempotency_request_digest,omitempty" yaml:"idempotency_request_digest,omitempty"`
	// OfferedAtUTC is the UTC instant the receipt was offered.
	OfferedAtUTC time.Time `json:"offered_at_utc" yaml:"offered_at_utc"`
	// AcceptedAtUTC is the UTC instant the receipt was accepted.
	AcceptedAtUTC time.Time `json:"accepted_at_utc" yaml:"accepted_at_utc"`
	// ExpiresAtUTC is the UTC instant after which the receipt is no longer valid.
	ExpiresAtUTC time.Time `json:"expires_at_utc" yaml:"expires_at_utc"`
	// Entries lists the covered paths.
	Entries []CustodyEntry `json:"entries" yaml:"entries"`
}

// CustodyEntry describes one covered path within a custody receipt.
type CustodyEntry struct {
	// Path is the repository-relative POSIX path covered by the receipt.
	Path string `json:"path" yaml:"path"`
	// OldPath is the pre-rename path. It is set only when ChangeKind is renamed.
	OldPath string `json:"old_path,omitempty" yaml:"old_path,omitempty"`
	// ChangeKind is added, modified, deleted, renamed, or staged_and_unstaged.
	ChangeKind string `json:"change_kind" yaml:"change_kind"`
	// CaseOnlyRename is true when OldPath and Path differ only by letter case.
	CaseOnlyRename bool `json:"case_only_rename,omitempty" yaml:"case_only_rename,omitempty"`
	// StagedState describes the Git index relationship of the covered content.
	StagedState string `json:"staged_state" yaml:"staged_state"`
	// OldBaseBlobSHA256 pins the pre-image of OldPath at BaseCommitSHA for renamed entries.
	OldBaseBlobSHA256 string `json:"old_base_blob_sha256,omitempty" yaml:"old_base_blob_sha256,omitempty"`
	// BaseBlobSHA256 pins Path at BaseCommitSHA. It is empty when Path did not exist at the base.
	BaseBlobSHA256 string `json:"base_blob_sha256" yaml:"base_blob_sha256"`
	// IndexSHA256 pins the Git index (stage 0) content of Path.
	IndexSHA256 string `json:"index_sha256" yaml:"index_sha256"`
	// WorktreeSHA256 pins the LF-normalized worktree content of Path.
	WorktreeSHA256 string `json:"worktree_sha256" yaml:"worktree_sha256"`
	// PatchDigestSHA256 pins the digest of the patch the receipt adopts.
	PatchDigestSHA256 string `json:"patch_digest_sha256" yaml:"patch_digest_sha256"`
	// Contributor identifies who produced the change: operator, stage, ship, or mixed_reconciled.
	Contributor string `json:"contributor" yaml:"contributor"`
	// SourceKind classifies where the covered content came from.
	SourceKind string `json:"source_kind" yaml:"source_kind"`
	// RenameGroupID links the deleted and added halves of a rename that is recorded as two entries.
	RenameGroupID string `json:"rename_group_id,omitempty" yaml:"rename_group_id,omitempty"`
}

// CustodyApprovalProof carries the signed operator approval claims for a custody mutation.
type CustodyApprovalProof struct {
	// KeyID names the trust anchor whose key signed the proof.
	KeyID string `json:"key_id" yaml:"key_id"`
	// Algorithm is the signature algorithm, currently ed25519.
	Algorithm string `json:"algorithm" yaml:"algorithm"`
	// Principal identifies the human or host principal that approved the change.
	Principal string `json:"principal" yaml:"principal"`
	// AuthorizedRole must equal operator for the proof to be accepted.
	AuthorizedRole string `json:"authorized_role" yaml:"authorized_role"`
	// WorkflowID binds the proof to a workflow.
	WorkflowID string `json:"workflow_id" yaml:"workflow_id"`
	// ShipmentID binds the proof to a shipment.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// ScopeBranch binds the proof to a work branch.
	ScopeBranch string `json:"scope_branch" yaml:"scope_branch"`
	// BaseCommitSHA binds the proof to the base commit.
	BaseCommitSHA string `json:"base_commit_sha" yaml:"base_commit_sha"`
	// CandidateSetDigest binds the proof to the exact approved candidate set.
	CandidateSetDigest string `json:"candidate_set_digest" yaml:"candidate_set_digest"`
	// AllowedAction is the single mutation the proof authorizes.
	AllowedAction string `json:"allowed_action" yaml:"allowed_action"`
	// TargetRevision is the custody revision the proof authorizes.
	TargetRevision int `json:"target_revision" yaml:"target_revision"`
	// NotBeforeUTC is the earliest UTC instant the proof is valid.
	NotBeforeUTC time.Time `json:"not_before_utc" yaml:"not_before_utc"`
	// NotAfterUTC is the latest UTC instant the proof is valid.
	NotAfterUTC time.Time `json:"not_after_utc" yaml:"not_after_utc"`
	// Nonce is the single-use value consumed when the proof is accepted.
	Nonce string `json:"nonce" yaml:"nonce"`
	// RequestIdentityDigest is the domain-separated digest of the approved request.
	RequestIdentityDigest string `json:"request_identity_digest" yaml:"request_identity_digest"`
	// ReceiptID binds the proof to one receipt. Empty leaves the proof unbound to a receipt.
	ReceiptID string `json:"receipt_id,omitempty" yaml:"receipt_id,omitempty"`
	// AcceptingRole binds the proof to the accepting role. Empty leaves the role unbound.
	AcceptingRole string `json:"accepting_role,omitempty" yaml:"accepting_role,omitempty"`
	// Channel names the operator approval channel that issued the proof.
	Channel string `json:"channel,omitempty" yaml:"channel,omitempty"`
	// Signature is the base64url Ed25519 signature over the canonical proof bytes.
	Signature string `json:"signature,omitempty" yaml:"signature,omitempty"`
}

// CustodyCandidateSpec describes one candidate path considered for custody.
type CustodyCandidateSpec struct {
	// Path is the repository-relative POSIX path of the candidate.
	Path string `json:"path" yaml:"path"`
	// OldPath is the pre-rename path when the candidate is a rename.
	OldPath string `json:"old_path,omitempty" yaml:"old_path,omitempty"`
	// ChangeKind is the candidate change kind.
	ChangeKind string `json:"change_kind" yaml:"change_kind"`
	// StagedState describes the Git index relationship of the candidate.
	StagedState string `json:"staged_state,omitempty" yaml:"staged_state,omitempty"`
	// Contributor identifies who produced the candidate change.
	Contributor string `json:"contributor,omitempty" yaml:"contributor,omitempty"`
	// SourceKind classifies where the candidate content came from.
	SourceKind string `json:"source_kind,omitempty" yaml:"source_kind,omitempty"`
	// RenameGroupID links the halves of a two-entry rename.
	RenameGroupID string `json:"rename_group_id,omitempty" yaml:"rename_group_id,omitempty"`
}

// CustodyPreviewOptions configures a zero-write custody preview.
type CustodyPreviewOptions struct {
	// ShipmentID selects the shipment whose custody is previewed.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// Branch is the work branch the preview is evaluated against.
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
	// Cwd is the working directory used to resolve a relative workspace root.
	Cwd string `json:"cwd,omitempty" yaml:"cwd,omitempty"`
	// Candidates lists the candidate paths to classify.
	Candidates []CustodyCandidateSpec `json:"candidates" yaml:"candidates"`
}

// CustodyPreviewResult reports the outcome of a custody preview.
type CustodyPreviewResult struct {
	// ShipmentID is the previewed shipment.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// Branch is the work branch the preview was evaluated against.
	Branch string `json:"branch" yaml:"branch"`
	// CandidateSetDigest is the digest over the classified candidate set.
	CandidateSetDigest string `json:"candidate_set_digest" yaml:"candidate_set_digest"`
	// RequiredAuthorityProfile is operator_verified or agent_workflow_handoff for the candidate set.
	RequiredAuthorityProfile string `json:"required_authority_profile" yaml:"required_authority_profile"`
	// Candidates lists the classified candidates.
	Candidates []CustodyCandidateSpec `json:"candidates" yaml:"candidates"`
	// Warnings lists non-fatal findings.
	Warnings []string `json:"warnings" yaml:"warnings"`
}

// CustodyOfferOptions configures a custody handoff offer.
type CustodyOfferOptions struct {
	// ShipmentID selects the shipment receiving the offer.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// Spec is the offered receipt.
	Spec CustodyReceipt `json:"spec" yaml:"spec"`
	// DryRun validates the offer without writing any state.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
}

// CustodyAcceptOptions configures a custody handoff acceptance.
type CustodyAcceptOptions struct {
	// ShipmentID selects the shipment whose offer is accepted.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// ReceiptID selects the offered receipt to accept.
	ReceiptID string `json:"receipt_id" yaml:"receipt_id"`
	// By identifies the accepting principal.
	By string `json:"by" yaml:"by"`
	// IdempotencyKey deduplicates retried accept requests.
	IdempotencyKey string `json:"idempotency_key,omitempty" yaml:"idempotency_key,omitempty"`
	// AuthToken is the encoded CustodyApprovalToken required for operator_verified receipts.
	AuthToken string `json:"auth_token,omitempty" yaml:"auth_token,omitempty"`
	// TrustAnchors, when non-empty, is the caller-injected operator trust set. It is authoritative and replaces workspace config anchors. It is never read from JSON or YAML input.
	TrustAnchors []config.TrustAnchor `json:"-" yaml:"-"`
	// ExternalPins, when non-nil, is the externally provisioned pin set that TrustAnchors must satisfy.
	ExternalPins *ExternalPinSet `json:"-" yaml:"-"`
	// DryRun verifies the acceptance without consuming a nonce or writing state.
	DryRun bool `json:"dry_run,omitempty" yaml:"dry_run,omitempty"`
}

// CustodyMutationResult reports the outcome of a custody offer or acceptance.
type CustodyMutationResult struct {
	// ShipmentID is the affected shipment.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// ReceiptID is the affected receipt.
	ReceiptID string `json:"receipt_id" yaml:"receipt_id"`
	// Status is the receipt status after the operation.
	Status string `json:"status" yaml:"status"`
	// AuthorityProfile is the authority profile the operation was verified under.
	AuthorityProfile string `json:"authority_profile" yaml:"authority_profile"`
	// CustodyRevision is the envelope revision after the operation.
	CustodyRevision int `json:"custody_revision" yaml:"custody_revision"`
	// DryRun reports whether the operation was a zero-write validation.
	DryRun bool `json:"dry_run" yaml:"dry_run"`
	// Verdicts lists the per-path outcomes.
	Verdicts []CustodyPathVerdict `json:"verdicts" yaml:"verdicts"`
	// Warnings lists non-fatal findings.
	Warnings []string `json:"warnings" yaml:"warnings"`
}

// CustodyVerifyOptions configures a custody state verification.
type CustodyVerifyOptions struct {
	// ShipmentID selects the shipment to verify.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// Branch is the work branch the verification is evaluated against.
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
	// Cwd is the working directory used to resolve a relative workspace root.
	Cwd string `json:"cwd,omitempty" yaml:"cwd,omitempty"`
	// TrustAnchors, when non-empty, is the caller-injected operator trust set. It is authoritative and replaces workspace config anchors. It is never read from JSON or YAML input.
	TrustAnchors []config.TrustAnchor `json:"-" yaml:"-"`
	// ExternalPins, when non-nil, is the externally provisioned pin set that TrustAnchors must satisfy.
	ExternalPins *ExternalPinSet `json:"-" yaml:"-"`
}

// CustodyPathVerdict reports the verification outcome for one custody path.
type CustodyPathVerdict struct {
	// Path is the repository-relative POSIX path that was verified.
	Path string `json:"path" yaml:"path"`
	// OldPath is the pre-rename path when the verdict covers a rename.
	OldPath string `json:"old_path,omitempty" yaml:"old_path,omitempty"`
	// ReceiptID is the receipt that covers the path, when one does.
	ReceiptID string `json:"receipt_id,omitempty" yaml:"receipt_id,omitempty"`
	// Verdict is the outcome code, such as ADOPTED_OPERATOR_VERIFIED or STALE_DIGEST.
	Verdict string `json:"verdict" yaml:"verdict"`
	// Reason is a short human-readable explanation of the verdict.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// CustodyVerifyResult reports the outcome of a custody state verification.
type CustodyVerifyResult struct {
	// ShipmentID is the verified shipment.
	ShipmentID string `json:"shipment_id" yaml:"shipment_id"`
	// Branch is the work branch the verification was evaluated against.
	Branch string `json:"branch" yaml:"branch"`
	// ActiveReceiptID is the receipt in force, when one exists.
	ActiveReceiptID string `json:"active_receipt_id,omitempty" yaml:"active_receipt_id,omitempty"`
	// Verdicts lists the per-path outcomes.
	Verdicts []CustodyPathVerdict `json:"verdicts" yaml:"verdicts"`
	// Warnings lists non-fatal findings.
	Warnings []string `json:"warnings" yaml:"warnings"`
}
