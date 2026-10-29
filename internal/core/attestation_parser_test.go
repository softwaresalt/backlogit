package core_test

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

const attWellFormedStatement = `{"_type":"https://in-toto.io/Statement/v1","subject":[{"name":"150-S","digest":{"sha256":"abc"}}],"predicateType":"https://backlogit.dev/delivery-evidence/v1","predicate":{"shipment_id":"150-S","merge_sha":"deadbeef","manifest_digest":"m1","closure_content_hash":"c1","signer_identity":"ci@example","issued_at":"2026-06-01T00:00:00Z","expires_at":"2026-07-01T00:00:00Z"}}`

func attEnvelopeJSON(statement string) []byte {
	payload := base64.StdEncoding.EncodeToString([]byte(statement))
	return []byte(`{"payloadType":"application/vnd.in-toto+json","payload":"` + payload + `","signatures":[{"keyid":"att","sig":"c2ln","sig_algo":"ed25519"}]}`)
}

func TestAttestationEnvelopeParser(t *testing.T) {
	t.Run("well-formed envelope parses", func(t *testing.T) {
		got, err := core.ParseAttestation(attEnvelopeJSON(attWellFormedStatement))
		require.NoError(t, err)
		require.Equal(t, "application/vnd.in-toto+json", got.Envelope.PayloadType)
		require.Len(t, got.Envelope.Signatures, 1)
		require.Equal(t, "att", got.Envelope.Signatures[0].KeyID)
		require.Equal(t, "ed25519", got.Envelope.Signatures[0].Algo)
		require.Equal(t, attWellFormedStatement, string(got.PayloadBytes))
		require.Equal(t, "150-S", got.Statement.Predicate.ShipmentID)
		require.Equal(t, "deadbeef", got.Statement.Predicate.MergeSHA)
		require.Equal(t, "m1", got.Statement.Predicate.ManifestDigest)
		require.Equal(t, "c1", got.Statement.Predicate.ClosureContentHash)
		require.Equal(t, "abc", got.Statement.Subject[0].Digest["sha256"])
	})

	t.Run("malformed envelope is rejected with a typed error", func(t *testing.T) {
		cases := map[string][]byte{
			"empty":              nil,
			"not json":           []byte("not json"),
			"truncated":          []byte(`{"payloadType":`),
			"trailing data":      append(attEnvelopeJSON(attWellFormedStatement), []byte(` {}`)...),
			"payload not base64": []byte(`{"payloadType":"t","payload":"!!!","signatures":[]}`),
			"payload not json":   attEnvelopeJSON("not json"),
			"wrong field type":   []byte(`{"payloadType":1,"payload":"","signatures":[]}`),
		}
		for name, data := range cases {
			t.Run(name, func(t *testing.T) {
				_, err := core.ParseAttestation(data)
				require.ErrorIs(t, err, core.ErrAttestationMalformed)
				var pe *core.AttestationParseError
				require.ErrorAs(t, err, &pe)
			})
		}
	})

	t.Run("exact-byte duplicate member is rejected at every depth", func(t *testing.T) {
		cases := []struct {
			name string
			data []byte
			path string
		}{
			{
				name: "envelope root",
				data: []byte(`{"payloadType":"a","payloadType":"b","payload":"","signatures":[]}`),
				path: "envelope",
			},
			{
				name: "statement",
				data: attEnvelopeJSON(`{"_type":"a","_type":"b","subject":[],"predicateType":"p","predicate":{}}`),
				path: "statement",
			},
			{
				name: "subject",
				data: attEnvelopeJSON(`{"_type":"a","subject":[{"name":"x","name":"y","digest":{}}],"predicateType":"p","predicate":{}}`),
				path: "statement.subject[0]",
			},
			{
				name: "predicate",
				data: attEnvelopeJSON(`{"_type":"a","subject":[],"predicateType":"p","predicate":{"merge_sha":"a","merge_sha":"b"}}`),
				path: "statement.predicate",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := core.ParseAttestation(tc.data)
				require.ErrorIs(t, err, core.ErrAttestationDuplicateMember)
				var pe *core.AttestationParseError
				require.ErrorAs(t, err, &pe)
				require.Equal(t, tc.path, pe.Path)
			})
		}
	})
}
