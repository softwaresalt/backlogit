package core

import (
	"context"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// PreviewCustodyCandidates reports custody candidates for ws without writing any state.
func PreviewCustodyCandidates(ctx context.Context, ws *Workspace, opts CustodyPreviewOptions) (CustodyPreviewResult, error) {
	return CustodyPreviewResult{}, blerrors.ErrNotImplemented
}

// OfferCustodyHandoff records a custody handoff offer for a shipment.
func OfferCustodyHandoff(ctx context.Context, ws *Workspace, opts CustodyOfferOptions) (CustodyMutationResult, error) {
	return CustodyMutationResult{}, blerrors.ErrNotImplemented
}

// AcceptCustodyHandoff records acceptance of a previously offered custody handoff.
func AcceptCustodyHandoff(ctx context.Context, ws *Workspace, opts CustodyAcceptOptions) (CustodyMutationResult, error) {
	return CustodyMutationResult{}, blerrors.ErrNotImplemented
}

// VerifyCustodyState checks recorded custody receipts against the current workspace.
func VerifyCustodyState(ctx context.Context, ws *Workspace, opts CustodyVerifyOptions) (CustodyVerifyResult, error) {
	return CustodyVerifyResult{}, blerrors.ErrNotImplemented
}
