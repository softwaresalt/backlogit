package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/docline"
)

// 154.003-T (U2b) CLI harness: writeMigrateResult text output must render
// plan.Findings (mirroring printLintText format).

// byteBuf satisfies io.Writer and collects written bytes.
type byteBuf struct{ buf []byte }

func (b *byteBuf) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}
func (b *byteBuf) String() string { return string(b.buf) }

// runWriteMigrateResult invokes writeMigrateResult with the given plan and
// returns the text output.
func runWriteMigrateResult(t *testing.T, plan docline.MigrationPlan, dryRun bool) string {
	t.Helper()
	cmd := NewRootCommand()
	var buf byteBuf
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	err := writeMigrateResult(cmd, "text", plan, nil, dryRun)
	require.NoError(t, err)
	return buf.String()
}

// TestU2bCLIRender_TextOutputIncludesFindings asserts that writeMigrateResult
// text output includes the finding's file and rule when plan.Findings is
// non-empty (mirrors printLintText format).
func TestU2bCLIRender_TextOutputIncludesFindings(t *testing.T) {
	t.Parallel()

	plan := docline.MigrationPlan{
		Changes: []docline.Change{},
		Findings: []docline.Finding{
			{
				File:     "docs/decisions/broken.md",
				Rule:     docline.RuleDecodeError,
				Severity: docline.SeverityError,
				Fix:      "malformed YAML frontmatter near line 1",
			},
		},
	}

	out := runWriteMigrateResult(t, plan, true)
	assert.Contains(t, out, "docs/decisions/broken.md",
		"text output must include the finding's file (mirrors printLintText)")
	assert.Contains(t, out, docline.RuleDecodeError,
		"text output must include the finding's rule")
	assert.Contains(t, out, "1 finding(s)",
		"text output must include the finding count")
}

// TestU2bCLIRender_EmptyFindingsTextOmitsFindingsSection asserts that when
// plan.Findings is empty the text output does not print a findings section.
// This must stay GREEN before and after U2b.
func TestU2bCLIRender_EmptyFindingsTextOmitsFindingsSection(t *testing.T) {
	t.Parallel()

	plan := docline.MigrationPlan{Changes: []docline.Change{}}
	out := runWriteMigrateResult(t, plan, true)
	assert.NotContains(t, out, "finding(s)",
		"text output must not print a findings section when findings is empty")
}
