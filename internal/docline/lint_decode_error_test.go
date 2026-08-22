package docline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLintTree_MalformedFrontmatter_ReportsFindingAndContinues covers all
// three scenarios of 146.016-T (U7a): a real on-disk fixture corpus
// containing one file whose YAML frontmatter cannot be decoded, plus at
// least two files with ordinary contract violations, must still yield
// findings for every file. No production seam is introduced: mdfront.Decode
// already returns a hard error on malformed YAML, so a genuine fixture such
// as `---\ntitle: [unclosed\n---\nBody.\n` produces a path-selective decode
// failure with no injection at all.
func TestLintTree_MalformedFrontmatter_ReportsFindingAndContinues(t *testing.T) {
	root := t.TempDir()

	// Genuinely undecodable YAML frontmatter (unclosed flow sequence).
	writeDoc(t, root, "docs/decisions/broken.md", "---\ntitle: [unclosed\n---\nBody.\n")
	// Two ordinary contract violations (missing source + doc_type is
	// implicit from directory classification, so these are missing source
	// only, which is a real per-file finding under the authoring profile).
	writeDoc(t, root, "docs/decisions/missing-source-1.md", "---\ntitle: Missing Source One\n---\nBody.\n")
	writeDoc(t, root, "docs/decisions/missing-source-2.md", "---\ntitle: Missing Source Two\n---\nBody.\n")

	findings, err := LintTree(Options{Root: root, Profile: ProfileAuthoring})
	require.NoError(t, err, "a per-file frontmatter decode failure must not abort the scan")
	require.NotNil(t, findings, "scenario 3: a corpus whose only problem is a decode failure must still yield a non-nil findings slice")

	// Scenario 1: the undecodable file yields exactly one finding with
	// Rule: RuleDecodeError and Severity: SeverityError.
	var brokenFindings []Finding
	for _, f := range findings {
		if f.File == "docs/decisions/broken.md" {
			brokenFindings = append(brokenFindings, f)
		}
	}
	require.Len(t, brokenFindings, 1, "the undecodable file must yield exactly one finding")
	assert.Equal(t, RuleDecodeError, brokenFindings[0].Rule)
	assert.Equal(t, SeverityError, brokenFindings[0].Severity)

	// Scenario 2: findings for the remaining files are still present,
	// proving the scan continued past the decode failure.
	var otherFiles []string
	for _, f := range findings {
		if f.File != "docs/decisions/broken.md" {
			otherFiles = append(otherFiles, f.File)
		}
	}
	assert.Contains(t, otherFiles, "docs/decisions/missing-source-1.md")
	assert.Contains(t, otherFiles, "docs/decisions/missing-source-2.md")
}

// TestLintTree_OnlyDecodeFailure_YieldsNonNilFindings is scenario 3 of
// 146.016-T (U7a) in isolation: a corpus whose only problem is the
// undecodable file still yields a non-empty findings slice rather than a
// nil slice with an error.
func TestLintTree_OnlyDecodeFailure_YieldsNonNilFindings(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/decisions/only-broken.md", "---\ntitle: [unclosed\n---\nBody.\n")

	findings, err := LintTree(Options{Root: root, Profile: ProfileAuthoring})
	require.NoError(t, err)
	require.NotEmpty(t, findings)
	assert.Equal(t, RuleDecodeError, findings[0].Rule)
}
