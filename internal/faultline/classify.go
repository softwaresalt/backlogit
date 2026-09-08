package faultline

// This file lands the U4b producer/consumer contract owner: the production
// Classify function and the SEC-04 safe-diagnostics primitives shared by the
// classifier and the cross-surface parity comparator.
//
// # Producer / consumer obligations (recorded, not implemented here)
//
// U4b binds obligation edges without landing any later-shipment code. The
// obligations documented here are contract text; the ACTUAL registration,
// golden fixtures, and accept-path enforcement are acceptance criteria of the
// future producer and consumer units, NOT of this task.
//
// Producers:
//   - S5-S8 (157-F, 158-F, 159-F, 160-F) and S9 (161-F) MUST emit an
//     EvidenceArtifact conforming to the U4a contract and MUST register their
//     OWN typed FamilyPayload plus a canonical golden fixture (e.g. S5
//     RepresentationEvidence, S9 RedGreenEvidence) under a schema-version bump.
//     They MUST NOT reuse ParityEvidence and MUST NOT smuggle data through
//     Detail. Each producer's dependency edge points at 156.006-T because
//     emitting/canonicalizing/validating a golden needs contract BEHAVIOR, not
//     just the body-free declarations.
//
// Consumers:
//   - S10 (162-F) and S11 (163-F) depend on this task (156.005-T): they consume
//     Classify on the decode/accept path and transitively retain the 156.006-T
//     behavior. The accept-path contract (SEC-03) is DEFINED here but NOT
//     implemented: (1) bounded DecodeAndValidate; (2) verify authenticated bytes
//     equal Canonical() output with domain separation; (3) validate producer
//     identity against a consumer-injected trust verifier; (4) atomic
//     anti-replay check-and-consume keyed by the canonical-artifact digest;
//     (5) applicability filtering; (6) status policy as the final gate
//     (Status=pass satisfies an enforced node, report_only is surfaced but does
//     NOT satisfy, fail blocks). OutcomeValid means WELL-FORMED, not satisfied.

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// MaxDiagnosticRunes bounds the rune length of any sanitized diagnostic string
// emitted by Classify or the cross-surface comparator (SEC-04). Longer inputs
// are truncated so a hostile or accidental oversized message can never produce
// an unbounded diagnostic.
const MaxDiagnosticRunes = 200

// truncationMarker is appended to a sanitized string that was truncated.
const truncationMarker = "...[truncated]"

// Classify decodes and validates a raw evidence artifact and classifies the
// outcome into one of the four typed categories. It is the four-outcome owner
// consumed by S10/S11 on the accept path.
//
// Absence is silent ONLY when raw is empty:
//   - present=false + empty raw   => OutcomeAbsent, nil error (genuinely absent).
//   - present=true  + empty raw   => OutcomeAbsent + ErrMalformed (contradictory:
//     evidence was claimed present but carried no bytes; surfaced, not silenced).
//   - present=false + non-empty raw => the raw bytes ARE decoded (never treated
//     as absent); a malformed/unknown payload is surfaced as
//     OutcomeUnknownOrMalformed rather than silently suppressed.
//
// For non-empty raw the typed DecodeAndValidate error category drives the
// outcome via errors.Is: ErrMalformed/ErrUnknownVersion/ErrUnknownFamily map to
// OutcomeUnknownOrMalformed and ErrValidation maps to OutcomeValidationFailed.
// A clean decode yields OutcomeValid with the decoded envelope and concrete
// family payload. All returned errors are wrapped as bounded, control-char-safe
// diagnostics (SEC-04) that still satisfy errors.Is against the sentinel set.
func Classify(present bool, raw []byte) (Outcome, EvidenceArtifact, FamilyPayload, error) {
	if len(raw) == 0 {
		if present {
			return OutcomeAbsent, EvidenceArtifact{}, nil, safeWrap(ErrMalformed, "malformed",
				fmt.Errorf("%w: evidence marked present but raw payload is empty (contradictory)", ErrMalformed))
		}
		return OutcomeAbsent, EvidenceArtifact{}, nil, nil
	}

	a, fp, err := DecodeAndValidate(raw)
	if err == nil {
		return OutcomeValid, a, fp, nil
	}

	switch {
	case errors.Is(err, ErrValidation):
		return OutcomeValidationFailed, a, fp, safeWrap(ErrValidation, "validation_failed", err)
	case errors.Is(err, ErrUnknownVersion):
		return OutcomeUnknownOrMalformed, a, fp, safeWrap(ErrUnknownVersion, "unknown_version", err)
	case errors.Is(err, ErrUnknownFamily):
		return OutcomeUnknownOrMalformed, a, fp, safeWrap(ErrUnknownFamily, "unknown_family", err)
	default:
		// ErrMalformed and any un-categorized decode failure fail closed as
		// unknown-or-malformed (loud, never silently absent).
		return OutcomeUnknownOrMalformed, a, fp, safeWrap(ErrMalformed, "malformed", err)
	}
}

// safeError is a bounded, sanitized diagnostic error (SEC-04). It carries a
// stable typed category and a control-char-escaped, length-capped detail while
// preserving errors.Is against the underlying sentinel via Unwrap. It never
// exposes raw bytes or unbounded values.
type safeError struct {
	category string
	detail   string
	sentinel error
}

// Error returns the bounded "category: detail" diagnostic.
func (e *safeError) Error() string { return e.category + ": " + e.detail }

// Unwrap exposes the sentinel so errors.Is keeps working on the typed category.
func (e *safeError) Unwrap() error { return e.sentinel }

// safeWrap builds a bounded, sanitized diagnostic that preserves errors.Is
// against sentinel while never surfacing raw bytes or control characters.
func safeWrap(sentinel error, category string, err error) error {
	return &safeError{
		category: category,
		detail:   SanitizeDiagnostic(err.Error()),
		sentinel: sentinel,
	}
}

// SanitizeDiagnostic returns a bounded, control-char-escaped copy of s that is
// safe to embed in diagnostics and evidence Detail fields (SEC-04). Control
// characters are escaped (never emitted raw), invalid UTF-8 is replaced, and
// the result is truncated to MaxDiagnosticRunes runes so no single value can be
// unbounded. It carries no secrets or PII of its own; callers must not pass
// sensitive material.
func SanitizeDiagnostic(s string) string {
	runes := []rune(s)
	truncated := false
	if len(runes) > MaxDiagnosticRunes {
		runes = runes[:MaxDiagnosticRunes]
		truncated = true
	}

	var b strings.Builder
	b.Grow(len(runes) + len(truncationMarker))
	for _, r := range runes {
		switch {
		case r == utf8.RuneError:
			b.WriteString(`\ufffd`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	if truncated {
		b.WriteString(truncationMarker)
	}
	return b.String()
}
