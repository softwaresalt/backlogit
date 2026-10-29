package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Sentinel errors for the authorization-token parser (170.012-T).
var (
	// ErrTokenMalformed reports a token that is not well-formed JSON or is
	// structurally invalid.
	ErrTokenMalformed = errors.New("authorization token malformed")
	// ErrTokenDuplicateMember reports a duplicate JSON object member name at
	// any object depth; last-wins acceptance is never allowed.
	ErrTokenDuplicateMember = errors.New("authorization token duplicate JSON member")
)

// TokenParseError is the typed error returned by ParseAuthorizationToken.
type TokenParseError struct {
	// Path names the object the failure concerns (for example "token",
	// "token.header", "token.claims", "token.binding").
	Path string
	// Err is the sentinel category.
	Err error
}

// Error implements error.
func (e *TokenParseError) Error() string {
	return "parse authorization token " + e.Path + ": " + e.Err.Error()
}

// Unwrap returns the sentinel category.
func (e *TokenParseError) Unwrap() error { return e.Err }

// ParseAuthorizationToken parses an operator authorization token. Duplicate
// member names are rejected at every object depth (token root, header, claims,
// binding) before any decode, so a last-wins decode can never accept an
// ambiguous document. It performs no signature or claim verification.
func ParseAuthorizationToken(data []byte) (AuthorizationToken, error) {
	var token AuthorizationToken
	if err := scanTokenMembers(data, "token", tokenMemberKey); err != nil {
		return token, err
	}
	if err := json.Unmarshal(data, &token); err != nil {
		return AuthorizationToken{}, &TokenParseError{Path: "token", Err: fmt.Errorf("%w: %w", ErrTokenMalformed, err)}
	}
	return token, nil
}

// tokenMemberKey maps a decoded member name to its collision key. It reuses
// the shared encoding/json-equivalent fold so exact, case-fold and
// Unicode-escape variants of one member name collide at every depth
// (170.012-T exact bytes, 170.018-T decoder-equivalent).
func tokenMemberKey(name string) string { return canonicalJSONMemberName(name) }

type tokenScanFrame struct {
	label     string
	object    bool
	expectKey bool
	lastKey   string
	index     int
	seen      map[string]struct{}
}

// scanTokenMembers walks data token by token and fails with a typed
// TokenParseError when the document is malformed, has data after the root
// value, or any object at any depth repeats a member (as judged by keyOf).
func scanTokenMembers(data []byte, rootLabel string, keyOf func(string) string) error {
	malformed := func(label string, err error) error {
		return &TokenParseError{Path: label, Err: fmt.Errorf("%w: %w", ErrTokenMalformed, err)}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var stack []tokenScanFrame
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
		tok, err := decoder.Token()
		if err != nil {
			return malformed(rootLabel, err)
		}
		if delim, ok := tok.(json.Delim); ok {
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
				frame := tokenScanFrame{label: label}
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
			key, _ := tok.(string)
			top := &stack[n-1]
			collision := keyOf(key)
			if _, dup := top.seen[collision]; dup {
				return &TokenParseError{Path: top.label, Err: fmt.Errorf("%w: %q", ErrTokenDuplicateMember, key)}
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
