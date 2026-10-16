package evidence

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ciAllGreen = `{"headRefOid":"abc123","state":"MERGED","mergeCommit":{"oid":"def456"},
"statusCheckRollup":[
 {"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"SUCCESS"},
 {"__typename":"CheckRun","name":"lint","status":"COMPLETED","conclusion":"SKIPPED"},
 {"__typename":"StatusContext","context":"legacy/ci","state":"SUCCESS"}]}`

func TestU159002_CIEvidenceAllPassedAndMerged(t *testing.T) {
	fr := &fakeRunner{resps: []fakeResp{{Stdout: ciAllGreen}}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).CIEvidence(context.Background(), 9)

	assert.Equal(t, StatusVerified, res.Status, res.Reason)
	assert.Equal(t, "abc123", res.HeadSHA)
	assert.True(t, res.AllPassed)
	assert.True(t, res.Merged)
	assert.Equal(t, "def456", res.MergeSHA)
	assert.Len(t, res.Checks, 3)
}

func TestU159002_CIFailureOrPendingIsNotPassed(t *testing.T) {
	cases := map[string]string{
		"failure": `{"headRefOid":"h","state":"OPEN","mergeCommit":null,"statusCheckRollup":[
{"__typename":"CheckRun","name":"build","status":"COMPLETED","conclusion":"FAILURE"},
{"__typename":"CheckRun","name":"lint","status":"COMPLETED","conclusion":"SUCCESS"}]}`,
		"pending": `{"headRefOid":"h","state":"OPEN","mergeCommit":null,"statusCheckRollup":[
{"__typename":"CheckRun","name":"build","status":"IN_PROGRESS","conclusion":""}]}`,
		"statusContext failure": `{"headRefOid":"h","state":"OPEN","mergeCommit":null,"statusCheckRollup":[
{"__typename":"StatusContext","context":"x","state":"FAILURE"}]}`,
		"no checks": `{"headRefOid":"h","state":"OPEN","mergeCommit":null,"statusCheckRollup":[]}`,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			fr := &fakeRunner{resps: []fakeResp{{Stdout: payload}}}
			res := (&GitHub{Runner: fr, Repo: "o/r"}).CIEvidence(context.Background(), 1)
			assert.Equal(t, StatusVerified, res.Status, res.Reason)
			assert.False(t, res.AllPassed)
			assert.False(t, res.Merged)
			assert.Empty(t, res.MergeSHA)
		})
	}
}

func TestU159002_TokenNotInArgvAndOutageDegrades(t *testing.T) {
	const tok = "ghp_abcdefghijklmnopqrstuvwxyz0123456789"
	fr := &fakeRunner{resps: []fakeResp{{
		Stderr: "dial tcp: Authorization: token " + tok,
		Err:    errors.New("exit status 4"),
	}}}
	res := (&GitHub{Runner: fr, Repo: "o/r", Token: tok}).CIEvidence(context.Background(), 3)

	require.Len(t, fr.calls, 1)
	assert.NotContains(t, fr.allArgs(), tok)
	assert.Contains(t, fr.calls[0].Env, "GH_TOKEN="+tok)
	assert.Equal(t, StatusDegraded, res.Status)
	assert.NotContains(t, res.Reason, tok)
	assert.False(t, res.AllPassed)
	assert.False(t, res.Merged)
}

func TestU159002_MalformedAndBadRepoDegrade(t *testing.T) {
	fr := &fakeRunner{resps: []fakeResp{{Stdout: "<html>"}}}
	assert.Equal(t, StatusDegraded, (&GitHub{Runner: fr, Repo: "o/r"}).CIEvidence(context.Background(), 1).Status)
	assert.Equal(t, StatusDegraded, (&GitHub{Runner: fr, Repo: "bad"}).CIEvidence(context.Background(), 1).Status)
	empty := &fakeRunner{resps: []fakeResp{{Stdout: `{}`}}}
	assert.Equal(t, StatusDegraded, (&GitHub{Runner: empty, Repo: "o/r"}).CIEvidence(context.Background(), 1).Status)
}

func TestU159002_MergedWithoutMergeSHADegrades(t *testing.T) {
	fr := &fakeRunner{resps: []fakeResp{{Stdout: `{"headRefOid":"h","state":"MERGED","mergeCommit":null,"statusCheckRollup":[]}`}}}
	res := (&GitHub{Runner: fr, Repo: "o/r"}).CIEvidence(context.Background(), 1)
	assert.Equal(t, StatusDegraded, res.Status)
}
