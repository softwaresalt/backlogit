package evidence

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCall struct {
	Name  string
	Args  []string
	Env   []string
	Stdin []byte
}

type fakeResp struct {
	Stdout string
	Stderr string
	Err    error
}

// fakeRunner returns scripted responses in order and records every call.
type fakeRunner struct {
	mu    sync.Mutex
	resps []fakeResp
	calls []fakeCall
}

func (f *fakeRunner) Run(_ context.Context, name string, args, env []string, stdin []byte) ([]byte, []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fakeCall{name, args, env, stdin})
	if len(f.resps) == 0 {
		return nil, nil, errors.New("fakeRunner: no scripted response")
	}
	r := f.resps[0]
	if len(f.resps) > 1 {
		f.resps = f.resps[1:]
	}
	return []byte(r.Stdout), []byte(r.Stderr), r.Err
}

func (f *fakeRunner) allArgs() string {
	var sb strings.Builder
	for _, c := range f.calls {
		sb.WriteString(c.Name + " " + strings.Join(c.Args, " ") + "\n")
	}
	return sb.String()
}

func reviewPage(head string, hasNext bool, cursor string, resolved ...bool) string {
	threads := make([]string, 0, len(resolved))
	for _, r := range resolved {
		threads = append(threads, fmt.Sprintf(`{"isResolved":%t}`, r))
	}
	return fmt.Sprintf(`{"data":{"repository":{"pullRequest":{"headRefOid":%q,
"reviewRequests":{"nodes":[{"requestedReviewer":{"login":"alice"}},{"requestedReviewer":{"name":"team-x"}}]},
"reviews":{"nodes":[
 {"state":"COMMENTED","commit":{"oid":"aaa111"}},
 {"state":"CHANGES_REQUESTED","commit":{"oid":"aaa111"}},
 {"state":"APPROVED","commit":{"oid":"bbb222"}},
 {"state":"PENDING","commit":{"oid":"ccc333"}}]},
"reviewThreads":{"nodes":[%s],"pageInfo":{"hasNextPage":%t,"endCursor":%q}}}}}}`,
		head, strings.Join(threads, ","), hasNext, cursor)
}

func TestU159001_ReviewEvidenceMatchesGroundTruth(t *testing.T) {
	fr := &fakeRunner{resps: []fakeResp{
		{Stdout: reviewPage("bbb222", true, "CUR1", true, false)},
		{Stdout: reviewPage("bbb222", false, "", false, true)},
	}}
	g := &GitHub{Runner: fr, Repo: "o/r"}
	res := g.ReviewEvidence(context.Background(), 42)

	assert.Equal(t, StatusVerified, res.Status, res.Reason)
	assert.Equal(t, 42, res.PR)
	assert.Equal(t, "bbb222", res.HeadSHA)
	assert.Equal(t, []string{"aaa111", "bbb222"}, res.ReviewedHeads)
	assert.True(t, res.ReviewedCurrent)
	assert.Equal(t, 3, res.ReviewCycles)
	assert.Equal(t, []string{"alice", "team-x"}, res.ReviewRequests)
	assert.Equal(t, 2, res.UnresolvedThreads, "threads paginated across pages")
	assert.Len(t, fr.calls, 2)
}

func TestU159001_ReviewedHeadStale(t *testing.T) {
	fr := &fakeRunner{resps: []fakeResp{{Stdout: reviewPage("newhead", false, "", true)}}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).ReviewEvidence(context.Background(), 1)
	assert.Equal(t, StatusVerified, res.Status, res.Reason)
	assert.False(t, res.ReviewedCurrent)
}

func TestU159001_TokenNeverInArgv(t *testing.T) {
	const tok = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	fr := &fakeRunner{resps: []fakeResp{{Stdout: reviewPage("h", false, "", true)}}}
	g := &GitHub{Runner: fr, Repo: "o/r", Token: tok}
	g.ReviewEvidence(context.Background(), 1)

	require.Len(t, fr.calls, 1)
	assert.NotContains(t, fr.allArgs(), tok)
	assert.Contains(t, fr.calls[0].Env, "GH_TOKEN="+tok)
}

func TestU159001_OutageDegradesAndScrubsStderr(t *testing.T) {
	const tok = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	fr := &fakeRunner{resps: []fakeResp{{
		Stderr: "HTTP 502 Authorization: Bearer " + tok + " token github_pat_11ABCDEFG0123456789_abcdefghijklmnop gho_abcdefghijklmnopqrstuvwxyz0123456789",
		Err:    errors.New("exit status 1"),
	}}}
	res := (&GitHub{Runner: fr, Repo: "o/r", Token: tok}).ReviewEvidence(context.Background(), 7)

	assert.Equal(t, StatusDegraded, res.Status)
	assert.NotEmpty(t, res.Reason)
	for _, secret := range []string{tok, "github_pat_11ABCDEFG", "gho_abcdef"} {
		assert.NotContains(t, res.Reason, secret)
	}
	assert.Zero(t, res.UnresolvedThreads)
	assert.False(t, res.ReviewedCurrent)
}

func TestU159001_MalformedOutputDegrades(t *testing.T) {
	fr := &fakeRunner{resps: []fakeResp{{Stdout: "not json"}}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).ReviewEvidence(context.Background(), 1)
	assert.Equal(t, StatusDegraded, res.Status)
}

func TestU159001_MissingPRDegrades(t *testing.T) {
	fr := &fakeRunner{resps: []fakeResp{{Stdout: `{"data":{"repository":{"pullRequest":null}}}`}}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).ReviewEvidence(context.Background(), 1)
	assert.Equal(t, StatusDegraded, res.Status)
}

func TestU159001_CancelledContextDegrades(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fr := &fakeRunner{resps: []fakeResp{{Stdout: reviewPage("h", false, "")}}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).ReviewEvidence(ctx, 1)
	assert.Equal(t, StatusDegraded, res.Status)
}

func TestU159001_ScrubRedactsTokenShapes(t *testing.T) {
	cases := []string{
		"ghp_abcdefghijklmnopqrstuvwxyz0123456789",
		"gho_abcdefghijklmnopqrstuvwxyz0123456789",
		"ghs_abcdefghijklmnopqrstuvwxyz0123456789",
		"github_pat_11ABCDEFG0123456789_abcdefghijklmnop",
	}
	for _, c := range cases {
		t.Run(c[:4], func(t *testing.T) {
			out := Scrub("failed with " + c + " end")
			assert.NotContains(t, out, c)
			assert.Contains(t, out, "[REDACTED]")
		})
	}
	t.Run("authorization header", func(t *testing.T) {
		out := Scrub("Authorization: Bearer abc.def-123\nnext line")
		assert.NotContains(t, out, "abc.def-123")
		assert.Contains(t, out, "next line")
	})
	t.Run("bearer", func(t *testing.T) {
		assert.NotContains(t, Scrub("got bearer sometoken123 ok"), "sometoken123")
	})
	t.Run("plain text untouched", func(t *testing.T) {
		assert.Equal(t, "HTTP 502 bad gateway", Scrub("HTTP 502 bad gateway"))
	})
}
