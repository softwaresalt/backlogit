package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/docline"
)

// 154.003-T (U2b) CLI harness: writeMigrateResult must render plan.Findings
// in BOTH text output (mirroring printLintText) and JSON output (a discrete
// top-level findings array). The JSON rendering is already present after U2a
// (NewMigrateReport includes findings); the text rendering is RED until U2b
// extends the text branch to emit findings like printLintText does.
//
// RED (text assertions) until writeMigrateResult's text branch renders Findings.

// planWithFindings returns a MigrationPlan that carries one decode_error
// Finding. plan.Changes is empty so there is nothing to apply or render
// beyond the finding itself.
func planWithFindings() docline.MigrationPlan {
	return docline.MigrationPlan{
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
}

// TestU2bCLIRender_TextOutputIncludesFindings verifies that writeMigrateResult
// in text mode renders plan.Findings (mirroring printLintText format).
// Pre-U2b: the text branch only renders changes and applied/skipped counts
// and does not include the finding → assertion on finding text fails RED ✓.
func TestU2bCLIRender_TextOutputIncludesFindings(t *testing.T) {
	t.Parallel()

	plan := planWithFindings()
	out, err := runDocs(t, t.TempDir(), "docs", "migrate", "--format", "text")
	// runDocs runs the whole command — for a targeted renderer test we exercise
	// writeMigrateResult directly via the docs migrate dry-run output on a tree
	// that has a finding (requires U2c); for U2b's unit gate we test the render
	// helper directly by creating a temp corpus that exercises the path.
	_ = out
	_ = err

	// Direct writeMigrateResult text invocation via a temporary docs tree:
	// build a root with only the fixture file so PlanMigration's Changes list
	// is non-empty and the corpus is rich enough to exercise the renderer.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "decisions"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "docs", "decisions", "bad.md"),
		[]byte("---\ntitle: Bad\n---\nBody.\n"), 0o644))

	// Dry-run through the CLI command, overriding the plan by constructing it
	// with a finding. Since writeMigrateResult is unexported, drive it through
	// a cobra command constructed programmatically to capture the text output.
	// The test exercises the text rendering path of the migrate dry-run:
	cmdOut, cmdErr := runDocs(t, root, "docs", "migrate", "--format", "text")
	// Pre-U2b: output does not mention findings (writeMigrateResult text branch
	// only renders changes/applied/skipped). Post-U2b: it will include finding info.
	_ = cmdErr
	_ = cmdOut

	// Plan with finding: we can't inject it into the CLI command directly, so
	// test the rendering helper via the JSON output which we CAN compare.
	cmdJSON, _ := runDocs(t, root, "docs", "migrate", "--format", "json")

	var report docline.MigrateReport
	require.NoError(t, json.Unmarshal([]byte(cmdJSON), &report))

	// The JSON path already works after U2a (findings:[] present).
	// The text path is what's missing: if the plan has a finding, the text output
	// must mention the finding file.
	// Use plan directly to test writeMigrateResult:
	root2 := t.TempDir()
	now := time.Now().UTC()
	textOut, textErr := runDocsWithPlan(t, root2, plan, now, false)
	require.NoError(t, textErr, "writeMigrateResult must not error on text output")
	assert.Contains(t, textOut, "docs/decisions/broken.md",
		"text output must include the finding's file (mirrors printLintText)")
	assert.Contains(t, textOut, docline.RuleDecodeError,
		"text output must include the finding's rule")
}

// runDocsWithPlan exercises writeMigrateResult directly by constructing a cobra
// command and invoking writeMigrateResult with the given plan.
func runDocsWithPlan(t *testing.T, root string, plan docline.MigrationPlan, _ time.Time, dryRun bool) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	var sb strings_builder
	cmd.SetOut(&sb)
	cmd.SetErr(&sb)
	// Call writeMigrateResult directly (same package — package cli).
	err := writeMigrateResult(cmd, "text", plan, nil, dryRun)
	return sb.String(), err
}

// strings_builder wraps a strings.Builder so it satisfies io.Writer for
// Cobra's SetOut without importing strings at the top level.
type strings_builder struct {
	buf []byte
}

func (b *strings_builder) Write(p []byte) (int, error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *strings_builder) String() string {
	return string(b.buf)
}
