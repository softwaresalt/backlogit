package integration_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	governedFullSuiteBudget  = "30m"
	governedFullSuiteCommand = "go test -timeout=" + governedFullSuiteBudget + " ./..."
)

func governedSection(content, heading string) string {
	lines := strings.Split(content, "\n")
	var fenceCharacter byte
	fenceLength := 0

	insideFence := func(line string) bool {
		trimmed := strings.TrimLeft(line, " \t")
		if fenceCharacter != 0 {
			if len(trimmed) > 0 && trimmed[0] == fenceCharacter {
				markerLength := 0
				for markerLength < len(trimmed) && trimmed[markerLength] == fenceCharacter {
					markerLength++
				}
				if markerLength >= fenceLength && strings.TrimSpace(trimmed[markerLength:]) == "" {
					fenceCharacter = 0
					fenceLength = 0
				}
			}
			return true
		}

		indent := len(line) - len(trimmed)
		if indent > 3 || len(trimmed) == 0 {
			return false
		}
		character := trimmed[0]
		if character != '`' && character != '~' {
			return false
		}
		markerLength := 0
		for markerLength < len(trimmed) && trimmed[markerLength] == character {
			markerLength++
		}
		if markerLength < 3 {
			return false
		}
		fenceCharacter = character
		fenceLength = markerLength
		return true
	}

	headingLevel := func(line string) int {
		trimmed := strings.TrimLeft(line, " ")
		if len(line)-len(trimmed) > 3 {
			return 0
		}
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level == 0 || level > 6 || (level < len(trimmed) && trimmed[level] != ' ' && trimmed[level] != '\t') {
			return 0
		}
		return level
	}

	anchorIndex := -1
	for index, line := range lines {
		if insideFence(line) {
			continue
		}
		if line == heading {
			anchorIndex = index
			break
		}
	}
	anchorLevel := headingLevel(heading)
	if anchorIndex < 0 || anchorLevel == 0 {
		return ""
	}

	fenceCharacter = 0
	fenceLength = 0
	for index := anchorIndex + 1; index < len(lines); index++ {
		if insideFence(lines[index]) {
			continue
		}
		level := headingLevel(lines[index])
		if level > 0 && level <= anchorLevel {
			return strings.Join(lines[anchorIndex:index], "\n")
		}
	}
	return strings.Join(lines[anchorIndex:], "\n")
}

func sectionHasPhrase(section, phrase string) bool {
	return strings.Contains(normalizeDocWhitespace(section), normalizeDocWhitespace(phrase))
}

func budgetlessFullSuiteLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	amendmentRow := regexp.MustCompile(`^\|\s*\d+\.\d+\.\d+\s*\|`)
	zeroTimeout := regexp.MustCompile(`-timeout[= ]0([smh])?([^0-9.]|$)`)
	matches := make([]string, 0, len(lines))
	for index, line := range lines {
		if amendmentRow.MatchString(line) {
			continue
		}
		if strings.Contains(line, "go test ./...") || zeroTimeout.MatchString(line) {
			matches = append(matches, fmt.Sprintf("L%d: %s", index+1, line))
		}
	}
	return matches
}

func TestGovernedFullSuiteBudget_ShipFinalGate(t *testing.T) {
	detectorInput := strings.Join([]string{
		"budget-less: go test ./...",
		"zero-equals: go test -timeout=0 ./...",
		"zero-separated: go test -timeout 0 ./...",
		"canonical: " + governedFullSuiteCommand,
		"| 1.2.3 | amendment: go test ./... -timeout=0 |",
	}, "\n")
	require.Equal(t, []string{
		"L1: budget-less: go test ./...",
		"L2: zero-equals: go test -timeout=0 ./...",
		"L3: zero-separated: go test -timeout 0 ./...",
	}, budgetlessFullSuiteLines(detectorInput))
	require.Equal(t, budgetlessFullSuiteLines(detectorInput), budgetlessFullSuiteLines(strings.ReplaceAll(detectorInput, "\n", "\r\n")))
	require.Empty(t, budgetlessFullSuiteLines("canonical: "+governedFullSuiteCommand))
	require.False(t, sectionHasPhrase("`go test -timeout=300m ./...`", "`"+governedFullSuiteCommand+"`"))

	fencedSection := strings.Join([]string{
		"## Anchor",
		"before",
		"````",
		"# fenced heading",
		"```",
		"## still fenced by the shorter marker",
		"~~~~",
		"## still fenced by the different marker",
		"`````",
		"### lower heading",
		"after lower heading",
		"## Same-level boundary",
		"outside",
	}, "\n")
	require.Equal(t, strings.Join([]string{
		"## Anchor",
		"before",
		"````",
		"# fenced heading",
		"```",
		"## still fenced by the shorter marker",
		"~~~~",
		"## still fenced by the different marker",
		"`````",
		"### lower heading",
		"after lower heading",
	}, "\n"), governedSection(fencedSection, "## Anchor"))
	require.Empty(t, governedSection("```\n## Hidden anchor\n```\n", "## Hidden anchor"))
	require.Empty(t, governedSection(fencedSection, "## Missing anchor"))
	require.Equal(t, "### Anchor\nbody", governedSection("### Anchor\nbody\n# Higher-level boundary\noutside", "### Anchor"))

	repoRoot := testRepoRoot(t)
	t.Run("ship-agent", func(t *testing.T) {
		content := readGovernedSurface(t, repoRoot, ".github/agents/_ship.agent.md")
		cases := []struct {
			heading string
			phrase  string
		}{
			{
				heading: "#### Step 4.6: Wave Convergence Gate (P-002.6)",
				phrase:  "run the **unfiltered full repository suite**: `" + governedFullSuiteCommand + "`",
			},
			{
				heading: "### Step 5: PR Lifecycle",
				phrase:  "**unfiltered** `" + governedFullSuiteCommand + "` with no tolerated red of any kind",
			},
		}
		for _, check := range cases {
			section := governedSection(content, check.heading)
			assert.Contains(t, content, check.heading, "anchor heading must be present: %s", check.heading)
			assert.NotEmpty(t, section, "anchor section must be non-empty: %s", check.heading)
			assert.True(t, sectionHasPhrase(section, check.phrase), "required phrase missing from %s: %s", check.heading, check.phrase)
		}
		assert.Empty(t, budgetlessFullSuiteLines(content), "Ship surface contains unbudgeted full-suite forms")
	})

	t.Run("workflow-policies", func(t *testing.T) {
		content := readGovernedSurface(t, repoRoot, ".github/policies/workflow-policies.md")
		cases := []struct {
			heading string
			phrase  string
		}{
			{
				heading: "### P-002.6 — Dependency-Aware Harness Waves (Scheduling Contract)",
				phrase:  "(`" + governedFullSuiteCommand + "`, no selector, no `-short`, no skip)",
			},
			{
				heading: "### P-002.6 — Dependency-Aware Harness Waves (Scheduling Contract)",
				phrase:  "run `" + governedFullSuiteCommand + "` and admit only failures",
			},
			{
				heading: "### P-002.6 — Dependency-Aware Harness Waves (Scheduling Contract)",
				phrase:  "**Governed full-suite budget (073-DL).** Every governed unfiltered full-suite run in P-002.6, in P-004, and in Ship Steps 4.6 and 5, including the full-suite runs Ship delegates during Step 5 (fix-ci and the github-pr-automation fix loop), is `" + governedFullSuiteCommand + "`, run from the repository root.",
			},
			{
				heading: "## P-004: Red Phase Before Implementation",
				phrase:  "AND `" + governedFullSuiteCommand + "` exits non-zero with expected failure markers",
			},
		}
		for _, check := range cases {
			section := governedSection(content, check.heading)
			assert.Contains(t, content, check.heading, "anchor heading must be present: %s", check.heading)
			assert.NotEmpty(t, section, "anchor section must be non-empty: %s", check.heading)
			assert.True(t, sectionHasPhrase(section, check.phrase), "required phrase missing from %s: %s", check.heading, check.phrase)
		}
		assert.Empty(t, budgetlessFullSuiteLines(content), "workflow policy surface contains unbudgeted full-suite forms")
	})
}
