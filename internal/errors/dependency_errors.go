package errors

import "errors"

// ErrInvalidDependencyType is returned when a dependency edge type is not one
// of the accepted values: blocks, relates_to, parent_of.
var ErrInvalidDependencyType = errors.New("backlogit: invalid dependency type")

// ValidDependencyType reports whether t is an accepted dep_type value.
// Accepted values: blocks, relates_to, parent_of.
func ValidDependencyType(t string) bool {
	return t == "blocks" || t == "relates_to" || t == "parent_of"
}
