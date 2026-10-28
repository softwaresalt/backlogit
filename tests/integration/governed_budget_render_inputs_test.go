package integration_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

var (
	renderFullSuiteInvocation = regexp.MustCompile("(^|[\\s;&|`\"'])go test\\b[^\\n]*\\./\\.\\.\\.")
	renderBudgetFlag          = regexp.MustCompile(`(^|\s)-timeout=` + governedFullSuiteBudget + `(\s|` + "`" + `|"|'|$)`)
)

// renderUnbudgetedFullSuiteLines flags `go test ... ./...` occurrences that lack
// the governed -timeout=30m budget or carry a zero timeout. Compile-only
// (-run=^$) forms and other gates (go vet, lint, fmt) are not flagged.
func renderUnbudgetedFullSuiteLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var matches []string
	for index, line := range strings.Split(text, "\n") {
		if !renderFullSuiteInvocation.MatchString(line) || workflowCompileOnlyRun.MatchString(line) {
			continue
		}
		if workflowZeroTimeout.MatchString(line) || !renderBudgetFlag.MatchString(line) {
			matches = append(matches, fmt.Sprintf("L%d: %s", index+1, strings.TrimSpace(line)))
		}
	}
	return matches
}

// renderYAMLFind returns the value of the first mapping key named key, searching
// depth-first, and whether it was found.
func renderYAMLFind(node *yaml.Node, key string) (*yaml.Node, bool) {
	if node == nil {
		return nil, false
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if found, ok := renderYAMLFind(child, key); ok {
				return found, true
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				return node.Content[i+1], true
			}
		}
		for i := 0; i+1 < len(node.Content); i += 2 {
			if found, ok := renderYAMLFind(node.Content[i+1], key); ok {
				return found, true
			}
		}
	}
	return nil, false
}

func renderYAMLPath(t *testing.T, content string, path ...string) string {
	t.Helper()
	var root yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(content), &root))
	node := &root
	for _, key := range path {
		next, ok := renderYAMLFind(node, key)
		require.True(t, ok, "anchor key %q missing in path %v", key, path)
		node = next
	}
	if node.Kind == yaml.SequenceNode {
		values := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			values = append(values, item.Value)
		}
		return strings.Join(values, "\n")
	}
	return node.Value
}

func TestGovernedFullSuiteBudget_RenderInputs(t *testing.T) {
	// Non-vacuity self-checks (require): the detector must be live.
	detectorInput := strings.Join([]string{
		"go test ./...",
		"go test -timeout=0 ./...",
		"go test -timeout 0 ./...",
		"* the `go test ./...` suite run",
		"go test -timeout=30m ./...",
		"* the `go test -timeout=30m ./...` suite run",
		"go test -run=^$ -count=1 ./...",
		"go vet ./...",
		"golangci-lint run",
		"gofmt -l .",
	}, "\n")
	require.Len(t, renderUnbudgetedFullSuiteLines(detectorInput), 4, "detector must flag plain, zero-equals, zero-separated, and backticked budget-less forms only")
	require.Empty(t, renderUnbudgetedFullSuiteLines("go vet ./...\ngolangci-lint run\ngofmt -l ."))
	require.Equal(t, "a b", renderYAMLPath(t, "x:\n  y: a b\n", "x", "y"))
	require.Equal(t, "p\nq", renderYAMLPath(t, "x:\n  y:\n    - p\n    - q\n", "x", "y"))

	repoRoot := testRepoRoot(t)
	manifest := readGovernedSurface(t, repoRoot, ".autoharness/harness-manifest.yaml")
	profile := readGovernedSurface(t, repoRoot, ".autoharness/workspace-profile.yaml")
	review := readGovernedSurface(t, repoRoot, ".github/instructions/copilot-code-review.instructions.md")

	anchors := []struct {
		name  string
		value string
		want  string
	}{
		{name: "manifest TEST_COMMAND", value: renderYAMLPath(t, manifest, "TEST_COMMAND"), want: governedFullSuiteCommand},
		{name: "manifest HARNESS_ENFORCED_SUMMARY", value: renderYAMLPath(t, manifest, "HARNESS_ENFORCED_SUMMARY"), want: "`" + governedFullSuiteCommand + "` suite run"},
		{name: "manifest TESTING_RULES", value: renderYAMLPath(t, manifest, "TESTING_RULES"), want: "`" + governedFullSuiteCommand + "`"},
		{name: "profile test.command", value: renderYAMLPath(t, profile, "test", "command"), want: governedFullSuiteCommand},
		{name: "profile ci.quality_gates", value: renderYAMLPath(t, profile, "ci", "quality_gates"), want: governedFullSuiteCommand},
		{name: "copilot-code-review instruction", value: review, want: "`" + governedFullSuiteCommand + "` suite run"},
	}
	require.Len(t, anchors, 6)

	for _, anchor := range anchors {
		t.Run(anchor.name, func(t *testing.T) {
			assert.Empty(t, renderUnbudgetedFullSuiteLines(anchor.value), "%s carries a budget-less or zero-timeout full-suite form", anchor.name)
			assert.Contains(t, anchor.value, anchor.want, "%s must name the governed budget command", anchor.name)
		})
	}

	// The profile runner stays plain `go test`.
	assert.Equal(t, "go test", renderYAMLPath(t, profile, "test", "runner"))
}
