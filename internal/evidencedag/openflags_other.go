//go:build !unix

package evidencedag

// readOnlyOpenFlags is zero where non-blocking open flags are unavailable; the
// pre-open type check and descriptor re-validation still apply.
const readOnlyOpenFlags = 0
