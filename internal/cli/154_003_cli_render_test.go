package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// TestU2bCLIApply_MixedCorpus_RejectsWithFindingsAndZeroWrites is the
// full Cobra command-path regression test for the ErrPlanHasFindings rejection
// branch: it exercises the complete docs migrate --apply pipeline with a real
// malformed file corpus, verifying render-before-return, dry_run:true in JSON,
// non-zero exit, findings array, and zero writes. This is the test the
// writeMigrateResult-direct tests cannot cover (they bypass newDocsMigrateCommand,
// SilenceErrors, and the dryRun=true render call entirely).
func TestU2bCLIApply_MixedCorpus_RejectsWithFindingsAndZeroWrites(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	// Malformed frontmatter file.
	writeFixtureDoc(t, root, "docs/decisions/broken.md",
		"---\ntitle: [unclosed yaml\n---\nBody.\n")
	// Valid (but unnormalized) file alongside — would be migrated if apply succeeded.
	writeFixtureDoc(t, root, "docs/decisions/valid.md",
		"---\ntitle: Valid Decision\n---\nBody.\n")

	// Record pre-apply state of valid.md.
	validPath := filepath.Join(root, filepath.FromSlash("docs/decisions/valid.md"))
	before, err := os.ReadFile(validPath)
	require.NoError(t, err)

	// Run the full Cobra command (not writeMigrateResult directly).
	out, cmdErr := runDocs(t, root, "docs", "migrate",
		"--apply", "--yes", "--path", "docs/decisions", "--format", "json")

	// Non-zero exit — ErrPlanHasFindings must be returned.
	require.Error(t, cmdErr, "docs migrate --apply must fail non-zero when plan has findings")
	require.ErrorIs(t, cmdErr, docline.ErrPlanHasFindings,
		"exit error must be ErrPlanHasFindings")

	// JSON output must carry dry_run:true (nothing was applied).
	var report map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(out), &report),
		"output must be valid JSON even on rejection")
	assert.Equal(t, true, report["dry_run"],
		"dry_run must be true on rejection (nothing was written)")

	// JSON output must carry findings with the decode_error for broken.md.
	findings, ok := report["findings"].([]interface{})
	require.True(t, ok, "findings must be a JSON array")
	require.NotEmpty(t, findings, "findings must be non-empty for a malformed corpus")
	first := findings[0].(map[string]interface{})
	assert.Equal(t, "docs/decisions/broken.md", first["file"],
		"finding must reference the malformed file")
	assert.Equal(t, "decode_error", first["rule"],
		"finding rule must be decode_error")

	// ZERO writes: valid.md must be unchanged.
	after, readErr := os.ReadFile(validPath)
	require.NoError(t, readErr)
	assert.Equal(t, string(before), string(after),
		"docs migrate --apply must write ZERO files when plan carries findings")
}
