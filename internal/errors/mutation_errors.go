package errors

import (
	"errors"
	"fmt"
	"strings"
)

// MutationPartialError reports a multi-step mutation that failed after the
// envelope had enough information to classify the outcome.
type MutationPartialError struct {
	Completed         []string
	FailedStep        string
	CompensationState string
	Class             string
	Cause             error
}

// Error returns the human-readable partial-mutation summary.
func (e *MutationPartialError) Error() string {
	if len(e.Completed) == 0 {
		return fmt.Sprintf("mutation partial (%s): failed at %q with no prior steps applied: %v",
			e.Class, e.FailedStep, e.Cause)
	}
	return fmt.Sprintf("mutation partial (%s): failed at %q after %s: %v",
		e.Class, e.FailedStep, strings.Join(e.Completed, ", "), e.Cause)
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
