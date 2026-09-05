package docline_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/docline"
)

// TestU3_ValidateClassifyPathRejectsEmpty asserts that an empty path is rejected (155.003-T / U3).
func TestU3_ValidateClassifyPathRejectsEmpty(t *testing.T) {
	err := docline.ValidateClassifyPath("/some/root", "")
	require.Error(t, err, "empty path must be rejected")
	assert.Contains(t, err.Error(), "empty")
}

// TestU3_ValidateClassifyPathRejectsBlank asserts that a whitespace-only path is rejected.
func TestU3_ValidateClassifyPathRejectsBlank(t *testing.T) {
	err := docline.ValidateClassifyPath("/some/root", "   ")
	require.Error(t, err, "blank path must be rejected")
	assert.Contains(t, err.Error(), "empty")
}

// TestU3_ValidateClassifyPathRejectsAbsolute asserts that a leading-/ path is rejected.
func TestU3_ValidateClassifyPathRejectsAbsolute(t *testing.T) {
	err := docline.ValidateClassifyPath("/some/root", "/docs/foo.md")
	require.Error(t, err, "absolute path (/) must be rejected")
	assert.Contains(t, err.Error(), "relative")
}

// TestU3_ValidateClassifyPathRejectsAbsoluteBackslash asserts that a leading-\ path is rejected.
func TestU3_ValidateClassifyPathRejectsAbsoluteBackslash(t *testing.T) {
	err := docline.ValidateClassifyPath("/some/root", `\docs\foo.md`)
	require.Error(t, err, `absolute path (\) must be rejected`)
	assert.Contains(t, err.Error(), "relative")
}

// TestU3_ValidateClassifyPathRejectsVolumeLetter asserts that volume-qualified paths (C:\) are rejected.
func TestU3_ValidateClassifyPathRejectsVolumeLetter(t *testing.T) {
	err := docline.ValidateClassifyPath("/some/root", `C:\docs\foo.md`)
	require.Error(t, err, "volume-qualified path must be rejected")
	assert.Contains(t, err.Error(), "relative")
}

// TestU3_ValidateClassifyPathRejectsUNC asserts that UNC paths (\\host\share) are rejected.
func TestU3_ValidateClassifyPathRejectsUNC(t *testing.T) {
	err := docline.ValidateClassifyPath("/some/root", `\\host\share\docs`)
	require.Error(t, err, "UNC path must be rejected")
	assert.Contains(t, err.Error(), "relative")
}

// TestU3_ValidateClassifyPathAcceptsRelative asserts that a valid relative path passes.
func TestU3_ValidateClassifyPathAcceptsRelative(t *testing.T) {
	root := t.TempDir()
	err := docline.ValidateClassifyPath(root, "docs/decisions/x.md")
	assert.NoError(t, err, "valid relative path must be accepted")
}

// TestU3_ClassifyParityMCPEquivalesCLI asserts that ValidateClassifyPath followed by
// Classify produces consistent results regardless of surface (CLI/MCP parity).
func TestU3_ClassifyParityMCPEquivalesCLI(t *testing.T) {
	root := t.TempDir()
	tests := []struct {
		path     string
		expected docline.DocType
	}{
		{"docs/decisions/001-arch.md", docline.DocTypeDecision},
		{"docs/reviews/rev.md", docline.DocTypeReview},
		{"docs/guides/how-to.md", docline.DocTypeGuide},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			// ValidateClassifyPath is called by BOTH CLI and MCP — test the shared path.
			err := docline.ValidateClassifyPath(root, tc.path)
			require.NoError(t, err, "valid relative path should not fail containment check")
			dt := docline.Classify(tc.path)
			assert.Equal(t, tc.expected, dt, "Classify result must be consistent across surfaces")
		})
	}
}
