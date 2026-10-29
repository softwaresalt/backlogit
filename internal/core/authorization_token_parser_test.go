package core_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

const tokenParserValidJSON = `{"header":{"alg":"ed25519","key_id":"op-1"},` +
	`"claims":{"not_before":"2026-01-01T00:00:00Z","not_after":"2026-01-01T00:05:00Z","nonce":"00ff"},` +
	`"binding":{"shipment_id":"151-S","request_identity_digest":"abc123"},"signature":"c2ln"}`

func TestTokenParser(t *testing.T) {
	t.Run("well-formed token parses into the typed structure", func(t *testing.T) {
		tok, err := core.ParseAuthorizationToken([]byte(tokenParserValidJSON))
		require.NoError(t, err)
		require.Equal(t, "ed25519", tok.Header.Algo)
		require.Equal(t, "op-1", tok.Header.KeyID)
		require.True(t, tok.Claims.NotBefore.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
		require.True(t, tok.Claims.NotAfter.Equal(time.Date(2026, 1, 1, 0, 5, 0, 0, time.UTC)))
		require.Equal(t, "00ff", tok.Claims.Nonce)
		require.Equal(t, "151-S", tok.Binding.ShipmentID)
		require.Equal(t, "abc123", tok.Binding.RequestIdentityDigest)
		require.Equal(t, "c2ln", tok.Signature)
	})

	malformed := []struct{ name, data string }{
		{"empty input", ``},
		{"not JSON", `not json`},
		{"truncated", `{"header":{"alg":"ed25519"`},
		{"array root", `[]`},
		{"trailing data", tokenParserValidJSON + ` {}`},
		{"claims wrong type", `{"header":{},"claims":"x","binding":{},"signature":""}`},
		{"bad timestamp", `{"header":{},"claims":{"not_before":"yesterday"},"binding":{},"signature":""}`},
	}
	for _, tc := range malformed {
		t.Run("malformed/"+tc.name, func(t *testing.T) {
			_, err := core.ParseAuthorizationToken([]byte(tc.data))
			require.ErrorIs(t, err, core.ErrTokenMalformed)
			var pe *core.TokenParseError
			require.ErrorAs(t, err, &pe)
		})
	}

	// Exact-byte duplicate members at every concrete object-bearing depth.
	duplicates := []struct{ name, path, data string }{
		{"root", "token", `{"signature":"a","signature":"b","header":{},"claims":{},"binding":{}}`},
		{"header", "token.header", `{"header":{"alg":"a","alg":"b"},"claims":{},"binding":{},"signature":""}`},
		{"claims", "token.claims", `{"header":{},"claims":{"nonce":"a","nonce":"b"},"binding":{},"signature":""}`},
		{"binding", "token.binding", `{"header":{},"claims":{},"binding":{"shipment_id":"a","shipment_id":"b"},"signature":""}`},
	}
	for _, tc := range duplicates {
		t.Run("exact duplicate/"+tc.name, func(t *testing.T) {
			_, err := core.ParseAuthorizationToken([]byte(tc.data))
			require.ErrorIs(t, err, core.ErrTokenDuplicateMember)
			var pe *core.TokenParseError
			require.ErrorAs(t, err, &pe)
			require.Equal(t, tc.path, pe.Path)
		})
	}
}
