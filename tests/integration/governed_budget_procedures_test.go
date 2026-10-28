package integration_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// procProhibitionShape is the reworded prohibition shape: it forbids any
	// repo-wide test-running form instead of allowlisting a budget-less literal.
	procProhibitionShape = "any repo-wide test-running `go test ... ./...` form"
	procCompileOnlyAllow = "`-run=^$`"
)

var procWhitespace = regexp.MustCompile(`\s+`)

// procRunInstructionLines flags budget-less or zero-timeout full-suite run
// instructions. Compile-only (-run=^$) lines and lines carrying the reworded
// prohibition shape are excluded; a prohibition line that still contains the
// bare budget-less literal is never excluded.
func procRunInstructionLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, procProhibitionShape) && !strings.Contains(line, "`go test ./...`") {
			continue
		}
		kept = append(kept, line)
	}
	return renderUnbudgetedFullSuiteLines(strings.Join(kept, "\n"))
}

func procNormalize(text string) string {
	return procWhitespace.ReplaceAllString(strings.ReplaceAll(text, "\r\n", "\n"), " ")
}

func TestGovernedFullSuiteBudget_Procedures(t *testing.T) {
	cmd := "`" + governedFullSuiteCommand + "`"

	// Non-vacuity self-checks (require): the detector must be live.
	require.Len(t, procRunInstructionLines("* **`wave_scoped: false`**: `go test ./...`, unchanged.\ngo test -timeout=0 ./..."), 2, "run-instruction rows must be flagged")
	require.Empty(t, procRunInstructionLines("* run "+cmd+" now\ngo test -timeout=30m ./..."), "budgeted run rows must pass")
	require.Len(t, procRunInstructionLines("**Do not run `go test ./...` here**"), 1, "the legacy literal prohibition must be flagged")
	require.Empty(t, procRunInstructionLines("**Do not run "+procProhibitionShape+" here** ("+procCompileOnlyAllow+" stays allowed)"), "the reworded prohibition must not trip the detector")
	require.Len(t, procRunInstructionLines("Do not run "+procProhibitionShape+" like `go test ./...`"), 1, "a shaped line that keeps the bare literal must be flagged")
	require.Empty(t, procRunInstructionLines("go test -run=^$ -count=1 ./..."), "compile-only rows must be excluded")

	repoRoot := testRepoRoot(t)
	read := func(rel string) string { return readGovernedSurface(t, repoRoot, rel) }
	buildFeature := read(".github/skills/build-feature/SKILL.md")
	fixCI := read(".github/skills/fix-ci/SKILL.md")
	prAutomation := read(".github/instructions/github-pr-automation.instructions.md")
	goEngineer := read(".github/agents/subagents/go-engineer.agent.md")
	harnessArchitect := read(".github/skills/harness-architect/SKILL.md")

	surfaces := []struct {
		name string
		text string
	}{
		{"build-feature", buildFeature},
		{"fix-ci", fixCI},
		{"github-pr-automation", prAutomation},
		{"go-engineer", goEngineer},
		{"harness-architect", harnessArchitect},
	}
	for _, surface := range surfaces {
		t.Run("no budget-less run: "+surface.name, func(t *testing.T) {
			assert.Empty(t, procRunInstructionLines(surface.text), "%s carries a budget-less or zero-timeout full-suite form", surface.name)
		})
	}

	runAnchors := []struct {
		name   string
		text   string
		phrase string
	}{
		{"build-feature non-wave path", buildFeature, "**`wave_scoped: false` or absent**: " + cmd},
		{"fix-ci configured gates", fixCI, "configured quality gates " + cmd},
		{"fix-ci Test step", fixCI, "Test (" + cmd + ")"},
		{"fix-ci fix order", fixCI, cmd + " → fix failing tests"},
		{"fix-ci code block", fixCI, "\n" + governedFullSuiteCommand + "\n"},
		{"github-pr-automation fix loop", prAutomation, "`golangci-lint run`, " + cmd + ")"},
		{"go-engineer", goEngineer, "Run " + cmd + " — all harness tests must pass"},
	}
	for _, anchor := range runAnchors {
		t.Run("anchor: "+anchor.name, func(t *testing.T) {
			text := strings.ReplaceAll(anchor.text, "\r\n", "\n")
			assert.Contains(t, text, anchor.phrase, "%s must name the governed command", anchor.name)
		})
	}

	prohibitions := []struct {
		name  string
		text  string
		count int
	}{
		{"build-feature", buildFeature, 1},
		{"harness-architect", harnessArchitect, 2},
	}
	for _, p := range prohibitions {
		t.Run("prohibition: "+p.name, func(t *testing.T) {
			normalized := procNormalize(p.text)
			assert.Equal(t, p.count, strings.Count(normalized, procProhibitionShape), "%s must carry the reworded prohibition shape %d time(s)", p.name, p.count)
			assert.Contains(t, normalized, procCompileOnlyAllow, "%s must keep the compile-only check explicitly allowed", p.name)
		})
	}
}
