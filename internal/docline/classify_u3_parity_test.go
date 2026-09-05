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

// TestU3_ValidateClassifyPathRejectsDotSegments asserts that non-canonical paths with "."
// or ".." segments are rejected, preventing misclassification (Copilot review thread 2).
// SafeResolve accepts in-root dot segments since they do not escape; ValidateClassifyPath
// must explicitly reject them to prevent e.g. "docs/decisions/../reviews/x.md" being
// classified as "decision" instead of "review".
func TestU3_ValidateClassifyPathRejectsDotSegments(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name string
		path string
	}{
		{"double-dot traversal", "docs/decisions/../reviews/x.md"},
		{"leading dot-segment", "./docs/foo.md"},
		{"single dot in middle", "docs/./reviews/x.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := docline.ValidateClassifyPath(root, tc.path)
			require.Error(t, err, "dot-segment path %q must be rejected", tc.path)
			assert.Contains(t, err.Error(), "dot-segment", "error message must mention dot-segment")
		})
	}
}

// TestU3_ValidateClassifyPathAcceptsRelative asserts that a valid relative path passes.
func TestU3_ValidateClassifyPathAcceptsRelative(t *testing.T) {
	root := t.TempDir()
	err := docline.ValidateClassifyPath(root, "docs/decisions/x.md")
	assert.NoError(t, err, "valid relative path must be accepted")
}

// TestU3_ClassifyParityMCPEquivalesCLI asserts that ValidateClassifyPath followed by
// Classify produces consistent results across both surfaces. The shared helper is tested
// here; actual CLI and MCP surface invocations are covered in internal/cli/ and
// internal/mcp/ to avoid cross-package import concerns in this package.
// Cross-surface rejection parity (empty, absolute, volume/UNC, traversal, dot-segment)
// is verified in TestU3_DocsClassify* in internal/mcp/docs_classify_u3_test.go and
// TestU3_CLI* in internal/cli/ test files.
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
			// Both CLI and MCP call ValidateClassifyPath then Classify.
			err := docline.ValidateClassifyPath(root, tc.path)
			require.NoError(t, err, "valid relative path should not fail containment check")
			dt := docline.Classify(tc.path)
			assert.Equal(t, tc.expected, dt, "Classify result must be consistent across surfaces")
		})
	}
}

