package core

import (
	"context"
	stderrors "errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestHeadDriftError pins the stable-head assertion: equal pre/post heads (including
// the both-empty no-repo case) report no drift (nil); any difference is a typed
// *GateBlockedError naming both observed heads, so a HEAD advance across the gate
// evaluation window fails closed.
func TestHeadDriftError(t *testing.T) {
	assert.NoError(t, headDriftError("084-S", "abc", "abc"), "equal heads: no drift")
	assert.NoError(t, headDriftError("084-S", "", ""), "both empty (no-repo): no drift, guard inert")

	err := headDriftError("084-S", "aaaaaaa", "bbbbbbb")
	require.Error(t, err, "differing heads must fail closed")
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "drift refusal must be a *GateBlockedError")
	assert.Contains(t, err.Error(), "aaaaaaa")
	assert.Contains(t, err.Error(), "bbbbbbb")
}

// TestHeadResolveError pins the fail-closed head-resolution refusal: a non-nil
// bounded-read (timeout/cancel) error becomes a typed *GateBlockedError so a
// shipment head that cannot be read under the deadline blocks completion rather
// than silently skipping the staleness guard.
func TestHeadResolveError(t *testing.T) {
	err := headResolveError("084-S", context.DeadlineExceeded)
	require.Error(t, err)
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "resolve refusal must be a *GateBlockedError")
	assert.Contains(t, err.Error(), "084-S")
}

// TestShipmentHeadUnresolvedInRepoError pins the dedicated 1AEA2B0E refusal: an
// empty shipment head inside a real work tree becomes a typed *GateBlockedError
// with an operator-assertable message distinct from the bounded-read timeout
// (headResolveError) path.
func TestShipmentHeadUnresolvedInRepoError(t *testing.T) {
	err := shipmentHeadUnresolvedInRepoError("085-S")
	require.Error(t, err)
	var blocked *bkerrors.GateBlockedError
	require.True(t, stderrors.As(err, &blocked), "in-repo unresolved-head refusal must be a *GateBlockedError")
	assert.Contains(t, err.Error(), "085-S")
	assert.Contains(t, err.Error(), "cannot resolve shipment head in repository")
}

// TestHeadSHABounded pins the bounded-read distinction that keeps the new timeout
// path fail-closed while preserving the legacy non-context empty skip:
//   - an already-expired parent context -> ("", ctxErr): a bounded-read failure
//     the caller MUST fail closed on.
//   - a live context in a non-repo dir -> ("", nil): the legacy resolution-failure
//     skip is preserved so existing no-repo shipment tests do not regress.
func TestHeadSHABounded(t *testing.T) {
	ws := newGateTestWorkspace(t) // temp dir, NOT a git repo

	// Expired parent context -> fail closed with a non-nil ctx error.
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Hour))
	defer cancel()
	h, err := ws.headSHABounded(expired)
	require.Error(t, err, "an expired bounded read must return a non-nil error (fail closed)")
	assert.Empty(t, h)
	assert.True(t, stderrors.Is(err, context.DeadlineExceeded), "want DeadlineExceeded, got %v", err)

	// Live context, non-repo dir -> legacy skip preserved: ("", nil).
	h, err = ws.headSHABounded(context.Background())
	require.NoError(t, err, "a legacy non-context resolution failure must stay a silent skip")
	assert.Empty(t, h)
}
