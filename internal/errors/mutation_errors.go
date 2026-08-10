package errors

import (
	"errors"
	"fmt"
	"strings"
)

// MutationPartialError reports a multi-step mutation that failed after the
// envelope had enough information to classify the outcome.
//
// For an indeterminate failure, ContinuationApplied lists any steps that
// succeeded during the post-failure continuation pass (commit-then-surface).
// These steps applied AFTER the classifying failure and are reported
// separately from Completed so callers can distinguish pre-failure and
// post-failure work without parsing error messages.
type MutationPartialError struct {
	// Completed contains steps applied before the classifying failure, in order.
	Completed []string
	// ContinuationApplied contains steps that succeeded during the indeterminate
	// continuation pass (non-nil only when Class is "indeterminate"). Nil or
	// empty when no continuation steps ran or all failed.
	ContinuationApplied []string
	// FailedStep is the name of the step that returned the classifying error.
	FailedStep string
	// CompensationState is "compensated", "not-compensated", or "unknown".
	CompensationState string
	// Class is "not-applied", "indeterminate", or "double-fault".
	Class string
	// Cause is the root-cause error (or joined errors on double-fault /
	// indeterminate with continuation failures). Wrapped with %w so
	// errors.Is/As traverse the chain.
	Cause error
}

// Error returns a human-readable summary of the partial mutation outcome.
func (e *MutationPartialError) Error() string {
	parts := []string{}
	if len(e.Completed) > 0 {
		parts = append(parts, "applied: "+strings.Join(e.Completed, ", "))
	}
	if len(e.ContinuationApplied) > 0 {
		parts = append(parts, "continuation: "+strings.Join(e.ContinuationApplied, ", "))
	}
	context := strings.Join(parts, "; ")
	if context == "" {
		return fmt.Sprintf("mutation partial (%s): failed at %q with no prior steps applied: %v",
			e.Class, e.FailedStep, e.Cause)
	}
	return fmt.Sprintf("mutation partial (%s): failed at %q (%s): %v",
		e.Class, e.FailedStep, context, e.Cause)
}

// Unwrap returns the classified root cause.
func (e *MutationPartialError) Unwrap() error {
	return e.Cause
}

// IsMutationPartial reports whether err wraps a MutationPartialError.
func IsMutationPartial(err error) bool {
	var partialErr *MutationPartialError
	return errors.As(err, &partialErr)
}
