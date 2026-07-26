package integration_test

// P-008: markdownlint gate characterization tests.
//
// These guard the crux invariant of the doctor-to-compliance provisioning
// (feature 126-F): .markdownlint.json enables EXACTLY MD001/MD025/MD041 with the
// MD025 front_matter_title retarget that makes the repo pass 0 violations
// repo-wide, and ci.yml runs the gate repo-wide (no scoped path classifier),
// SHA-pinned, via `make md-lint`.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMarkdownlintConfigEnablesExactlyP008Rules is the standing guard for the
// P-008 crux invariant. .markdownlint.json MUST enable exactly MD001, MD025, and
// MD041 (default:false, no other rule IDs), and MD025's front_matter_title MUST
// be retargeted to a non-existent `_title` key so a frontmatter `title:` is not
// counted as a heading. That retarget is what lets the repo pass 0 violations
// repo-wide without editing the 229 frontmatter-title + body-H1 artifacts;
// reverting MD025 to its default (matching `title:`) would silently re-break all
// of them. MD041 MUST stay `true` (default options) so a frontmatter `title:`
// still credits MD041 — disabling or retargeting it to `_title` would fail every
// frontmatter file.
func TestMarkdownlintConfigEnablesExactlyP008Rules(t *testing.T) {
	root := findRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".markdownlint.json"))
	require.NoError(t, err, ".markdownlint.json must exist (P-008 rule config)")

	var cfg map[string]any
	require.NoError(t, json.Unmarshal(data, &cfg), ".markdownlint.json must be valid JSON")

	allowed := map[string]bool{"default": true, "MD001": true, "MD025": true, "MD041": true}
	for k := range cfg {
		assert.Truef(t, allowed[k],
			".markdownlint.json enables unexpected key %q (only default/MD001/MD025/MD041 allowed)", k)
	}
	assert.Len(t, cfg, 4, ".markdownlint.json must contain exactly default + MD001 + MD025 + MD041")

	assert.Equal(t, false, cfg["default"], "default must be false so only the three named rules run")
	assert.Equal(t, true, cfg["MD001"], "MD001 must be enabled")
	assert.Equal(t, true, cfg["MD041"], "MD041 must stay true (default options) so a frontmatter title: credits MD041")

	md025, ok := cfg["MD025"].(map[string]any)
	require.True(t, ok, "MD025 must be an object configuring front_matter_title")
	assert.Equal(t, `^\s*_title\s*[:=]`, md025["front_matter_title"],
		"MD025.front_matter_title must retarget to the non-existent _title key so a frontmatter title: is not double-counted with the body H1")
}

// TestMarkdownLintGateIsRepoWideAndPinned asserts the ci.yml md-lint job runs
// repo-wide (the retired Option-B `md_touched` scoped classifier is absent),
// invokes `make md-lint`, and pins actions/setup-node to a full 40-char SHA.
func TestMarkdownLintGateIsRepoWideAndPinned(t *testing.T) {
	ciPath, _, _ := workflowPaths(t)
	wf := readCIWorkflow(t, ciPath)

	job, ok := wf.Jobs["md-lint"]
	require.True(t, ok, "ci.yml must define the md-lint job (repo-wide P-008 gate)")

	// Repo-wide: no scoped md_touched change classifier gates the gate.
	changes, ok := wf.Jobs["changes"]
	require.True(t, ok, "ci.yml should define the changes job")
	_, hasMdTouched := changes.Outputs["md_touched"]
	assert.False(t, hasMdTouched,
		"md_touched scoped classifier must NOT exist — the gate is repo-wide (Option-B machinery removed)")

	// setup-node present and pinned to a full 40-char SHA (F013).
	var setupNodeUses string
	for _, step := range job.Steps {
		if strings.Contains(step.Uses, "actions/setup-node") {
			setupNodeUses = step.Uses
			break
		}
	}
	require.NotEmpty(t, setupNodeUses, "md-lint job must set up Node via actions/setup-node")
	parts := strings.SplitN(setupNodeUses, "@", 2)
	require.Len(t, parts, 2, "actions/setup-node must be pinned with @<sha>")
	sha := strings.Fields(parts[1])[0] // tolerate a trailing "# vX.Y.Z" comment
	assert.Regexp(t, "^[0-9a-f]{40}$", sha, "actions/setup-node must be pinned to a full 40-char SHA (F013)")

	// The gate runs `make md-lint`.
	foundRun := false
	for _, step := range job.Steps {
		if strings.Contains(step.Run, "make md-lint") {
			foundRun = true
			break
		}
	}
	assert.True(t, foundRun, "md-lint job must run `make md-lint`")
}
