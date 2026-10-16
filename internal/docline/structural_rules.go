package docline

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/softwaresalt/backlogit/internal/core"
)

// Structural rule identifiers reported in Finding.Rule.
const (
	// RuleMissingFrontmatter flags a document without a frontmatter block.
	RuleMissingFrontmatter = "structural_missing_frontmatter"
	// RuleTableShape flags a table row whose column count differs from its header.
	RuleTableShape = "structural_table_shape"
	// RuleMultipleH1 flags a document with more than one level-1 heading.
	RuleMultipleH1 = "structural_multiple_h1"
	// RuleHeadingSkippedLevel flags a heading that skips a level (for example H2 to H4).
	RuleHeadingSkippedLevel = "structural_heading_skip"
	// RuleNumericClaim flags a docline:count claim that disagrees with the workspace.
	RuleNumericClaim = "structural_numeric_claim"
	// RuleTopologyClaim flags a docline:topology claim that disagrees with the workspace.
	RuleTopologyClaim = "structural_topology_claim"
)

// mdLine is one body line with its 1-based number and fenced-code state.
type mdLine struct {
	n      int
	text   string
	inCode bool // inside (or delimiting) a fenced code block
}

// splitLines splits body into lines and marks fenced code blocks so rules can
// ignore illustrative content.
func splitLines(body string) []mdLine {
	raw := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	out := make([]mdLine, 0, len(raw))
	var fence string
	for i, l := range raw {
		t := strings.TrimSpace(l)
		inCode := fence != ""
		if marker := fenceMarker(t); marker != "" {
			switch {
			case fence == "":
				fence = marker
				inCode = true
			case strings.HasPrefix(t, fence):
				fence = ""
				inCode = true
			}
		}
		out = append(out, mdLine{n: i + 1, text: l, inCode: inCode})
	}
	return out
}

func fenceMarker(trimmed string) string {
	for _, m := range []string{"```", "~~~"} {
		if strings.HasPrefix(trimmed, m) {
			return m
		}
	}
	return ""
}

func warnFinding(rel, field, rule, fix string) Finding {
	return Finding{File: rel, Field: field, Rule: rule, Severity: SeverityWarn, Fix: fix}
}

// lintStructural runs every structural rule for one decoded document.
func lintStructural(root, rel string, md *Markdown) []Finding {
	body := string(md.Body)
	var fs []Finding
	fs = append(fs, checkFrontmatterPresent(rel, md)...)
	fs = append(fs, checkHeadings(rel, body)...)
	fs = append(fs, checkTables(rel, body)...)
	fs = append(fs, checkClaims(root, rel, body)...)
	return fs
}

func checkFrontmatterPresent(rel string, md *Markdown) []Finding {
	if md != nil && md.HasFrontmatter {
		return nil
	}
	return []Finding{warnFinding(rel, "frontmatter", RuleMissingFrontmatter,
		"add a leading --- fenced YAML frontmatter block")}
}

var headingPattern = regexp.MustCompile(`^(#{1,6})\s+\S`)

func checkHeadings(rel, body string) []Finding {
	var fs []Finding
	prev, h1s := 0, 0
	for _, l := range splitLines(body) {
		if l.inCode {
			continue
		}
		m := headingPattern.FindStringSubmatch(l.text)
		if m == nil {
			continue
		}
		level := len(m[1])
		if level == 1 {
			h1s++
			if h1s == 2 {
				fs = append(fs, warnFinding(rel, fmt.Sprintf("line %d", l.n), RuleMultipleH1,
					"use a single H1 per document; demote this heading"))
			}
		}
		if prev > 0 && level > prev+1 {
			fs = append(fs, warnFinding(rel, fmt.Sprintf("line %d", l.n), RuleHeadingSkippedLevel,
				fmt.Sprintf("heading jumps from H%d to H%d; add the intermediate level", prev, level)))
		}
		prev = level
	}
	return fs
}

var tableDelimiter = regexp.MustCompile(`^\|?\s*:?-+:?\s*(\|\s*:?-+:?\s*)*\|?$`)

// tableCells counts cells in a table row, ignoring escaped pipes and pipes
// inside inline code spans.
func tableCells(line string) int {
	t := strings.TrimSpace(line)
	cells, inCode, escaped := 1, false, false
	for i, r := range t {
		switch {
		case escaped:
			escaped = false
		case r == '\\':
			escaped = true
		case r == '`':
			inCode = !inCode
		case r == '|' && !inCode && i > 0:
			cells++
		}
	}
	if strings.HasSuffix(t, "|") && !strings.HasSuffix(t, "\\|") {
		cells--
	}
	return cells
}

func checkTables(rel, body string) []Finding {
	lines := splitLines(body)
	var fs []Finding
	for i := 0; i+1 < len(lines); i++ {
		h, d := lines[i], lines[i+1]
		if h.inCode || d.inCode || !strings.HasPrefix(strings.TrimSpace(h.text), "|") ||
			!tableDelimiter.MatchString(strings.TrimSpace(d.text)) {
			continue
		}
		want := tableCells(h.text)
		j := i + 2
		for ; j < len(lines) && !lines[j].inCode && strings.HasPrefix(strings.TrimSpace(lines[j].text), "|"); j++ {
			if got := tableCells(lines[j].text); got != want {
				fs = append(fs, warnFinding(rel, fmt.Sprintf("line %d", lines[j].n), RuleTableShape,
					fmt.Sprintf("table row has %d columns; header has %d", got, want)))
			}
		}
		i = j - 1
	}
	return fs
}

var (
	claimPattern = regexp.MustCompile(`<!--\s*docline:(count|topology)\s+(.*?)\s*-->`)
	attrPattern  = regexp.MustCompile(`(\w+)="([^"]*)"|(\w+)=(\S+)`)
)

func parseAttrs(s string) map[string]string {
	out := map[string]string{}
	for _, m := range attrPattern.FindAllStringSubmatch(s, -1) {
		if m[1] != "" {
			out[m[1]] = m[2]
		} else {
			out[m[3]] = m[4]
		}
	}
	return out
}

// claimTarget resolves a claim-relative pattern inside root, rejecting
// absolute patterns and any that escape the workspace.
func claimTarget(root, pattern string) (string, bool) {
	if pattern == "" || filepath.IsAbs(pattern) || strings.HasPrefix(pattern, "/") || strings.HasPrefix(pattern, "\\") {
		return "", false
	}
	abs, err := core.SafeResolve(root, pattern)
	if err != nil {
		return "", false
	}
	return abs, true
}

// checkClaims verifies <!-- docline:count glob="..." expect=N --> and
// <!-- docline:topology path="..." kind=file|dir --> claims against the
// workspace tree. Claim targets are confined to the workspace root.
func checkClaims(root, rel, body string) []Finding {
	var fs []Finding
	for _, l := range splitLines(body) {
		if l.inCode {
			continue
		}
		for _, m := range claimPattern.FindAllStringSubmatch(l.text, -1) {
			attrs := parseAttrs(m[2])
			field := fmt.Sprintf("line %d", l.n)
			if m[1] == "count" {
				fs = append(fs, checkCountClaim(root, rel, field, attrs)...)
			} else {
				fs = append(fs, checkTopologyClaim(root, rel, field, attrs)...)
			}
		}
	}
	return fs
}

func checkCountClaim(root, rel, field string, attrs map[string]string) []Finding {
	expect, err := strconv.Atoi(attrs["expect"])
	pattern, ok := claimTarget(root, attrs["glob"])
	if err != nil || !ok {
		return []Finding{warnFinding(rel, field, RuleNumericClaim,
			"count claim needs a workspace-relative glob and an integer expect")}
	}
	matches, gerr := filepath.Glob(pattern)
	if gerr != nil {
		return []Finding{warnFinding(rel, field, RuleNumericClaim, "count claim glob is malformed")}
	}
	if len(matches) != expect {
		return []Finding{warnFinding(rel, field, RuleNumericClaim,
			fmt.Sprintf("claim says %d matches for %q; workspace has %d", expect, attrs["glob"], len(matches)))}
	}
	return nil
}

func checkTopologyClaim(root, rel, field string, attrs map[string]string) []Finding {
	kind := attrs["kind"]
	abs, ok := claimTarget(root, attrs["path"])
	if !ok || (kind != "file" && kind != "dir") {
		return []Finding{warnFinding(rel, field, RuleTopologyClaim,
			"topology claim needs a workspace-relative path and kind=file|dir")}
	}
	info, err := os.Stat(abs)
	switch {
	case err != nil:
		return []Finding{warnFinding(rel, field, RuleTopologyClaim,
			fmt.Sprintf("claimed %s %q does not exist", kind, attrs["path"]))}
	case kind == "dir" && !info.IsDir(), kind == "file" && info.IsDir():
		return []Finding{warnFinding(rel, field, RuleTopologyClaim,
			fmt.Sprintf("%q exists but is not a %s", attrs["path"], kind))}
	}
	return nil
}
