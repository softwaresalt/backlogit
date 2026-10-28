package config

import (
	"errors"
	"fmt"
)

// Sentinel errors for trust_anchors validation. Each is wrapped by a
// *TrustAnchorValidationError so callers can use errors.Is for the category
// and errors.As for the offending anchor.
var (
	// ErrTrustAnchorDuplicateID reports two trust anchors sharing one id.
	ErrTrustAnchorDuplicateID = errors.New("duplicate trust anchor id")
	// ErrTrustAnchorDisallowedAlgo reports an algo outside the allowlist.
	ErrTrustAnchorDisallowedAlgo = errors.New("trust anchor algo not allowed")
)

// TrustAnchorValidationError is the typed validation error returned for an
// invalid trust_anchors entry.
type TrustAnchorValidationError struct {
	// Index is the zero-based position of the offending anchor.
	Index int
	// ID is the anchor id (may be empty).
	ID string
	// Field is the YAML field name that failed validation.
	Field string
	// Err is the sentinel category (see the ErrTrustAnchor* variables).
	Err error
}

// Error implements error.
func (e *TrustAnchorValidationError) Error() string {
	return fmt.Sprintf("trust_anchors[%d] (id %q) field %s: %v", e.Index, e.ID, e.Field, e.Err)
}

// Unwrap returns the sentinel category.
func (e *TrustAnchorValidationError) Unwrap() error { return e.Err }
