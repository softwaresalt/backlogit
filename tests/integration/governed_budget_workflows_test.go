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
	workflowFullSuiteInvocation = regexp.MustCompile(`(^|[\s;&|])go test\b[^\n]*\./\.\.\.`)
	workflowCompileOnlyRun      = regexp.MustCompile(`-run[= ]\^\$`)
	workflowBudgetFlag          = regexp.MustCompile(`(^|\s)-timeout=` + governedFullSuiteBudget + `(\s|$)`)
	workflowZeroTimeout         = regexp.MustCompile(`-timeout[= ]0([smh])?([^0-9.]|$)`)
)

// workflowUnbudgetedFullSuiteLines returns every `go test ... ./...` line in a
// workflow step's run script that lacks the governed -timeout=30m budget or
// carries a zero-timeout form. Compile-only (-run=^$) invocations are excluded.
func workflowUnbudgetedFullSuiteLines(run string) []string {
	run = strings.ReplaceAll(run, "\r\n", "\n")
	var matches []string
	for index, line := range strings.Split(run, "\n") {
		if !workflowFullSuiteInvocation.MatchString(line) || workflowCompileOnlyRun.MatchString(line) {
			continue
		}
		if workflowZeroTimeout.MatchString(line) || !workflowBudgetFlag.MatchString(line) {
			matches = append(matches, fmt.Sprintf("L%d: %s", index+1, strings.TrimSpace(line)))
		}
	}
	return matches
}

func TestGovernedFullSuiteBudget_Workflows(t *testing.T) {
	// Non-vacuity self-check: the detector must flag and clear the right shapes.
	detectorInput := strings.Join([]string{
		"plain: go test ./...",
		"race: go test -race -coverprofile=coverage.out ./...",
		"zero-equals: go test -timeout=0 ./...",
		"zero-separated: go test -timeout 0 ./...",
		"compile-only: go test -run=^$ -count=1 ./...",
		"scoped: go test ./tests/integration/ -run '^X$' -count=1",
		"race-budgeted: go test -race -timeout=30m -coverprofile=coverage.out ./...",
		"budgeted: go test -timeout=30m ./...",
	}, "\n")
	expected := []string{
		"L1: plain: go test ./...",
		"L2: race: go test -race -coverprofile=coverage.out ./...",
		"L3: zero-equals: go test -timeout=0 ./...",
		"L4: zero-separated: go test -timeout 0 ./...",
	}
	require.Equal(t, expected, workflowUnbudgetedFullSuiteLines(detectorInput))
	require.Equal(t, expected, workflowUnbudgetedFullSuiteLines(strings.ReplaceAll(detectorInput, "\n", "\r\n")))
	require.Empty(t, workflowUnbudgetedFullSuiteLines("go test -timeout=30m ./..."))
	require.Len(t, workflowUnbudgetedFullSuiteLines("go test -timeout=300m ./..."), 1)

	ciPath, releasePath, _ := workflowPaths(t)
	surfaces := []struct {
		name    string
		path    string
		command string
	}{
		{name: ".github/workflows/ci.yml", path: ciPath, command: "go test -race -timeout=" + governedFullSuiteBudget + " -coverprofile=coverage.out ./..."},
		{name: ".github/workflows/release.yml", path: releasePath, command: "go test -timeout=" + governedFullSuiteBudget + " ./..."},
	}
	for _, surface := range surfaces {
		t.Run(surface.name, func(t *testing.T) {
			wf := readCIWorkflow(t, surface.path)
			testJob, ok := wf.Jobs["test"]
			require.True(t, ok, "%s must define a test job", surface.name)
			step := findStep(t, testJob, "Test")
			assert.Empty(t, workflowUnbudgetedFullSuiteLines(step.Run), "%s Test step contains an unbudgeted or zero-timeout full-suite form", surface.name)
			assert.Equal(t, surface.command, strings.TrimSpace(step.Run), "%s Test step must run the governed budget command", surface.name)
		})
	}
}
