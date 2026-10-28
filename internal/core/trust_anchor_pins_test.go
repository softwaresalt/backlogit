package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

type fakePinProvider struct {
	set *core.ExternalPinSet
	err error
}

func (f fakePinProvider) ExternalPinSet(context.Context) (*core.ExternalPinSet, error) {
	return f.set, f.err
}

func pinAnchors() []config.TrustAnchor {
	return []config.TrustAnchor{
		{ID: "a1", Algo: "ed25519", Fingerprint: "sha256:aa", Status: config.TrustAnchorStatusActive},
		{ID: "a2", Algo: "ed25519", Fingerprint: "sha256:bb", Status: config.TrustAnchorStatusActive},
	}
}

func TestTrustAnchorExternalPinGate(t *testing.T) {
	ctx := context.Background()

	t.Run("all fingerprints pinned passes", func(t *testing.T) {
		p := fakePinProvider{set: core.NewExternalPinSet("sha256:aa", "sha256:bb", "sha256:cc")}
		require.NoError(t, core.VerifyTrustAnchorPins(ctx, pinAnchors(), p))
	})

	t.Run("unpinned fingerprint rejected with typed error", func(t *testing.T) {
		p := fakePinProvider{set: core.NewExternalPinSet("sha256:aa")}
		err := core.VerifyTrustAnchorPins(ctx, pinAnchors(), p)
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned)
		var pe *core.TrustAnchorPinError
		require.True(t, errors.As(err, &pe))
		require.Equal(t, "a2", pe.ID)
		require.Equal(t, 1, pe.Index)
	})

	t.Run("empty fingerprint is never pinned", func(t *testing.T) {
		p := fakePinProvider{set: core.NewExternalPinSet("", "sha256:aa")}
		err := core.VerifyTrustAnchorPins(ctx, []config.TrustAnchor{{ID: "x"}}, p)
		require.ErrorIs(t, err, core.ErrTrustAnchorNotPinned)
	})

	// Negative: absent pin set => feature disabled => fail closed.
	t.Run("absent pin set fails closed", func(t *testing.T) {
		cases := map[string]core.ExternalPinSetProvider{
			"nil provider":     nil,
			"nil set":          fakePinProvider{},
			"provider absent":  fakePinProvider{err: core.ErrTrustPinSetAbsent},
			"provider failure": fakePinProvider{err: errors.New("boom")},
		}
		for name, p := range cases {
			t.Run(name, func(t *testing.T) {
				err := core.VerifyTrustAnchorPins(ctx, pinAnchors(), p)
				require.Error(t, err)
				require.ErrorIs(t, err, core.ErrTrustPinSetAbsent)
			})
		}
	})

	t.Run("absent pin set fails closed even with no anchors", func(t *testing.T) {
		err := core.VerifyTrustAnchorPins(ctx, nil, nil)
		require.ErrorIs(t, err, core.ErrTrustPinSetAbsent)
	})
}
