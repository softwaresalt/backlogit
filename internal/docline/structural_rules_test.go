package docline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func rulesOf(fs []Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Rule)
	}
	return out
}

func TestU159005_FrontmatterPresent(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{"missing", "# Title\n\nBody.\n", true},
		{"present", "---\ntitle: X\n---\n# Title\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			md, err := Decode([]byte(tc.raw))
			require.NoError(t, err)
			got := checkFrontmatterPresent("docs/x.md", md)
			if tc.want {
				require.Len(t, got, 1)
				assert.Equal(t, RuleMissingFrontmatter, got[0].Rule)
				assert.Equal(t, SeverityWarn, got[0].Severity)
				assert.Equal(t, "docs/x.md", got[0].File)
			} else {
				assert.Empty(t, got)
			}
		})
	}
}

func TestU159005_Headings(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"clean", "# A\n\n## B\n\n### C\n\n## D\n", nil},
		{"two h1", "# A\n\n## B\n\n# C\n", []string{RuleMultipleH1}},
		{"skipped level", "# A\n\n### C\n", []string{RuleHeadingSkippedLevel}},
		{"fenced headings ignored", "# A\n\n```md\n# not a heading\n#### nope\n```\n\n## B\n", nil},
		{"going up is fine", "# A\n\n## B\n\n### C\n\n## D\n", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rulesOf(checkHeadings("docs/x.md", tc.body))
			if tc.want == nil {
				assert.Empty(t, got)
			} else {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestU159005_Tables(t *testing.T) {
	clean := "| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n"
	assert.Empty(t, checkTables("docs/x.md", clean))

	escaped := "| a | b |\n|---|---|\n| x \\| y | `p|q` |\n"
	assert.Empty(t, checkTables("docs/x.md", escaped))

	fenced := "```\n| a | b |\n|---|---|\n| 1 |\n```\n"
	assert.Empty(t, checkTables("docs/x.md", fenced))

	bad := "| a | b |\n|---|---|\n| 1 | 2 | 3 |\n| 4 |\n"
	got := checkTables("docs/x.md", bad)
	require.Len(t, got, 2)
	for _, f := range got {
		assert.Equal(t, RuleTableShape, f.Rule)
		assert.Equal(t, SeverityWarn, f.Severity)
	}
}

func TestU159005_Claims(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "internal/pkg/a.go", "package pkg\n")
	writeDoc(t, root, "internal/pkg/b.go", "package pkg\n")

	good := "<!-- docline:count glob=\"internal/pkg/*.go\" expect=2 -->\n<!-- docline:topology path=\"internal/pkg\" kind=dir -->\n<!-- docline:topology path=\"internal/pkg/a.go\" kind=file -->\n"
	assert.Empty(t, checkClaims(root, "docs/x.md", good))

	badCount := "<!-- docline:count glob=\"internal/pkg/*.go\" expect=3 -->\n"
	got := checkClaims(root, "docs/x.md", badCount)
	require.Len(t, got, 1)
	assert.Equal(t, RuleNumericClaim, got[0].Rule)

	badTopo := "<!-- docline:topology path=\"internal/nope\" kind=dir -->\n<!-- docline:topology path=\"internal/pkg\" kind=file -->\n"
	got = checkClaims(root, "docs/x.md", badTopo)
	require.Len(t, got, 2)
	for _, f := range got {
		assert.Equal(t, RuleTopologyClaim, f.Rule)
	}

	escape := "<!-- docline:count glob=\"../*.go\" expect=0 -->\n<!-- docline:topology path=\"..\" kind=dir -->\n"
	got = checkClaims(root, "docs/x.md", escape)
	require.Len(t, got, 2)

	fenced := "```\n<!-- docline:count glob=\"internal/pkg/*.go\" expect=99 -->\n```\n"
	assert.Empty(t, checkClaims(root, "docs/x.md", fenced))
}

func TestU159005_LintTreeOptIn(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/decisions/x.md", "---\ntitle: X\nsource: y\ndoc_type: decision\n---\n# A\n\n### C\n")

	base, err := LintTree(Options{Root: root})
	require.NoError(t, err)
	assert.Empty(t, base, "structural rules are opt-in")

	got, err := LintTree(Options{Root: root, Rules: RuleSet{Structural: true}})
	require.NoError(t, err)
	assert.Equal(t, []string{RuleHeadingSkippedLevel}, rulesOf(got))
}

func TestCountClaim_OutwardDirSymlinkZeroMatchesFlagged(t *testing.T) {
	outside := t.TempDir()
	root := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	got := checkClaims(root, "docs/x.md", "<!-- docline:count glob=\"link/*.go\" expect=0 -->\n")
	require.Len(t, got, 1)
	assert.Equal(t, RuleNumericClaim, got[0].Rule)
}

func TestCountClaim_MissingDirZeroMatchesPasses(t *testing.T) {
	root := t.TempDir()
	assert.Empty(t, checkClaims(root, "docs/x.md", "<!-- docline:count glob=\"nope/*.go\" expect=0 -->\n"))
}
