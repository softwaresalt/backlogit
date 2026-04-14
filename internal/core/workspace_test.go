package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestSafeResolve_ValidPath(t *testing.T) {
	// Arrange
	root := t.TempDir()

	// Act
	resolved, err := core.SafeResolve(root, "subdir/file.md")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, resolved, "subdir")
}

func TestSafeResolve_PathTraversal(t *testing.T) {
	// Arrange
	root := t.TempDir()

	// Act
	_, err := core.SafeResolve(root, "../../etc/passwd")

	// Assert
	assert.Error(t, err)
}
