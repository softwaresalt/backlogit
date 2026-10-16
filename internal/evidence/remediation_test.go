package evidence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reviewJSON(head, prAuthor, reviews, extra string, hasNext bool, cursor string) string {
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"headRefOid":%q,"author":{"login":%q},
"reviewRequests":{"nodes":[],"pageInfo":{"hasNextPage":%t}},
"reviews":{"nodes":[%s],"pageInfo":{"hasPreviousPage":%t}},
"reviewThreads":{"nodes":[],"pageInfo":{"hasNextPage":%t,"endCursor":%q}}%s}}}}`,
		head, prAuthor, false, reviews, false, hasNext, cursor, extra)
}

func TestRemediation_DismissedAndSelfReviewsIgnored(t *testing.T) {
	reviews := `{"state":"DISMISSED","author":{"login":"bob"},"commit":{"oid":"head1"}},
{"state":"APPROVED","author":{"login":"author"},"commit":{"oid":"head1"}}`
	fr := &fakeRunner{resps: []fakeResp{{Stdout: reviewJSON("head1", "author", reviews, "", false, "")}}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).ReviewEvidence(context.Background(), 1)
	assert.Equal(t, StatusVerified, res.Status, res.Reason)
	assert.False(t, res.ReviewedCurrent)
	assert.Empty(t, res.ReviewedHeads)
	assert.Zero(t, res.ReviewCycles)
}

func TestRemediation_HeadChangeAcrossPagesDegrades(t *testing.T) {
	fr := &fakeRunner{resps: []fakeResp{
		{Stdout: reviewJSON("h1", "a", "", "", true, "C1")},
		{Stdout: reviewJSON("h2", "a", "", "", false, "")},
	}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).ReviewEvidence(context.Background(), 1)
	assert.Equal(t, StatusDegraded, res.Status)
}

func TestRemediation_IncompleteReviewPagesDegrade(t *testing.T) {
	more := strings.NewReplacer(`"reviews":{"nodes":[],"pageInfo":{"hasPreviousPage":false}}`,
		`"reviews":{"nodes":[],"pageInfo":{"hasPreviousPage":true}}`)
	moreReq := strings.NewReplacer(`"reviewRequests":{"nodes":[],"pageInfo":{"hasNextPage":false}}`,
		`"reviewRequests":{"nodes":[],"pageInfo":{"hasNextPage":true}}`)
	base := reviewJSON("h", "a", "", "", false, "")
	for name, payload := range map[string]string{"reviews": more.Replace(base), "requests": moreReq.Replace(base)} {
		t.Run(name, func(t *testing.T) {
			assert.NotEqual(t, base, payload)
			fr := &fakeRunner{resps: []fakeResp{{Stdout: payload}}}
			res := (&GitHub{Runner: fr, Repo: "o/r"}).ReviewEvidence(context.Background(), 1)
			assert.Equal(t, StatusDegraded, res.Status)
		})
	}
}

func TestRemediation_CIRollupCapAndSkippedOnly(t *testing.T) {
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"__typename":"CheckRun","name":"c","status":"COMPLETED","conclusion":"SUCCESS"}`)
	}
	capped := `{"headRefOid":"h","state":"OPEN","mergeCommit":null,"statusCheckRollup":[` + sb.String() + `]}`
	fr := &fakeRunner{resps: []fakeResp{{Stdout: capped}}}
	assert.Equal(t, StatusDegraded, (&GitHub{Runner: fr, Repo: "o/r"}).CIEvidence(context.Background(), 1).Status)

	skipped := `{"headRefOid":"h","state":"OPEN","mergeCommit":null,"statusCheckRollup":[
{"__typename":"CheckRun","name":"a","status":"COMPLETED","conclusion":"SKIPPED"},
{"__typename":"CheckRun","name":"b","status":"COMPLETED","conclusion":"NEUTRAL"}]}`
	fr = &fakeRunner{resps: []fakeResp{{Stdout: skipped}}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).CIEvidence(context.Background(), 1)
	assert.Equal(t, StatusVerified, res.Status, res.Reason)
	assert.False(t, res.AllPassed)
}

func TestRemediation_ScrubExtraPatternsAndStderrCap(t *testing.T) {
	cases := map[string]string{
		"userinfo":     "https://user:s3cretpw@github.com/o/r",
		"token query":  "https://x/y?token=abcd1234efgh&z=1",
		"access_token": "https://x/y?a=1&access_token=zzzz9999",
		"classic hex":  "leaked " + strings.Repeat("ab12", 10) + " end",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			out := Scrub(in)
			assert.Contains(t, out, "[REDACTED]")
			assert.NotContains(t, out, "s3cretpw")
			assert.NotContains(t, out, "abcd1234efgh")
			assert.NotContains(t, out, "zzzz9999")
			assert.NotContains(t, out, strings.Repeat("ab12", 10))
		})
	}
	long := degraded("x", fmt.Errorf("boom"), []byte(strings.Repeat("e", 5000)))
	assert.Less(t, len(long), 700)
}

func TestRemediation_ShaMatchesAndBacklogID(t *testing.T) {
	assert.False(t, shaMatches("zzzzzz", "zzzzzz1234"))
	assert.False(t, shaMatches("abc12g", "abc12g1234"))
	assert.True(t, shaMatches("ABC123", "abc123def"))

	fr := &fakeRunner{}
	g := &GitHub{Runner: fr, Repo: "o/r"}
	assert.Equal(t, StatusDegraded, g.ReviewEvidence(context.Background(), 0).Status)
	assert.Equal(t, StatusDegraded, g.CIEvidence(context.Background(), -1).Status)
	assert.Empty(t, fr.calls)
}

func TestRemediation_BacklogLoadIDMismatchDegrades(t *testing.T) {
	root := fixtureWorkspace(t, "active")
	data, err := os.ReadFile(filepath.Join(root, "queue", "900.002-T.md"))
	require.NoError(t, err)
	swapped := strings.Replace(string(data), "id: 900.002-T", "id: 999.999-T", 1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "queue", "900.002-T.md"), []byte(swapped), 0o644))
	res := (&Backlog{Root: root}).ShipmentEvidence("900-S")
	assert.Equal(t, StatusDegraded, res.Status)
}
