//go:build !unix

package evidencedag

// readOnlyOpenFlags is zero where non-blocking open flags are unavailable; the
// pre-open type check and descriptor re-validation still apply.
const readOnlyOpenFlags = 0

// syncDir is an explicit no-op where directories cannot be fsynced (Windows):
// it reports success because durability there rests on the file sync alone.
func syncDir(string) error { return nil }
