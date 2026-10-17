package faultline

// RedGreen evidence (S9 / 161.003-T): the v2 "red_green" family records the
// observed baseline (red) and head (green) result of each newly added test plus
// the honesty classification. S9 is a PRODUCER; the schema and registry stay
// owned by this foundation package, and v1 ({parity}) is immutable.

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// EvidenceSchemaVersionV2 is the schema version whose closed family set is
// {parity, red_green}. EvidenceSchemaVersion (v1, {parity}) is immutable.
const EvidenceSchemaVersionV2 = 2

// NodeFamilyRedGreen is the v2 node family for red-test honesty evidence (S9).
const NodeFamilyRedGreen = "red_green"

// RedGreenTest is one test's observed baseline (red) and head (green) result
// and honesty classification.
type RedGreenTest struct {
	Package          string   `json:"package"`
	Test             string   `json:"test"`
	BaselineResult   string   `json:"baseline_result"`
	HeadResult       string   `json:"head_result"`
	Honest           bool     `json:"honest"`
	RejectionClasses []string `json:"rejection_classes"`
}

// RedGreenEvidence is the v2 "red_green" family payload.
type RedGreenEvidence struct {
	BaseSHA string         `json:"base_sha"`
	HeadSHA string         `json:"head_sha"`
	Tests   []RedGreenTest `json:"tests"`
}

var (
	redGreenSHAPattern   = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	redGreenClassPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	redGreenTestPattern  = regexp.MustCompile(`^Test[\p{L}\p{N}_]*$`)
)

func validRedGreenResult(s string) bool {
	switch s {
	case "pass", "fail", "unrunnable":
		return true
	default:
		return false
	}
}

// ValidCommitSHA reports whether s is a full lowercase hex commit object name
// (SHA-1 or SHA-256 length). Producers use it so they and this contract can
// never disagree about what an immutable commit identifier is.
func ValidCommitSHA(s string) bool { return redGreenSHAPattern.MatchString(s) }

// ValidTestName reports whether s is a safe top-level Go test function name
// (also safe to embed in an anchored `go test -run` regular expression).
func ValidTestName(s string) bool { return redGreenTestPattern.MatchString(s) }

// ValidPackageDir reports whether p is "." or a safe slash-separated relative
// package directory: no empty, "." or ".." elements, no "..." wildcard, no
// backslash, colon, control character, leading slash or leading dash.
func ValidPackageDir(p string) bool { return validRedGreenPackage(p) }

// validRedGreenPackage reports whether p is a slash-separated relative package
// directory ("." or elements without "..", backslashes or a leading slash).
func validRedGreenPackage(p string) bool {
	if p == "." {
		return true
	}
	if p == "" || len(p) > 512 || p[0] == '/' || p[0] == '-' {
		return false
	}
	start := 0
	for i := 0; i <= len(p); i++ {
		if i == len(p) || p[i] == '/' {
			el := p[start:i]
			if el == "" || el == "." || el == ".." || strings.Contains(el, "...") {
				return false
			}
			start = i + 1
			continue
		}
		if p[i] == '\\' || p[i] < 0x20 || p[i] == 0x7f || p[i] == ':' {
			return false
		}
	}
	return true
}

func (t RedGreenTest) key() string { return t.Package + "::" + t.Test }

// sortedTests returns the tests ordered by (package, test) for canonical output.
func (p *RedGreenEvidence) sortedTests() []RedGreenTest {
	out := append([]RedGreenTest(nil), p.Tests...)
	sort.Slice(out, func(i, j int) bool { return out[i].key() < out[j].key() })
	return out
}

// CanonicalMap implements FamilyPayload. Tests are ordered by (package, test)
// and rejection classes are sorted, so the digest is independent of source
// ordering; empty collections are empty arrays, never null.
func (p *RedGreenEvidence) CanonicalMap() map[string]any {
	tests := make([]any, 0, len(p.Tests))
	for _, t := range p.sortedTests() {
		tests = append(tests, map[string]any{
			"package":           t.Package,
			"test":              t.Test,
			"baseline_result":   t.BaselineResult,
			"head_result":       t.HeadResult,
			"honest":            t.Honest,
			"rejection_classes": sortedAnySet(t.RejectionClasses),
		})
	}
	return map[string]any{
		"base_sha": p.BaseSHA,
		"head_sha": p.HeadSHA,
		"tests":    tests,
	}
}

// Validate implements FamilyPayload. It enforces payload invariants and the
// cross-envelope rules: head_sha is bound to the envelope producing_commit, and
// the envelope status is pass only when every test is honest (fail otherwise).
// report_only has no meaning for red_green evidence and is rejected.
func (p *RedGreenEvidence) Validate(view EvidenceView) error {
	if !redGreenSHAPattern.MatchString(p.BaseSHA) || !redGreenSHAPattern.MatchString(p.HeadSHA) {
		return fmt.Errorf("%w: base_sha and head_sha must be full lowercase commit SHAs", ErrValidation)
	}
	if p.BaseSHA == p.HeadSHA {
		return fmt.Errorf("%w: base_sha must differ from head_sha", ErrValidation)
	}
	if view.ProducingCommit != p.HeadSHA {
		return fmt.Errorf("%w: head_sha must equal the envelope producing_commit", ErrValidation)
	}
	if len(p.Tests) == 0 {
		return fmt.Errorf("%w: tests is required", ErrValidation)
	}
	seen := make(map[string]struct{}, len(p.Tests))
	dishonest := 0
	for i, t := range p.Tests {
		if !validRedGreenPackage(t.Package) {
			return fmt.Errorf("%w: tests[%d].package is not a safe relative package path", ErrValidation, i)
		}
		if !redGreenTestPattern.MatchString(t.Test) {
			return fmt.Errorf("%w: tests[%d].test is not a valid test name", ErrValidation, i)
		}
		if _, dup := seen[t.key()]; dup {
			return fmt.Errorf("%w: tests[%d] duplicates an earlier test", ErrValidation, i)
		}
		seen[t.key()] = struct{}{}
		if !validRedGreenResult(t.BaselineResult) || !validRedGreenResult(t.HeadResult) {
			return fmt.Errorf("%w: tests[%d] result not in {pass, fail, unrunnable}", ErrValidation, i)
		}
		if hasDuplicate(t.RejectionClasses) {
			return fmt.Errorf("%w: tests[%d].rejection_classes contains duplicates", ErrValidation, i)
		}
		for _, c := range t.RejectionClasses {
			if !redGreenClassPattern.MatchString(c) {
				return fmt.Errorf("%w: tests[%d].rejection_classes has a malformed class token", ErrValidation, i)
			}
		}
		if t.Honest {
			if t.BaselineResult != "fail" || t.HeadResult != "pass" || len(t.RejectionClasses) != 0 {
				return fmt.Errorf("%w: tests[%d] honest requires baseline=fail, head=pass and no rejection classes", ErrValidation, i)
			}
		} else {
			dishonest++
			if len(t.RejectionClasses) == 0 {
				return fmt.Errorf("%w: tests[%d] dishonest requires at least one rejection class", ErrValidation, i)
			}
		}
	}
	switch view.Status {
	case StatusPass:
		if dishonest != 0 {
			return fmt.Errorf("%w: status=pass requires every test honest", ErrValidation)
		}
	case StatusFail:
		if dishonest == 0 {
			return fmt.Errorf("%w: status=fail requires at least one dishonest test", ErrValidation)
		}
	default:
		return fmt.Errorf("%w: status %q not valid for red_green evidence (pass or fail only)", ErrValidation, view.Status)
	}
	return nil
}
