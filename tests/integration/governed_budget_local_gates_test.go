package integration_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	localPrePushTestGate  = regexp.MustCompile(`^\s*(?:Invoke-Gate|run_gate)\s+"Test"\s`)
	localPrePushBuildGate = regexp.MustCompile(`^\s*(?:Invoke-Gate|run_gate)\s+"Build"\s`)
	localMakeTarget       = regexp.MustCompile(`^([A-Za-z0-9_.-]+)\s*:`)

	// Script lines wrap the command in quotes, so the shared workflow regexes
	// (which require whitespace around the command) are adapted here.
	localFullSuiteInvocation = regexp.MustCompile(`(^|[\s;&|"'])go test\b[^\n]*\./\.\.\.`)
	localBudgetFlag          = regexp.MustCompile(`(^|\s)-timeout=` + governedFullSuiteBudget + `(\s|"|'|$)`)
)

// localUnbudgetedFullSuiteLines flags `go test ... ./...` lines that lack the
// governed -timeout=30m budget or carry a zero timeout. Compile-only
// (-run=^$) invocations are excluded.
func localUnbudgetedFullSuiteLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var matches []string
	for index, line := range strings.Split(text, "\n") {
		if !localFullSuiteInvocation.MatchString(line) || workflowCompileOnlyRun.MatchString(line) {
			continue
		}
		if workflowZeroTimeout.MatchString(line) || !localBudgetFlag.MatchString(line) {
			matches = append(matches, fmt.Sprintf("L%d: %s", index+1, strings.TrimSpace(line)))
		}
	}
	return matches
}

// localGateAnchoredLines returns the lines selected by an anchor, with the
// original (1-based) line number prefix stripped; the lines are joined with
// newlines so they can be fed to localUnbudgetedFullSuiteLines.
func localGateAnchoredLines(content string, anchor *regexp.Regexp) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		if anchor.MatchString(line) {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

// localMakeRecipeLines returns the tab-indented recipe lines of one Makefile target.
func localMakeRecipeLines(content, target string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	var lines []string
	inTarget := false
	for _, line := range strings.Split(content, "\n") {
		if m := localMakeTarget.FindStringSubmatch(line); m != nil {
			inTarget = m[1] == target
			continue
		}
		if inTarget && strings.HasPrefix(line, "\t") {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	return lines
}

func TestGovernedFullSuiteBudget_LocalGates(t *testing.T) {
	// Non-vacuity self-checks (require): the detector and anchors must be live.
	detectorInput := strings.Join([]string{
		`Invoke-Gate "Test" "go" "go test ./..."`,
		`run_gate "Test" "go" "go test -timeout=0 ./..."`,
		`run_gate "Test" "go" "go test -timeout 0 ./..."`,
		`Invoke-Gate "Build" "go" "go test -run=^$ -count=1 ./..."`,
		`Invoke-Gate "Test" "go" "go test -timeout=30m ./..."`,
		"\tgo test -race -coverprofile=coverage.out ./...",
		"\tgo test -race -timeout=30m -coverprofile=coverage.out ./...",
	}, "\n")
	flagged := localUnbudgetedFullSuiteLines(detectorInput)
	require.Len(t, flagged, 4, "detector must flag plain, zero-equals, zero-separated, and race-without-budget lines only")
	require.Empty(t, localUnbudgetedFullSuiteLines(`Invoke-Gate "Build" "go" "go test -run=^$ -count=1 ./..."`), "Build gate compile-only form must not be flagged")
	require.Empty(t, localUnbudgetedFullSuiteLines(`run_gate "Test" "go" "go test -timeout=30m ./..."`))
	require.Len(t, localUnbudgetedFullSuiteLines(`run_gate "Test" "go" "go test -timeout=300m ./..."`), 1)
	require.Len(t, localGateAnchoredLines(detectorInput, localPrePushTestGate), 4)
	require.Len(t, localGateAnchoredLines(detectorInput, localPrePushBuildGate), 1)
	require.Equal(t, []string{
		"go test -race -coverprofile=coverage.out ./...",
		"go test -race -timeout=30m -coverprofile=coverage.out ./...",
	}, localMakeRecipeLines("build:\n\tgo build\n\ntest:\n\tgo test -race -coverprofile=coverage.out ./...\n\tgo test -race -timeout=30m -coverprofile=coverage.out ./...\n\nlint:\n\tx\n", "test"))

	repoRoot := testRepoRoot(t)
	const buildGate = "go test -run=^$ -count=1 ./..."

	for _, script := range []struct {
		path string
		test string
	}{
		{path: "scripts/pre-push-quality-gates.ps1", test: `Invoke-Gate "Test" "go" "` + governedFullSuiteCommand + `"`},
		{path: "scripts/pre-push-quality-gates.sh", test: `run_gate "Test" "go" "` + governedFullSuiteCommand + `"`},
	} {
		t.Run(script.path, func(t *testing.T) {
			content := readGovernedSurface(t, repoRoot, script.path)
			testGate := localGateAnchoredLines(content, localPrePushTestGate)
			require.Len(t, testGate, 1, "%s must define exactly one Test gate", script.path)
			assert.Empty(t, localUnbudgetedFullSuiteLines(strings.Join(testGate, "\n")), "%s Test gate lacks the governed budget or carries a zero timeout", script.path)
			assert.Equal(t, script.test, testGate[0], "%s Test gate must run the governed budget command", script.path)

			buildGates := localGateAnchoredLines(content, localPrePushBuildGate)
			require.Len(t, buildGates, 1, "%s must define exactly one Build gate", script.path)
			assert.Contains(t, buildGates[0], buildGate, "%s Build gate compile-only check must stay unchanged", script.path)
			assert.NotContains(t, buildGates[0], "-timeout", "%s Build gate must not gain a timeout", script.path)
		})
	}

	t.Run("Makefile", func(t *testing.T) {
		content := readGovernedSurface(t, repoRoot, "Makefile")
		recipe := localMakeRecipeLines(content, "test")
		require.Len(t, recipe, 1, "Makefile test target must have exactly one recipe line")
		assert.Empty(t, localUnbudgetedFullSuiteLines(strings.Join(recipe, "\n")), "Makefile test recipe lacks the governed budget or carries a zero timeout")
		assert.Equal(t, "go test -race -timeout="+governedFullSuiteBudget+" -coverprofile=coverage.out ./...", recipe[0], "Makefile test recipe must run the governed budget command")
	})
}
