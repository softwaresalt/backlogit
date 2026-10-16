package docline

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Reference rule identifiers reported in Finding.Rule.
const (
	// RuleAnchorOutsideRoot flags an anchor that resolves outside the workspace root.
	RuleAnchorOutsideRoot = "reference_anchor_outside_root"
	// RuleAnchorUnresolved flags an anchor whose file, line or symbol does not exist.
	RuleAnchorUnresolved = "reference_anchor_unresolved"
	// RuleTaskIDMissing flags a task ID that is not a known backlog item.
	RuleTaskIDMissing = "reference_task_id_missing"
	// RuleStaleTerm flags a deprecated term from the configured stale-term map.
	RuleStaleTerm = "reference_stale_term"
	// RuleCommentDrift flags a docline:comment-ref whose quoted text no longer
	// matches the implementation doc comment.
	RuleCommentDrift = "reference_comment_drift"
)

var (
	codeSpan = regexp.MustCompile("`([^`\\s]+)`")
	// anchorCandidate keeps ordinary code spans (host:port, bare file names)
	// out of the anchor rule: it needs a known extension plus :line or ::symbol.
	anchorCandidate = regexp.MustCompile(`^(?:[A-Za-z]:)?[^\s:]+\.(?:go|md|ya?ml|json|sql|ps1|sh|ts|py|toml)(?::\d+(?:-\d+)?|::[A-Za-z_][\w.]*)$`)
	taskIDPattern   = regexp.MustCompile(`\b\d{3}(?:\.\d{3}){0,2}-[A-Z]{1,3}\b`)
	itemFileName    = regexp.MustCompile(`^(\d{3}(?:\.\d{3}){0,2}-[A-Z]{1,3})\.md$`)
	commentRefTag   = regexp.MustCompile(`<!--\s*docline:comment-ref\s+(.*?)\s*-->`)
)

// lintReferences runs every enabled reference rule for one decoded document.
// taskIDs may be nil, which skips the task-ID rule.
func lintReferences(root, rel string, md *Markdown, rules RuleSet, taskIDs func(string) bool) []Finding {
	body := string(md.Body)
	out := checkAnchors(root, rel, body)
	if taskIDs != nil {
		out = append(out, checkTaskIDs(rel, body, taskIDs)...)
	}
	out = append(out, checkStaleTerms(rel, body, rules.StaleTerms)...)
	out = append(out, checkCommentDrift(root, rel, body)...)
	return out
}

// anchorFinding classifies a resolver error as a finding.
func anchorFinding(rel, field, anchor string, err error) Finding {
	if errors.Is(err, ErrAnchorOutsideRoot) {
		return warnFinding(rel, field, RuleAnchorOutsideRoot,
			fmt.Sprintf("anchor %q resolves outside the workspace root", anchor))
	}
	return warnFinding(rel, field, RuleAnchorUnresolved, fmt.Sprintf("anchor %q: %v", anchor, err))
}

func checkAnchors(root, rel, body string) []Finding {
	var out []Finding
	for _, l := range splitLines(body) {
		if l.inCode {
			continue
		}
		for _, m := range codeSpan.FindAllStringSubmatch(l.text, -1) {
			if !anchorCandidate.MatchString(m[1]) {
				continue
			}
			if _, err := ResolveAnchorString(root, m[1]); err != nil {
				out = append(out, anchorFinding(rel, fmt.Sprintf("line %d", l.n), m[1], err))
			}
		}
	}
	return out
}

func checkTaskIDs(rel, body string, exists func(string) bool) []Finding {
	var out []Finding
	for _, l := range splitLines(body) {
		if l.inCode {
			continue
		}
		for _, id := range taskIDPattern.FindAllString(l.text, -1) {
			if !exists(id) {
				out = append(out, warnFinding(rel, fmt.Sprintf("line %d", l.n), RuleTaskIDMissing,
					fmt.Sprintf("task ID %s is not a known backlog item", id)))
			}
		}
	}
	return out
}

func checkStaleTerms(rel, body string, terms map[string]string) []Finding {
	if len(terms) == 0 {
		return nil
	}
	pats := make(map[string]*regexp.Regexp, len(terms))
	keys := make([]string, 0, len(terms))
	for term := range terms {
		pats[term] = regexp.MustCompile(`\b` + regexp.QuoteMeta(term) + `\b`)
		keys = append(keys, term)
	}
	sort.Strings(keys)
	var out []Finding
	for _, l := range splitLines(body) {
		if l.inCode {
			continue
		}
		for _, term := range keys {
			if pats[term].MatchString(l.text) {
				out = append(out, warnFinding(rel, fmt.Sprintf("line %d", l.n), RuleStaleTerm,
					fmt.Sprintf("stale term %q; use %q", term, terms[term])))
			}
		}
	}
	return out
}

func normalizeProse(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// declComment returns the contiguous // comment block directly above the
// declaration of symbol in content, and whether the declaration was found.
func declComment(content, symbol string) (string, bool) {
	if i := strings.LastIndex(symbol, "."); i >= 0 && i < len(symbol)-1 {
		symbol = symbol[i+1:]
	}
	decl := regexp.MustCompile(`^(?:func\s+(?:\([^)]*\)\s*)?|type\s+|var\s+|const\s+)` + regexp.QuoteMeta(symbol) + `\b`)
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i, l := range lines {
		if !decl.MatchString(l) {
			continue
		}
		var block []string
		for j := i - 1; j >= 0 && strings.HasPrefix(strings.TrimSpace(lines[j]), "//"); j-- {
			block = append([]string{strings.TrimPrefix(strings.TrimSpace(lines[j]), "//")}, block...)
		}
		return strings.Join(block, " "), true
	}
	return "", false
}

// checkCommentDrift verifies <!-- docline:comment-ref path="..." symbol="..."
// text="..." --> tags: the quoted text must still appear in the doc comment of
// the referenced declaration. Paths are confined to the workspace root.
func checkCommentDrift(root, rel, body string) []Finding {
	var out []Finding
	for _, l := range splitLines(body) {
		if l.inCode {
			continue
		}
		for _, m := range commentRefTag.FindAllStringSubmatch(l.text, -1) {
			attrs := parseAttrs(m[1])
			field := fmt.Sprintf("line %d", l.n)
			_, content, err := loadAnchor(root, Anchor{Path: attrs["path"], Symbol: attrs["symbol"]})
			if err != nil {
				if errors.Is(err, ErrAnchorOutsideRoot) {
					out = append(out, anchorFinding(rel, field, attrs["path"], err))
				} else {
					out = append(out, warnFinding(rel, field, RuleCommentDrift,
						fmt.Sprintf("comment reference %s::%s: %v", attrs["path"], attrs["symbol"], err)))
				}
				continue
			}
			comment, found := declComment(content, attrs["symbol"])
			switch {
			case !found:
				out = append(out, warnFinding(rel, field, RuleCommentDrift,
					fmt.Sprintf("declaration of %q not found in %q", attrs["symbol"], attrs["path"])))
			case attrs["text"] == "" || !strings.Contains(normalizeProse(comment), normalizeProse(attrs["text"])):
				out = append(out, warnFinding(rel, field, RuleCommentDrift,
					fmt.Sprintf("doc comment of %q no longer contains %q", attrs["symbol"], attrs["text"])))
			}
		}
	}
	return out
}

// workspaceTaskIDs returns the set of backlog item IDs found under the
// workspace backlog directories (.backlogit and .backlog), keyed by file name.
// A workspace with no backlog directory yields an empty set.
func workspaceTaskIDs(root string) (map[string]bool, error) {
	ids := map[string]bool{}
	for _, dir := range []string{".backlogit", ".backlog"} {
		base := filepath.Join(root, dir)
		info, err := os.Stat(base)
		if err != nil || !info.IsDir() {
			continue
		}
		walkErr := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if m := itemFileName.FindStringSubmatch(d.Name()); m != nil {
				ids[m[1]] = true
			}
			return nil
		})
		if walkErr != nil {
			return nil, fmt.Errorf("docline.workspaceTaskIDs: walk %s: %w", dir, walkErr)
		}
	}
	return ids, nil
}
