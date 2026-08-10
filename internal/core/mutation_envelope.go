package core

import (
	"context"
	"errors"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// MutationStep describes one ordered mutation and its compensation.
//
// Name must be a stable human-readable identifier used in MutationPartialError
// so callers can identify the step without parsing error text. Apply performs
// the mutation; Compensate reverses it on rollback. A nil Apply succeeds as a
// no-op. A nil Compensate is a documented no-op (suitable for append-only
// steps such as JSONL event appends that must never be reversed).
type MutationStep struct {
	Name       string
	Apply      func(context.Context) error
	Compensate func(context.Context) error
}

// MutationEnvelope executes ordered mutation steps and classifies partial
// failures into machine-readable outcomes.
//
// On failure the envelope branches on classification:
//
//   - ErrWriteNotApplied or a plain error → compensate applied steps in
//     reverse order; return *blerrors.MutationPartialError with
//     Class="not-applied" (or "double-fault" if compensation also fails).
//   - ErrWriteIndeterminate → do NOT compensate; continue remaining steps so
//     stores converge (commit-then-surface); collect successful continuation
//     step names in MutationPartialError.ContinuationApplied; return
//     *blerrors.MutationPartialError with Class="indeterminate".
//
// Named invariant (classification precedence): if any step returns
// ErrWriteIndeterminate, Class is "indeterminate" regardless of other joined
// errors. No compensation is ever run in the indeterminate path.
//
// The envelope takes no domain types. Callers capture whatever they need
// inside their Apply and Compensate closures.
func MutationEnvelope(ctx context.Context, steps []MutationStep) error {
	appliedIndexes := make([]int, 0, len(steps))
	appliedNames := make([]string, 0, len(steps))

	for i, step := range steps {
		var stepErr error
		if step.Apply != nil {
			stepErr = step.Apply(ctx)
		}
		if stepErr == nil {
			appliedIndexes = append(appliedIndexes, i)
			appliedNames = append(appliedNames, step.Name)
			continue
		}

		if blerrors.IsWriteIndeterminate(stepErr) {
			// Never compensate on indeterminate. Run remaining steps to converge
			// stores (commit-then-surface), tracking which continuation steps
			// succeed so MutationPartialError.ContinuationApplied is accurate.
			allErrs := []error{stepErr}
			var continuationApplied []string
			for j := i + 1; j < len(steps); j++ {
				if steps[j].Apply == nil {
					continue
				}
				if contErr := steps[j].Apply(ctx); contErr != nil {
					allErrs = append(allErrs, contErr)
				} else {
					continuationApplied = append(continuationApplied, steps[j].Name)
				}
			}
			return &blerrors.MutationPartialError{
				Completed:           appliedNames,
				ContinuationApplied: continuationApplied,
				FailedStep:          step.Name,
				CompensationState:   "not-compensated",
				Class:               "indeterminate",
				Cause:               errors.Join(allErrs...),
			}
		}

		// Not indeterminate: compensate applied steps in reverse order.
		// Detach from the original (possibly canceled) context so a
		// request cancellation or deadline expiry does not immediately
		// fail the SQL and artifact rollback calls, converting a
		// recoverable cancellation into a spurious double-fault.
		compensateCtx := context.WithoutCancel(ctx)
		var compensationErr error
		for j := len(appliedIndexes) - 1; j >= 0; j-- {
			appliedStep := steps[appliedIndexes[j]]
			if appliedStep.Compensate == nil {
				continue
			}
			if err := appliedStep.Compensate(compensateCtx); err != nil {
				compensationErr = errors.Join(compensationErr, err)
			}
		}
		if compensationErr != nil {
			return &blerrors.MutationPartialError{
				Completed:         appliedNames,
				FailedStep:        step.Name,
				CompensationState: "unknown",
				Class:             "double-fault",
				Cause:             errors.Join(stepErr, compensationErr),
			}
		}
		return &blerrors.MutationPartialError{
			Completed:         appliedNames,
			FailedStep:        step.Name,
			CompensationState: "compensated",
			Class:             "not-applied",
			Cause:             stepErr,
		}
	}

	return nil
}

