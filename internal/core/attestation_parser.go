package core

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
// Duplicate member names are rejected at every object depth (envelope root,
// statement, subject, predicate) before any decode, so a last-wins decode can
// never accept an ambiguous document.
func ParseAttestation(data []byte) (ParsedAttestation, error) {
	var parsed ParsedAttestation
	if err := scanAttestationMembers(data, "envelope"); err != nil {
		return parsed, err
	}
	if err := json.Unmarshal(data, &parsed.Envelope); err != nil {
		return parsed, &AttestationParseError{Path: "envelope", Err: fmt.Errorf("%w: %w", ErrAttestationMalformed, err)}
	}
	payload, err := base64.StdEncoding.DecodeString(parsed.Envelope.Payload)
	if err != nil {
		return parsed, &AttestationParseError{Path: "envelope.payload", Err: fmt.Errorf("%w: %w", ErrAttestationMalformed, err)}
	}
	parsed.PayloadBytes = payload
	if err := scanAttestationMembers(payload, "statement"); err != nil {
		return parsed, err
	}
	if err := json.Unmarshal(payload, &parsed.Statement); err != nil {
		return parsed, &AttestationParseError{Path: "statement", Err: fmt.Errorf("%w: %w", ErrAttestationMalformed, err)}
	}
	return parsed, nil
}

// attestationMemberKey maps a decoded member name to its collision key. It
// reuses the shared encoding/json-equivalent fold so case-fold and
// Unicode-escape variants of one member name collide (169.015-T).
func attestationMemberKey(name string) string { return canonicalJSONMemberName(name) }

type attestationScanFrame struct {
	label     string
	object    bool
	expectKey bool
	lastKey   string
	index     int
	seen      map[string]struct{}
}

// scanAttestationMembers walks data token by token and fails with a typed
// AttestationParseError when the document is malformed, has data after the
// root value, or any object at any depth repeats a member name. Path labels
// name the failing object relative to rootLabel.
func scanAttestationMembers(data []byte, rootLabel string) error {
	malformed := func(label string, err error) error {
		return &AttestationParseError{Path: label, Err: fmt.Errorf("%w: %w", ErrAttestationMalformed, err)}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var stack []attestationScanFrame
	finish := func() bool {
		if len(stack) == 0 {
			return true
		}
		top := &stack[len(stack)-1]
		if top.object {
			top.expectKey = true
		} else {
			top.index++
		}
		return false
	}
	for done := false; !done; {
		token, err := decoder.Token()
		if err != nil {
			return malformed(rootLabel, err)
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{', '[':
				label := rootLabel
				if n := len(stack); n > 0 {
					if top := stack[n-1]; top.object {
						label = top.label + "." + top.lastKey
					} else {
						label = fmt.Sprintf("%s[%d]", top.label, top.index)
					}
				}
				frame := attestationScanFrame{label: label}
				if delim == '{' {
					frame.object, frame.expectKey, frame.seen = true, true, make(map[string]struct{})
				}
				stack = append(stack, frame)
			default:
				stack = stack[:len(stack)-1]
				done = finish()
			}
			continue
		}
		if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].expectKey {
			key, _ := token.(string)
			top := &stack[n-1]
			collision := attestationMemberKey(key)
			if _, dup := top.seen[collision]; dup {
				return &AttestationParseError{Path: top.label, Err: fmt.Errorf("%w: %q", ErrAttestationDuplicateMember, key)}
			}
			top.seen[collision] = struct{}{}
			top.lastKey = key
			top.expectKey = false
			continue
		}
		done = finish()
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return malformed(rootLabel, errors.New("unexpected data after the root value"))
	}
	return nil
}
