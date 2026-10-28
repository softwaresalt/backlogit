package integration_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// guidanceFullSuiteInvocation matches any `go test ... ./...` invocation,
	// including the -coverprofile variant.
	guidanceFullSuiteInvocation = regexp.MustCompile("go test\\b[^\\n]*\\./\\.\\.\\.")
	guidanceCompileOnly         = regexp.MustCompile(`-run=\^\$`)
	guidanceZeroTimeout         = regexp.MustCompile(`-timeout[= ]0([smh])?([^0-9.]|$)`)
	guidanceBudgetFlag          = regexp.MustCompile(`-timeout=` + governedFullSuiteBudget + `(\s|$|[^0-9a-zA-Z])`)
)

// guidanceUnbudgetedLines flags every full-suite `go test` form inside an
// anchored normative section that lacks the governed -timeout budget, plus any
// zero-timeout form.
func guidanceUnbudgetedLines(section string) []string {
	section = strings.ReplaceAll(section, "\r\n", "\n")
	var matches []string
	for index, line := range strings.Split(section, "\n") {
		if guidanceCompileOnly.MatchString(line) {
			continue
		}
		if guidanceZeroTimeout.MatchString(line) ||
			(guidanceFullSuiteInvocation.MatchString(line) && !guidanceBudgetFlag.MatchString(line)) {
			matches = append(matches, fmt.Sprintf("L%d: %s", index+1, strings.TrimSpace(line)))
		}
	}
	return matches
}

func TestGovernedFullSuiteBudget_Guidance(t *testing.T) {
	// Non-vacuity self-checks (require): the detector must be live.
	require.Len(t, guidanceUnbudgetedLines("go test ./...\n"), 1, "bare full-suite form must be flagged")
	require.Len(t, guidanceUnbudgetedLines("go test -coverprofile=coverage.out ./...\n"), 1, "coverage variant without a budget must be flagged")
	require.Len(t, guidanceUnbudgetedLines("go test -timeout=0 ./...\n"), 1, "zero-timeout form must be flagged")
	require.Len(t, guidanceUnbudgetedLines("go test -timeout 0 ./...\n"), 1, "separated zero-timeout form must be flagged")
	require.Empty(t, guidanceUnbudgetedLines("go test -timeout=30m ./...\ngo test -timeout=30m -coverprofile=coverage.out ./...\n"), "budgeted forms must pass")
	require.Empty(t, guidanceUnbudgetedLines("go test -run=^$ -count=1 ./...\n"), "compile-only form must be excluded")
	require.Empty(t, guidanceUnbudgetedLines(""), "empty input must pass")
	require.Len(t, guidanceUnbudgetedLines("go test -timeout=300m ./...\n"), 1, "a longer budget must not satisfy the 30m budget")

	repoRoot := testRepoRoot(t)
	command := "`" + governedFullSuiteCommand + "`"
	fenced := governedFullSuiteCommand

	anchors := []struct {
		file    string
		heading string
		phrase  string
	}{
		{".github/instructions/constitution.instructions.md", "### II. Test-First Development (NON-NEGOTIABLE)", command},
		{".github/instructions/constitution.instructions.md", "## Technical Constraints", command},
		{".github/instructions/constitution.instructions.md", "## Quality Gates", fenced},
		{"AGENTS.md", "### II. Test-First Development (NON-NEGOTIABLE)", command},
		{"AGENTS.md", "## Technical Constraints", command},
		{"AGENTS.md", "## Quality Gates", fenced},
		{"README.md", "## Contributing", command},
		{".github/copilot-instructions.md", "## Technology Stack", command},
		// copilot-instructions.md carries a pre-existing doubled code fence in
		// "## Project Structure" that swallows the "## Commands" heading, so the
		// commands block is anchored through the enclosing Project Structure section.
		{".github/copilot-instructions.md", "## Project Structure", fenced},
		{".github/instructions/go.instructions.md", "## Commands", fenced},
		{".github/instructions/go.instructions.md", "## Commands", "go test -timeout=" + governedFullSuiteBudget + " -coverprofile=coverage.out ./..."},
		{".github/copilot-review-instructions.md", "## Tests and verification expectations", command},
	}
	for _, anchor := range anchors {
		t.Run(anchor.file+" "+anchor.heading+" "+anchor.phrase, func(t *testing.T) {
			content := readGovernedSurface(t, repoRoot, anchor.file)
			section := governedSection(content, anchor.heading)
			require.NotEmpty(t, section, "anchor section must be found: %s %s", anchor.file, anchor.heading)
			assert.True(t, sectionHasPhrase(section, anchor.phrase), "governed command missing from %s %s: %s", anchor.file, anchor.heading, anchor.phrase)
			assert.Empty(t, guidanceUnbudgetedLines(section), "unbudgeted full-suite form in %s %s", anchor.file, anchor.heading)
		})
	}
}

// TestU180006_ManifestChecksumsCurrent pins that the four managed files edited
// by E4 carry LF-normalized SHA-256 checksums in the harness manifest.
func TestU180006_ManifestChecksumsCurrent(t *testing.T) {
	repoRoot := testRepoRoot(t)
	manifest := readGovernedSurface(t, repoRoot, ".autoharness/harness-manifest.yaml")
	managed := []string{
		".github/instructions/constitution.instructions.md",
		".github/instructions/go.instructions.md",
		"AGENTS.md",
		".github/copilot-instructions.md",
	}
	for _, rel := range managed {
		t.Run(rel, func(t *testing.T) {
			content := readGovernedSurface(t, repoRoot, rel)
			sum := sha256.Sum256([]byte(content))
			want := hex.EncodeToString(sum[:])

			start := strings.Index(manifest, "- path: \""+rel+"\"")
			require.GreaterOrEqual(t, start, 0, "manifest entry must exist: %s", rel)
			require.Equal(t, 1, strings.Count(manifest, "- path: \""+rel+"\""), "manifest entry must be unique: %s", rel)
			entry := manifest[start+1:]
			if next := strings.Index(entry, "\n  - path:"); next >= 0 {
				entry = entry[:next]
			}
			match := regexp.MustCompile(`checksum: "([0-9a-f]{64})"`).FindStringSubmatch(entry)
			require.Len(t, match, 2, "manifest entry must carry a checksum: %s", rel)
			assert.Equal(t, want, match[1], "manifest checksum is stale for %s", rel)
		})
	}
}
