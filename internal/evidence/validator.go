package evidence

import (
	"fmt"
	"strings"
)

// Verdict is the outcome of checking one claim.
type Verdict string

const (
	// VerdictVerified means the evidence confirms the claim.
	VerdictVerified Verdict = "verified"
	// VerdictContradicted means the evidence disproves the claim.
	VerdictContradicted Verdict = "contradicted"
	// VerdictUnverifiable means no usable evidence exists for the claim.
	VerdictUnverifiable Verdict = "unverifiable"
)

// Claims are prose readiness claims lifted into structured form. A nil field
// means the claim was not made.
type Claims struct {
	ReviewedHead      *string
	CIPassed          *bool
	MergeSHA          *string
	TasksDone         *int
	TasksTotal        *int
	UnresolvedThreads *int
	ShipmentStatus    *string
}

// Derived bundles the evidence sources a claim may be checked against. A nil
// source means evidence was never gathered.
type Derived struct {
	Review  *ReviewResult
	CI      *CIResult
	Backlog *BacklogResult
}

// Finding is the per-claim validation outcome.
type Finding struct {
	Claim   string
	Verdict Verdict
	Reason  string
}

// Report is the validation outcome. Pass is true only when at least one claim
// was made and every claim is verified.
type Report struct {
	Pass     bool
	Findings []Finding
}

// minSHAPrefix is the shortest abbreviated SHA accepted as a match.
const minSHAPrefix = 6

func shaMatches(claimed, actual string) bool {
	claimed = strings.ToLower(strings.TrimSpace(claimed))
	actual = strings.ToLower(actual)
	return len(claimed) >= minSHAPrefix && strings.HasPrefix(actual, claimed)
}

func verified(claim string) Finding {
	return Finding{Claim: claim, Verdict: VerdictVerified, Reason: "matches derived evidence"}
}

func contradicted(claim, format string, args ...any) Finding {
	return Finding{Claim: claim, Verdict: VerdictContradicted, Reason: fmt.Sprintf(format, args...)}
}

func unverifiable(claim, reason string) Finding {
	return Finding{Claim: claim, Verdict: VerdictUnverifiable, Reason: reason}
}

func reviewUsable(claim string, r *ReviewResult) (Finding, bool) {
	if r == nil {
		return unverifiable(claim, "no GitHub review evidence was derived"), false
	}
	if r.Status != StatusVerified {
		return unverifiable(claim, "GitHub review evidence degraded: "+r.Reason), false
	}
	return Finding{}, true
}

func ciUsable(claim string, c *CIResult) (Finding, bool) {
	if c == nil {
		return unverifiable(claim, "no GitHub CI evidence was derived"), false
	}
	if c.Status != StatusVerified {
		return unverifiable(claim, "GitHub CI evidence degraded: "+c.Reason), false
	}
	return Finding{}, true
}

func backlogUsable(claim string, b *BacklogResult) (Finding, bool) {
	if b == nil {
		return unverifiable(claim, "no backlog evidence was derived"), false
	}
	if b.Status != StatusVerified {
		return unverifiable(claim, "backlog evidence degraded: "+b.Reason), false
	}
	return Finding{}, true
}

func intClaim(claim string, claimed, actual int, label string) Finding {
	if claimed != actual {
		return contradicted(claim, "claimed %s %d but evidence shows %d", label, claimed, actual)
	}
	return verified(claim)
}

// Validate compares claims against derived evidence, failing closed on
// contradicted or unverifiable claims. Only claims that were made are checked;
// a report with no claims does not pass.
func Validate(c Claims, d Derived) Report {
	var fs []Finding
	add := func(f Finding) { fs = append(fs, f) }

	if c.ReviewedHead != nil {
		const k = "reviewed_head"
		if f, ok := reviewUsable(k, d.Review); !ok {
			add(f)
		} else {
			r := d.Review
			switch {
			case !shaMatches(*c.ReviewedHead, r.HeadSHA) && !anySHAMatches(*c.ReviewedHead, r.ReviewedHeads):
				add(contradicted(k, "claimed reviewed head %s was never reviewed; reviewed heads %v, current head %s", *c.ReviewedHead, r.ReviewedHeads, r.HeadSHA))
			case !shaMatches(*c.ReviewedHead, r.HeadSHA):
				add(contradicted(k, "claimed head %s is stale; current head is %s", *c.ReviewedHead, r.HeadSHA))
			case !r.ReviewedCurrent:
				add(contradicted(k, "current head %s has no submitted review", r.HeadSHA))
			default:
				add(verified(k))
			}
		}
	}
	if c.UnresolvedThreads != nil {
		const k = "unresolved_threads"
		if f, ok := reviewUsable(k, d.Review); !ok {
			add(f)
		} else {
			add(intClaim(k, *c.UnresolvedThreads, d.Review.UnresolvedThreads, "unresolved threads"))
		}
	}
	if c.CIPassed != nil {
		const k = "ci_passed"
		if f, ok := ciUsable(k, d.CI); !ok {
			add(f)
		} else if *c.CIPassed != d.CI.AllPassed {
			add(contradicted(k, "claimed ci passed=%t but ci checks passed=%t", *c.CIPassed, d.CI.AllPassed))
		} else {
			add(verified(k))
		}
	}
	if c.MergeSHA != nil {
		const k = "merge_sha"
		if f, ok := ciUsable(k, d.CI); !ok {
			add(f)
		} else if !d.CI.Merged {
			add(contradicted(k, "claimed merge sha %s but the pull request is not merged", *c.MergeSHA))
		} else if !shaMatches(*c.MergeSHA, d.CI.MergeSHA) {
			add(contradicted(k, "claimed merge sha %s but evidence shows %s", *c.MergeSHA, d.CI.MergeSHA))
		} else {
			add(verified(k))
		}
	}
	if c.TasksDone != nil {
		const k = "tasks_done"
		if f, ok := backlogUsable(k, d.Backlog); !ok {
			add(f)
		} else {
			add(intClaim(k, *c.TasksDone, d.Backlog.TasksDone, "tasks done"))
		}
	}
	if c.TasksTotal != nil {
		const k = "tasks_total"
		if f, ok := backlogUsable(k, d.Backlog); !ok {
			add(f)
		} else {
			add(intClaim(k, *c.TasksTotal, d.Backlog.TaskTotal, "tasks total"))
		}
	}
	if c.ShipmentStatus != nil {
		const k = "shipment_status"
		if f, ok := backlogUsable(k, d.Backlog); !ok {
			add(f)
		} else if !strings.EqualFold(*c.ShipmentStatus, d.Backlog.ShipmentStatus) {
			add(contradicted(k, "claimed shipment status %q but evidence shows %q", *c.ShipmentStatus, d.Backlog.ShipmentStatus))
		} else {
			add(verified(k))
		}
	}

	pass := len(fs) > 0
	for _, f := range fs {
		if f.Verdict != VerdictVerified {
			pass = false
		}
	}
	return Report{Pass: pass, Findings: fs}
}

func anySHAMatches(claimed string, actual []string) bool {
	for _, a := range actual {
		if shaMatches(claimed, a) {
			return true
		}
	}
	return false
}
