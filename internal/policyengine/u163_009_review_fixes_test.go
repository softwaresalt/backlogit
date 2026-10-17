package policyengine

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU163_009_DigestSurvivesBundleRoundTripWithHTMLAndLineSeparators(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	artifact := json.RawMessage("{\"note\":\"a&b <c> \u2028 \u2029 d\"}")
	rec := e.rec(NodeHarnessScaffold, e.c[1], e.at(1, 30), OutcomePass, artifact)

	again := e.rec(NodeHarnessScaffold, e.c[1], e.at(1, 30), OutcomePass, artifact)
	assert.Equal(t, rec.Digest, again.Digest, "digest is deterministic")

	data, err := json.Marshal(Bundle{SchemaVersion: EvidenceSchemaVersion, TaskID: e.task, HeadCommit: e.c[4], Records: []Record{rec}})
	require.NoError(t, err)
	b, err := DecodeBundle(data)
	require.NoError(t, err)
	require.Len(t, b.Records, 1)

	want, err := b.Records[0].computeDigest()
	require.NoError(t, err)
	assert.Equal(t, b.Records[0].Digest, want)
	assert.Equal(t, rec.Digest, want, "digest is stable across the round trip")
}

func TestU163_009_GreenRecordedAfterMergeReadyBlocksThroughDerivedGates(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	m := e.good()
	m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.at(5, 30), OutcomePass, nil)
	m[NodeMergeReady] = e.rec(NodeMergeReady, e.c[4], e.at(4, 30), OutcomePass, nil)
	d := check(t, e.engine(), e.request(m))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeMergeReady, CodeOutOfOrder), "%+v", d.Errors)
}

func TestU163_009_InOrderChainThroughDerivedGatesStillPasses(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	m := e.good()
	m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.at(4, 30), OutcomePass, nil)
	d := check(t, e.engine(), e.request(m))
	assert.True(t, d.Allowed, "%+v", d.Errors)
}
