package errors

import (
	stderrors "errors"
	"strings"
)

// AmbiguousWorkspaceRootError is returned when both .backlog and .backlogit
// exist in the workspace root with no BACKLOGIT_WORKSPACE_DIR override.
type AmbiguousWorkspaceRootError struct {
	Roots []string
}

// ErrAmbiguousWorkspaceRoot is the sentinel for AmbiguousWorkspaceRootError.
var ErrAmbiguousWorkspaceRoot = stderrors.New("backlogit: ambiguous workspace root")

// Error returns the formatted error string.
func (e *AmbiguousWorkspaceRootError) Error() string {
	return "ambiguous workspace root: both " + strings.Join(e.Roots, " and ") +
		" exist; set BACKLOGIT_WORKSPACE_DIR to one of the supported names or remove one"
}

// Is reports whether target matches ErrAmbiguousWorkspaceRoot.
func (e *AmbiguousWorkspaceRootError) Is(target error) bool {
	return target == ErrAmbiguousWorkspaceRoot
}
