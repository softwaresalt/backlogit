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
	"fmt"
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
// frontmatter file. The test also guards the runner config (.markdownlint-cli2.jsonc)
// so no rule-altering key there can change the EFFECTIVE merged rule set.
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
	assert.Len(t, md025, 1,
		"MD025 must configure ONLY front_matter_title — a stray key such as `level` would pass a value-only check while silently changing which heading level MD025 treats as the title")
	assert.Equal(t, `^\s*_title\s*[:=]`, md025["front_matter_title"],
		"MD025.front_matter_title must retarget to the non-existent _title key so a frontmatter title: is not double-counted with the body H1")

	// Effective-config guard: cli2 loads BOTH the rule config (.markdownlint.json,
	// auto-discovered — empirically verified: a 320-char line yields no MD013, so
	// default:false from .markdownlint.json is active) AND the runner config
	// (.markdownlint-cli2.jsonc). The runner config can carry rule-altering keys
	// (`config`, `customRules`, `overrides`, bare MD### / `default`) that would
	// change the EFFECTIVE rule set, and scope-altering keys (`ignores`, `globs`)
	// that would narrow the repo-wide corpus — either could pass silently while the
	// checks above and the lint run stay green. Enforce that the runner config
	// contains ONLY the `gitignore` runner option, so the merged config is exactly
	// {MD001, MD025, MD041} with default:false over the full non-gitignored corpus.
	cli2Raw, err := os.ReadFile(filepath.Join(root, ".markdownlint-cli2.jsonc"))
	require.NoError(t, err, ".markdownlint-cli2.jsonc must exist (runner options)")

	var cli2 map[string]any
	require.NoError(t, json.Unmarshal(stripJSONCComments(cli2Raw), &cli2),
		".markdownlint-cli2.jsonc must be valid JSONC (comments stripped)")

	ruleAltering := map[string]bool{"config": true, "customRules": true, "overrides": true, "default": true}
	scopeAltering := map[string]bool{"ignores": true, "globs": true}
	allowedRunnerKeys := map[string]bool{"gitignore": true}
	for k := range cli2 {
		assert.Falsef(t, ruleAltering[k],
			".markdownlint-cli2.jsonc must not carry the rule-altering key %q (rules belong in .markdownlint.json)", k)
		assert.Falsef(t, scopeAltering[k],
			".markdownlint-cli2.jsonc must not carry the scope-altering key %q (ignores/globs would narrow the repo-wide corpus)", k)
		assert.NotRegexpf(t, `^(?i)MD\d+$`, k,
			".markdownlint-cli2.jsonc must not enable/override rule %q directly (rules belong in .markdownlint.json)", k)
		assert.Truef(t, allowedRunnerKeys[k],
			".markdownlint-cli2.jsonc must contain only the gitignore runner option (unexpected key %q)", k)
	}
	assert.Equal(t, true, cli2["gitignore"],
		".markdownlint-cli2.jsonc must set gitignore:true so the gate lints the non-gitignored corpus (exactly the tracked set in a clean CI checkout; local==CI parity)")
}

// stripJSONCComments removes // line comments from JSONC so the runner config can
// be parsed as JSON. It is string-aware (does not strip // that appears inside a
// double-quoted string, e.g. a URL) and does not handle block comments, which the
// runner config does not use.
func stripJSONCComments(b []byte) []byte {
	var out strings.Builder
	for _, line := range strings.Split(string(b), "\n") {
		inString := false
		escaped := false
		cut := -1
		for i := 0; i < len(line); i++ {
			c := line[i]
			if escaped {
				escaped = false
				continue
			}
			switch {
			case c == '\\' && inString:
				escaped = true
			case c == '"':
				inString = !inString
			case c == '/' && !inString && i+1 < len(line) && line[i+1] == '/':
				cut = i
			}
			if cut >= 0 {
				break
			}
		}
		if cut >= 0 {
			line = line[:cut]
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return []byte(out.String())
}

// TestMarkdownLintGateIsRepoWideAndPinned asserts the ci.yml md-lint job runs
// repo-wide (the retired Option-B `md_touched` scoped classifier is absent),
// always runs (no `needs`/`if` gating), invokes `make md-lint`, and pins
// actions/setup-node to a full 40-char SHA at the required node-version "22".
func TestMarkdownLintGateIsRepoWideAndPinned(t *testing.T) {
	ciPath, _, _ := workflowPaths(t)
	wf := readCIWorkflow(t, ciPath)

	job, ok := wf.Jobs["md-lint"]
	require.True(t, ok, "ci.yml must define the md-lint job (repo-wide P-008 gate)")

	// Always-running contract: the repo-wide gate must not be conditioned on any
	// other job (`needs`) or expression (`if`) that could skip it and let a
	// violating change through with the required check reported as passed/skipped.
	assert.Empty(t, job.Needs,
		"md-lint job must have no `needs` — it runs unconditionally as a repo-wide gate")
	assert.Empty(t, job.If,
		"md-lint job must have no `if` — it runs unconditionally as a repo-wide gate")

	// Hard-fail contract: neither the md-lint job nor any of its steps may set
	// `continue-on-error` truthy. A `continue-on-error: true` on the job or the
	// lint step would let a markdownlint violation report as a non-blocking
	// step — silently degrading the gate to advisory while every other assertion
	// in this guard still passes. Require it unset or explicitly false.
	assert.Truef(t, continueOnErrorHardFails(job.ContinueOnError),
		"md-lint job must not set continue-on-error truthy (got %v) — the gate must hard-fail on violations", job.ContinueOnError)
	for _, step := range job.Steps {
		assert.Truef(t, continueOnErrorHardFails(step.ContinueOnError),
			"md-lint step %q must not set continue-on-error truthy (got %v) — the gate must hard-fail on violations", step.Name, step.ContinueOnError)
	}

	// Repo-wide: no scoped md_touched change classifier gates the gate.
	changes, ok := wf.Jobs["changes"]
	require.True(t, ok, "ci.yml should define the changes job")
	_, hasMdTouched := changes.Outputs["md_touched"]
	assert.False(t, hasMdTouched,
		"md_touched scoped classifier must NOT exist — the gate is repo-wide (Option-B machinery removed)")

	// setup-node present and pinned to a full 40-char SHA (F013). Match the
	// canonical `actions/setup-node@` prefix (not a substring) so a look-alike such
	// as `other-owner/actions/setup-node@<sha>` cannot satisfy this guard.
	var setupNode ciStep
	foundSetupNode := false
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, "actions/setup-node@") {
			setupNode = step
			foundSetupNode = true
			break
		}
	}
	require.True(t, foundSetupNode, "md-lint job must set up Node via actions/setup-node")
	parts := strings.SplitN(setupNode.Uses, "@", 2)
	require.Len(t, parts, 2, "actions/setup-node must be pinned with @<sha>")
	sha := strings.Fields(parts[1])[0] // tolerate a trailing "# vX.Y.Z" comment
	assert.Regexp(t, "^[0-9a-f]{40}$", sha, "actions/setup-node must be pinned to a full 40-char SHA (F013)")

	// Guard the load-bearing runtime version. markdownlint-cli2@0.23.1 declares
	// engines.node ">=22", so a regression to node-version 20 would reintroduce the
	// unsupported-runtime defect while the SHA-pin check above still passes. Assert
	// the exact node-version the setup step requests.
	require.NotNil(t, setupNode.With, "actions/setup-node step must set a `with` map (node-version)")
	assert.Equal(t, "22", fmt.Sprintf("%v", setupNode.With["node-version"]),
		`actions/setup-node must request node-version "22" (markdownlint-cli2@0.23.1 requires Node >=22)`)

	// The gate runs exactly `make md-lint`. Match a whole trimmed line (not a
	// substring) so a disabled/neutered invocation such as `echo make md-lint`
	// cannot report this characterization test green.
	foundRun := false
	for _, step := range job.Steps {
		for _, line := range strings.Split(step.Run, "\n") {
			if strings.TrimSpace(line) == "make md-lint" {
				foundRun = true
				break
			}
		}
		if foundRun {
			break
		}
	}
	assert.True(t, foundRun, "md-lint job must run `make md-lint` (exact command line, not a substring)")
}

// continueOnErrorHardFails reports whether a `continue-on-error` value keeps a
// job/step hard-failing on error. It is satisfied only when the field is unset
// (nil) or an explicit boolean false. Any true value — or an expression string
// such as `${{ ... }}` that could evaluate true — is rejected, because a
// hard-fail gate must never let a lint violation pass as a non-blocking step.
func continueOnErrorHardFails(v any) bool {
	if v == nil {
		return true
	}
	b, ok := v.(bool)
	return ok && !b
}
