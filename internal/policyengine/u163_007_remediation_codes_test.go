package policyengine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestU163_007_TimestampInTheFutureIsRejectedWithItsOwnCode(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	m := e.good()
	m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.t0.Add(48*time.Hour), OutcomePass, nil)
	d := check(t, e.engine(), e.request(m))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeGreenPhaseVerify, CodeTimestampFuture), "%+v", d.Errors)
}

func TestU163_007_TimestampBeforeItsCommitIsRejectedWithItsOwnCode(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	m := e.good()
	m[NodeGreenPhaseVerify] = e.rec(NodeGreenPhaseVerify, e.c[3], e.at(0, 5), OutcomePass, nil)
	d := check(t, e.engine(), e.request(m))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeGreenPhaseVerify, CodeTimestampRetroactive), "%+v", d.Errors)
}

func TestU163_007_SuccessorRecordedBeforeItsPredecessorIsOutOfOrder(t *testing.T) {
	e := newEnv(t, enforceRegistry)
	m := e.good()
	// Both records are individually valid (after their own commits); only
	// their relative recording order is wrong.
	m[NodeHarnessScaffold] = e.rec(NodeHarnessScaffold, e.c[1], e.at(2, 50), OutcomePass, nil)
	m[NodeRedPhaseVerify] = e.rec(NodeRedPhaseVerify, e.c[2], e.at(2, 30), OutcomePass, e.redArtifact(e.c[2]))
	d := check(t, e.engine(), e.request(m))
	assert.False(t, d.Allowed)
	assert.True(t, hasViolation(d, NodeRedPhaseVerify, CodeOutOfOrder), "%+v", d.Errors)
	assert.False(t, hasViolation(d, NodeRedPhaseVerify, CodeTimestampRetroactive), "must be the ordering defect, not a timestamp one")
}
