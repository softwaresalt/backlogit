package evidence

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }
func intp(i int) *int       { return &i }

func goodDerived() Derived {
	return Derived{
		Review: &ReviewResult{Status: StatusVerified, HeadSHA: "bbb222", ReviewedHeads: []string{"aaa111", "bbb222"},
			ReviewedCurrent: true, UnresolvedThreads: 0},
		CI:      &CIResult{Status: StatusVerified, HeadSHA: "bbb222", AllPassed: true, Merged: true, MergeSHA: "def456"},
		Backlog: &BacklogResult{Status: StatusVerified, ShipmentStatus: "shipped", Closed: true, TaskTotal: 4, TasksDone: 4},
	}
}

func accurateClaims() Claims {
	return Claims{
		ReviewedHead:      strp("bbb222"),
		CIPassed:          boolp(true),
		MergeSHA:          strp("def456"),
		TasksDone:         intp(4),
		TasksTotal:        intp(4),
		UnresolvedThreads: intp(0),
		ShipmentStatus:    strp("shipped"),
	}
}

func findingFor(t *testing.T, r Report, claim string) Finding {
	t.Helper()
	for _, f := range r.Findings {
		if f.Claim == claim {
			return f
		}
	}
	require.Failf(t, "missing finding", "claim %q", claim)
	return Finding{}
}

func TestU159004_AccurateClaimsPass(t *testing.T) {
	r := Validate(accurateClaims(), goodDerived())
	assert.True(t, r.Pass, "%+v", r.Findings)
	assert.Len(t, r.Findings, 7)
	for _, f := range r.Findings {
		assert.Equal(t, VerdictVerified, f.Verdict, f.Claim)
	}
}

func TestU159004_ContradictedClaimsFailWithSpecificReason(t *testing.T) {
	cases := []struct {
		claim  string
		mutate func(*Claims)
		reason string
	}{
		{"reviewed_head", func(c *Claims) { c.ReviewedHead = strp("zzz999") }, "zzz999"},
		{"ci_passed", func(c *Claims) {}, "ci"},
		{"merge_sha", func(c *Claims) { c.MergeSHA = strp("000000") }, "def456"},
		{"tasks_done", func(c *Claims) { c.TasksDone = intp(5) }, "5"},
		{"tasks_total", func(c *Claims) { c.TasksTotal = intp(9) }, "9"},
		{"unresolved_threads", func(c *Claims) { c.UnresolvedThreads = intp(0) }, "3"},
		{"shipment_status", func(c *Claims) { c.ShipmentStatus = strp("active") }, "shipped"},
	}
	for _, tc := range cases {
		t.Run(tc.claim, func(t *testing.T) {
			c, d := accurateClaims(), goodDerived()
			tc.mutate(&c)
			switch tc.claim {
			case "ci_passed":
				d.CI.AllPassed = false
			case "unresolved_threads":
				d.Review.UnresolvedThreads = 3
			}
			r := Validate(c, d)
			assert.False(t, r.Pass)
			f := findingFor(t, r, tc.claim)
			assert.Equal(t, VerdictContradicted, f.Verdict)
			assert.Contains(t, f.Reason, tc.reason)
		})
	}
}

func TestU159004_StaleReviewedHeadContradicted(t *testing.T) {
	d := goodDerived()
	d.Review.HeadSHA = "newhead"
	d.Review.ReviewedCurrent = false
	c := Claims{ReviewedHead: strp("bbb222")}
	r := Validate(c, d)
	assert.False(t, r.Pass)
	f := findingFor(t, r, "reviewed_head")
	assert.Equal(t, VerdictContradicted, f.Verdict)
	assert.Contains(t, f.Reason, "newhead")
}

func TestU159004_DegradedOrMissingEvidenceIsUnverifiable(t *testing.T) {
	c := accurateClaims()

	d := goodDerived()
	d.Review = &ReviewResult{Status: StatusDegraded, Reason: "gh outage"}
	d.CI = &CIResult{Status: StatusDegraded, Reason: "gh outage"}
	d.Backlog = &BacklogResult{Status: StatusDegraded, Reason: "missing member"}
	r := Validate(c, d)
	assert.False(t, r.Pass)
	for _, f := range r.Findings {
		assert.Equal(t, VerdictUnverifiable, f.Verdict, f.Claim)
		assert.NotEmpty(t, f.Reason)
	}

	r = Validate(c, Derived{})
	assert.False(t, r.Pass)
	for _, f := range r.Findings {
		assert.Equal(t, VerdictUnverifiable, f.Verdict, f.Claim)
	}
}

func TestU159004_DegradedCIDoesNotPassFalseClaim(t *testing.T) {
	d := goodDerived()
	d.CI = &CIResult{Status: StatusDegraded, Reason: "outage"}
	r := Validate(Claims{CIPassed: boolp(false)}, d)
	assert.False(t, r.Pass, "even a negative claim is unverifiable under degradation")
}

func TestU159004_NoClaimsFailsClosed(t *testing.T) {
	r := Validate(Claims{}, goodDerived())
	assert.False(t, r.Pass)
	assert.Empty(t, r.Findings)
}

func TestU159004_PartialClaimsOnlyCheckMade(t *testing.T) {
	r := Validate(Claims{MergeSHA: strp("def456")}, Derived{CI: goodDerived().CI})
	assert.True(t, r.Pass, "%+v", r.Findings)
	assert.Len(t, r.Findings, 1)
}

func TestU159004_ShortSHAPrefixAccepted(t *testing.T) {
	r := Validate(Claims{MergeSHA: strp("def456")}, Derived{CI: &CIResult{Status: StatusVerified, Merged: true, MergeSHA: "def4567890abcdef"}})
	assert.True(t, r.Pass, "%+v", r.Findings)
	r = Validate(Claims{MergeSHA: strp("d")}, Derived{CI: &CIResult{Status: StatusVerified, Merged: true, MergeSHA: "def4567890abcdef"}})
	assert.False(t, r.Pass, "too-short prefix must not match")
}

func TestU159004_MergeClaimWhenNotMerged(t *testing.T) {
	d := Derived{CI: &CIResult{Status: StatusVerified, Merged: false}}
	r := Validate(Claims{MergeSHA: strp("def456")}, d)
	assert.False(t, r.Pass)
	assert.Equal(t, VerdictContradicted, findingFor(t, r, "merge_sha").Verdict)
}
