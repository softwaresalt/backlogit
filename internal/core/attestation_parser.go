package core

import (
	"errors"
)

// Sentinel errors for the attestation envelope parser (169.010-T).
var (
	// ErrAttestationMalformed reports an envelope or statement that is not
	// well-formed or structurally valid.
	ErrAttestationMalformed = errors.New("attestation malformed")
	// ErrAttestationDuplicateMember reports a duplicate JSON object member
	// name at any object depth; last-wins acceptance is never allowed.
	ErrAttestationDuplicateMember = errors.New("attestation duplicate JSON member")
)

// AttestationParseError is the typed error returned by ParseAttestation.
type AttestationParseError struct {
	// Path names the object the failure concerns (for example "envelope",
	// "statement", "statement.subject[0]", "statement.predicate").
	Path string
	// Err is the sentinel category.
	Err error
}

// Error implements error.
func (e *AttestationParseError) Error() string {
	return "parse attestation " + e.Path + ": " + e.Err.Error()
}

// Unwrap returns the sentinel category.
func (e *AttestationParseError) Unwrap() error { return e.Err }

// ParsedAttestation is a successfully parsed attestation.
type ParsedAttestation struct {
	// Envelope is the parsed DSSE envelope.
	Envelope AttestationEnvelope
	// PayloadBytes is the decoded envelope payload: the exact bytes the
	// signature covers.
	PayloadBytes []byte
	// Statement is the parsed in-toto statement.
	Statement AttestationStatement
}

// ParseAttestation parses an attestation envelope and its embedded statement.
// Not yet implemented (169.010-T harness stub).
func ParseAttestation(data []byte) (ParsedAttestation, error) {
	return ParsedAttestation{}, errors.New("not implemented")
}
