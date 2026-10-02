package integration_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUCS1_ClaimStartContract(t *testing.T) {
	repoRoot := testRepoRoot(t)
	readSource := func(relativePath string) string {
		t.Helper()

		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relativePath)))
		require.NoError(t, err, "read contract source %s", relativePath)
		return string(content)
	}
	normalizeWhitespace := func(text string) string {
		return strings.Join(strings.Fields(text), " ")
	}
	sliceSection := func(text, startAnchor, endAnchor string) (string, bool) {
		startPattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(startAnchor) + `[^\r\n]*`)
		endPattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(endAnchor) + `[^\r\n]*`)
		starts := startPattern.FindAllStringIndex(text, -1)
		ends := endPattern.FindAllStringIndex(text, -1)
		if len(starts) != 1 || len(ends) != 1 || ends[0][0] <= starts[0][0] {
			return "", false
		}
		return text[starts[0][0]:ends[0][0]], true
	}
	policyRows := func(policyTaxonomy, code string) []string {
		rows := make([]string, 0, 1)
		rowPrefix := "| `" + code + "` |"
		for _, line := range strings.Split(policyTaxonomy, "\n") {
			if strings.HasPrefix(line, rowPrefix) {
				rows = append(rows, line)
			}
		}
		return rows
	}
	assertRemovedText := func(t *testing.T, section, sectionName, text string) {
		t.Helper()

		normalizedSection := normalizeWhitespace(section)
		normalizedText := normalizeWhitespace(text)
		if strings.Contains(normalizedSection, normalizedText) {
			t.Logf("PRE-REPAIR PRESENT: %s contains removed literal %q", sectionName, text)
		} else {
			t.Logf("PRE-REPAIR ABSENT: %s does not contain removed literal %q", sectionName, text)
		}
		assert.NotContains(t, normalizedSection, normalizedText,
			"%s must not retain removed sentence %q", sectionName, text)
	}

	ship := readSource(".github/agents/_ship.agent.md")
	policy := readSource(".github/policies/workflow-policies.md")
	shipmentLifecycle := readSource("internal/core/shipment_lifecycle.go")
	commits := readSource("internal/core/commits.go")

	claimStartTokens := []string{
		"claim-assigned",
		"scheduler_baseline_claim",
		"custom_fields.items",
		"WORK_STARTED:",
		"start epoch",
		"shipment claimed",
		"logs/<id>.jsonl",
		"manifest drift",
		"stale read",
		"WAVE_CLAIM_STATE_INDETERMINATE",
		"active residual",
		"queued or claim-assigned",
		"fail closed",
	}

	t.Run("Preserved", func(t *testing.T) {
		step40, ok := sliceSection(ship, "#### Step 4.0: Wave Admission (P-002.6)", "#### Step 4.1a:")
		require.True(t, ok, "Ship Step 4.0 section must have unique anchored delimiters")
		step40Normalized := normalizeWhitespace(step40)
		for _, token := range []string{
			"WAVE_MEMBER_BLOCKED",
			"WAVE_STATUS_UNSUPPORTED",
			"WAVE_CYCLE_DETECTED",
			"WAVE_BUDGET_EXCEEDED",
			"terminal_success = M",
		} {
			require.Contains(t, step40Normalized, token, "Ship Step 4.0 must retain %q", token)
		}

		claimHeading := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta("#### Step 4.1b: Claim Task") + `\r?$`)
		require.Len(t, claimHeading.FindAllString(ship, -1), 1, "Ship must have exactly one Claim Task heading")

		policyWave, ok := sliceSection(policy,
			"### P-002.6 — Dependency-Aware Harness Waves (Scheduling Contract)", "### P-013.1")
		require.True(t, ok, "P-002.6 section must have unique anchored delimiters")
		require.Contains(t, normalizeWhitespace(policyWave), `"No ready tasks" is never a completion condition`)

		lifecycleNormalized := normalizeWhitespace(shipmentLifecycle)
		require.Contains(t, lifecycleNormalized, `models.StatusActive, "shipment claimed", shipmentID`)

		commitsNormalized := normalizeWhitespace(commits)
		require.Contains(t, commitsNormalized, `EventType: "comment",`)
		require.Contains(t, commitsNormalized, `Delta: map[string]any{"comment": comment},`)

		policyTaxonomy, ok := sliceSection(policy,
			"### P-002.2 — Harness-Exempt Halt Taxonomy and Reporting", "### P-002.3")
		require.True(t, ok, "P-002.2 section must have unique anchored delimiters")
		optionalPolicyRows := []struct {
			code    string
			content string
		}{
			{code: "WAVE_CLAIM_STATE_INDETERMINATE", content: "wave admission only"},
			{code: "TASK_START_NOT_RECORDED", content: "Step 4.1b"},
			{code: "WAVE_NO_PROGRESS", content: "wave admission only"},
		}
		// These future contract rows may be absent; count once to reject duplicates, and
		// check content only when exactly one row exists.
		for _, row := range optionalPolicyRows {
			rows := policyRows(policyTaxonomy, row.code)
			require.LessOrEqual(t, len(rows), 1, "P-002.2 must not duplicate the %s row", row.code)
			if len(rows) == 1 {
				require.Contains(t, normalizeWhitespace(rows[0]), row.content,
					"P-002.2 %s row must retain %q", row.code, row.content)
			}
		}
	})

	t.Run("Ship", func(t *testing.T) {
		step40, ok := sliceSection(ship, "#### Step 4.0: Wave Admission (P-002.6)", "#### Step 4.1a:")
		if !assert.True(t, ok, "Ship Step 4.0 section must have unique anchored delimiters") {
			return
		}
		step2, ok := sliceSection(ship, "### Step 2: Harness Generation", "#### Step 2a:")
		if !assert.True(t, ok, "Ship Step 2 section must have unique anchored delimiters") {
			return
		}
		step41a, ok := sliceSection(ship, "#### Step 4.1a:", "#### Step 4.1b:")
		if !assert.True(t, ok, "Ship Step 4.1a section must have unique anchored delimiters") {
			return
		}
		step41b, ok := sliceSection(ship, "#### Step 4.1b: Claim Task", "#### Step 4.1c:")
		if !assert.True(t, ok, "Ship Step 4.1b section must have unique anchored delimiters") {
			return
		}
		step46, ok := sliceSection(ship, "#### Step 4.6: Wave Convergence Gate (P-002.6)", "### Step 5:")
		if !assert.True(t, ok, "Ship Step 4.6 section must have unique anchored delimiters") {
			return
		}

		step40Normalized := normalizeWhitespace(step40)
		for _, token := range claimStartTokens {
			assert.Contains(t, step40Normalized, normalizeWhitespace(token),
				"Ship Step 4.0 must contain %q", token)
		}
		assert.GreaterOrEqual(t, strings.Count(step40Normalized, "WAVE_CLAIM_STATE_INDETERMINATE"), 3,
			"Ship Step 4.0 must name WAVE_CLAIM_STATE_INDETERMINATE in classification, item 7, and replay")

		item7, ok := sliceSection(step40, "7. **Report deterministically on halt.", "8.")
		if assert.True(t, ok, "Ship Step 4.0 item 7 must have unique anchored delimiters") {
			assert.Contains(t, normalizeWhitespace(item7), "WAVE_CLAIM_STATE_INDETERMINATE",
				"Ship Step 4.0 item 7 must report WAVE_CLAIM_STATE_INDETERMINATE")
		}

		assert.Contains(t, normalizeWhitespace(step2), "queued or claim-assigned")
		assert.Contains(t, normalizeWhitespace(step41a), "claim-assigned")
		for _, phrase := range []string{
			"WORK_STARTED:",
			"backlogit_append_comment",
			"backlogit comment add",
			"--actor ship",
			"TASK_START_NOT_RECORDED",
			"P-005",
			"A claim-assigned task is never moved again",
			"only when no valid start record exists",
			"re-reads the item log before any CLI fallback",
			"exactly one valid start record",
			"before any dispatch",
		} {
			assert.Contains(t, normalizeWhitespace(step41b), phrase,
				"Ship Step 4.1b must contain %q", phrase)
		}

		for _, removed := range []struct {
			section string
			text    string
		}{
			{section: "Step 4.0", text: "ready_k = { t in queued : every dependency of t is terminal_success }"},
			{section: "Step 4.0", text: "If any member is still `active` at wave admission, it is an unfinished claim from a prior wave, not progress"},
			{section: "Step 2", text: "the queued tasks of the target feature or chore whose dependencies are"},
			{section: "Step 4.1b", text: "Update task status to `active` using the backlog tool's move operation."},
		} {
			var section string
			var sectionName string
			switch removed.section {
			case "Step 4.0":
				section = step40Normalized
				sectionName = "Ship Step 4.0"
			case "Step 2":
				section = normalizeWhitespace(step2)
				sectionName = "Ship Step 2"
			case "Step 4.1b":
				section = normalizeWhitespace(step41b)
				sectionName = "Ship Step 4.1b"
			}
			assertRemovedText(t, section, sectionName, removed.text)
		}

		step46Normalized := normalizeWhitespace(step46)
		assert.Contains(t, step46Normalized, "no member of `ready_k` is `active`")
		assert.Contains(t, step46Normalized, "A member of `ready_k` still `active`")
		assertRemovedText(t, step46Normalized, "Ship Step 4.6",
			"and no member is `active`, `blocked`, or in an unsupported status")
	})

	t.Run("Policy", func(t *testing.T) {
		policyWave, ok := sliceSection(policy,
			"### P-002.6 — Dependency-Aware Harness Waves (Scheduling Contract)", "### P-013.1")
		if !assert.True(t, ok, "P-002.6 section must have unique anchored delimiters") {
			return
		}
		policyWaveNormalized := normalizeWhitespace(policyWave)
		for _, token := range claimStartTokens {
			assert.Contains(t, policyWaveNormalized, normalizeWhitespace(token),
				"P-002.6 must contain %q", token)
		}
		assert.Contains(t, policyWaveNormalized, "none of them satisfies a dependency",
			"P-002.6 must retain the rule that active members do not satisfy dependencies")

		for _, removed := range []string{
			"ready_k = { t ∈ queued : deps(t) ⊆ terminal_success }",
			"If `active` is non-empty → **halt** with `WAVE_NO_PROGRESS`",
			"it never treats an `active` member as satisfied",
			"A member still carrying `active` at wave admission is a claim from a prior wave that never reached `done`",
			"The scheduler never admits a new wave over an unfinished claim",
			"the queued members whose every dependency has reached a terminal-success status",
		} {
			assertRemovedText(t, policyWaveNormalized, "Policy P-002.6", removed)
		}

		policyTaxonomy, ok := sliceSection(policy,
			"### P-002.2 — Harness-Exempt Halt Taxonomy and Reporting", "### P-002.3")
		if !assert.True(t, ok, "P-002.2 section must have unique anchored delimiters") {
			return
		}
		indeterminateRows := policyRows(policyTaxonomy, "WAVE_CLAIM_STATE_INDETERMINATE")
		assert.Len(t, indeterminateRows, 1, "P-002.2 must have exactly one WAVE_CLAIM_STATE_INDETERMINATE row")
		if len(indeterminateRows) == 1 {
			assert.Contains(t, normalizeWhitespace(indeterminateRows[0]), "wave admission only")
		}

		startNotRecordedRows := policyRows(policyTaxonomy, "TASK_START_NOT_RECORDED")
		assert.Len(t, startNotRecordedRows, 1, "P-002.2 must have exactly one TASK_START_NOT_RECORDED row")
		if len(startNotRecordedRows) == 1 {
			assert.Contains(t, normalizeWhitespace(startNotRecordedRows[0]), "Step 4.1b")
		}

		noProgressRows := policyRows(policyTaxonomy, "WAVE_NO_PROGRESS")
		assert.Len(t, noProgressRows, 1, "P-002.2 must have exactly one WAVE_NO_PROGRESS row")
		if len(noProgressRows) == 1 {
			noProgressRow := normalizeWhitespace(noProgressRows[0])
			assert.Contains(t, noProgressRow, "no `queued` or claim-assigned member has all dependencies terminal")
			assert.Contains(t, noProgressRow, "claim-assigned")
			assertRemovedText(t, noProgressRow, "P-002.2 WAVE_NO_PROGRESS",
				"no `queued` member has all dependencies terminal")
			assertRemovedText(t, noProgressRow, "P-002.2 WAVE_NO_PROGRESS",
				"**or** a member is still `active` at wave admission")
		}
	})
}
