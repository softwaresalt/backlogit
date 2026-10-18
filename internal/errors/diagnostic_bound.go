package errors

import "unicode/utf8"

// DiagnosticBudget is the maximum byte length of a caller-facing diagnostic
// built from a wrapped cause that may echo caller-controlled input.
const DiagnosticBudget = 512

// TruncationMarker is appended when a diagnostic is shortened.
const TruncationMarker = "…[truncated]"

// BoundDiagnostic returns s unchanged when it fits within DiagnosticBudget
// bytes; otherwise it returns a prefix (never cut mid-rune) followed by
// TruncationMarker, with total length <= DiagnosticBudget. The budget and the
// marker length (len(TruncationMarker)) are measured in bytes, not runes. The
// budget applies to the echoed cause only; callers may append fixed opt-in
// advice after the bounded text. The result is valid UTF-8 only when s is
// valid UTF-8; invalid input is not repaired here (JSON marshaling sanitizes
// any remaining invalid bytes).
func BoundDiagnostic(s string) string {
	if len(s) <= DiagnosticBudget {
		return s
	}
	cut := DiagnosticBudget - len(TruncationMarker)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + TruncationMarker
}
