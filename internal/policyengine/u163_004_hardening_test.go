package policyengine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU163_004_HardeningCRLFInputsLoadLikeLF(t *testing.T) {
	crlf := strings.ReplaceAll(canonicalRegistryYAML, "\n", "\r\n")
	r, err := LoadRegistry([]byte(crlf))
	require.NoError(t, err)
	assert.Len(t, r.Gates(), 6)

	e := newEnv(t, enforceRegistry)
	req := e.request(e.good())
	raw, err := json.MarshalIndent(Bundle{SchemaVersion: EvidenceSchemaVersion, TaskID: req.TaskID, HeadCommit: req.HeadCommit, PR: req.PR, Records: req.Records}, "", "  ")
	require.NoError(t, err)
	b, err := DecodeBundle([]byte(strings.ReplaceAll(string(raw), "\n", "\r\n")))
	require.NoError(t, err)
	d := check(t, e.engine(), b.Request(SurfaceCLI))
	assert.True(t, d.Allowed, "%+v", d.Errors)
}

func TestU163_004_HardeningHostileRecordsBlockWithoutPanicking(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	good := e.good()
	// Every mutation is re-digested (unless the digest is the hostile part), so
	// the record is rejected for its own defect and not for a stale digest.
	hostile := map[string]struct {
		mut      func(*Record)
		redigest bool
		code     string
	}{
		"huge artifact": {func(r *Record) {
			r.Artifact = json.RawMessage(`"` + strings.Repeat("a", MaxArtifactBytes) + `"`)
		}, true, CodeEvidenceInvalid},
		"null artifact":      {func(r *Record) { r.Artifact = json.RawMessage(`null`) }, true, CodeEvidenceInvalid},
		"truncated artifact": {func(r *Record) { r.Artifact = json.RawMessage(`{"a":`) }, true, CodeEvidenceInvalid},
		"far future":         {func(r *Record) { r.RecordedAt = time.Date(2199, 1, 1, 0, 0, 0, 0, time.UTC) }, true, CodeTimestampFuture},
		"far past":           {func(r *Record) { r.RecordedAt = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC) }, true, CodeEvidenceInvalid},
		"control characters": {func(r *Record) { r.Node = NodeType("green-phase-verify\x00\x1b[31m") }, true, CodeInvalidRequest},
		"empty digest":       {func(r *Record) { r.Digest = "" }, false, CodeDigestMismatch},
		"digest of zeros":    {func(r *Record) { r.Digest = strings.Repeat("0", 64) }, false, CodeDigestMismatch},
	}
	for name, tc := range hostile {
		t.Run(name, func(t *testing.T) {
			recs := map[NodeType]Record{}
			for k, v := range good {
				recs[k] = v
			}
			r := recs[NodeGreenPhaseVerify]
			tc.mut(&r)
			if tc.redigest {
				if d, err := r.computeDigest(); err == nil {
					r.Digest = d
				}
			}
			recs[NodeGreenPhaseVerify] = r
			req := e.request(recs)
			assert.NotPanics(t, func() {
				d, _ := eng.Check(context.Background(), req)
				assert.False(t, d.Allowed)
				assert.NotEqual(t, DispositionPass, d.Disposition)
				require.NotEmpty(t, d.Errors)
				codes := make([]string, 0, len(d.Errors))
				for _, v := range d.Errors {
					codes = append(codes, v.Code)
					assert.LessOrEqual(t, len(v.Reason), MaxErrorBytes)
					assert.NotContains(t, v.Reason, "\x1b")
				}
				assert.Contains(t, codes, tc.code, "%+v", d.Errors)
			})
		})
	}
}

func TestU163_004_HardeningRecordForADisabledGateIsInert(t *testing.T) {
	doc := strings.Replace(enforceRegistry, "{id: green-phase-verify, enabled: true", "{id: green-phase-verify, enabled: false", 1)
	e := newEnv(t, doc)
	m := e.good()
	m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, sha(999), e.at(3, 30), OutcomePass, nil) // fabricated, but the gate is off
	d := check(t, e.engine(), e.request(m))
	assert.True(t, d.Allowed, "%+v", d.Errors)
	assert.Equal(t, "not-applicable", gateStatus(d, NodeGreenPhaseVerify))
}

func TestU163_004_HardeningManyConcurrentChecksAreIndependent(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	eng := e.engine()
	okReq := e.request(e.good())
	bad := e.good()
	delete(bad, NodeHarnessScaffold)
	badReq := e.request(bad)
	done := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		go func(i int) {
			req := okReq
			want := true
			if i%2 == 1 {
				req, want = badReq, false
			}
			d, err := eng.Check(context.Background(), req)
			done <- err == nil && d.Allowed == want
		}(i)
	}
	for i := 0; i < 16; i++ {
		assert.True(t, <-done)
	}
}
