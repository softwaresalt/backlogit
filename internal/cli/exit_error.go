package cli

import "errors"

// ExitError carries an explicit process exit code so main can honor a versioned
// exit-code contract (e.g. doctor --target's 0/1/2/3/4 table) instead of
// collapsing every command failure to exit 1, which is Cobra's default.
type ExitError struct {
	// Code is the process exit code to return.
	Code int
	// Msg is a short human-readable summary of the outcome.
	Msg string
	// Err is the optional underlying error this ExitError classifies. When
	// set, Unwrap exposes it so errors.Is/errors.As can still see through an
	// *ExitError to the original typed/sentinel error it wraps (e.g. a
	// caller checking errors.Is(err, corerrors.ErrConfirmationRequired)),
	// instead of the exit-code mapping opaquely discarding that chain.
	Err error
}

// Error implements the error interface.
func (e *ExitError) Error() string { return e.Msg }

// ExitCode returns the explicit process exit code.
func (e *ExitError) ExitCode() int { return e.Code }

// Unwrap exposes the wrapped underlying error (if any) so errors.Is/errors.As
// can see through an *ExitError to the original error it classifies.
func (e *ExitError) Unwrap() error { return e.Err }

// ExitCodeFor returns the explicit exit code carried by err when present,
// 0 for a nil error, or 1 for any other (generic) failure.
func ExitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var ee *ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return 1
}
