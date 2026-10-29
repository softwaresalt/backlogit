package core_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestAttestationJSONEquivDuplicateAllDepths(t *testing.T) {
	// Each pair names the same member after decoding: by case folding, by a
	// \uXXXX escape, or by both.
	pairs := []struct{ name, first, second string }{
		{"case-fold", `"merge_sha"`, `"MERGE_SHA"`},
		{"unicode-escape", `"merge_sha"`, `"merge\u005fsha"`},
		{"mixed case-fold and escape", `"MERGE_SHA"`, `"merge\u005fsha"`},
	}
	depths := []struct {
		name string
		path string
		data func(a, b string) []byte
	}{
		{"envelope root", "envelope", func(a, b string) []byte {
			return []byte(`{` + a + `:"x",` + b + `:"y","payloadType":"t","payload":"","signatures":[]}`)
		}},
		{"statement", "statement", func(a, b string) []byte {
			return attEnvelopeJSON(`{` + a + `:"x",` + b + `:"y","_type":"t","subject":[],"predicateType":"p","predicate":{}}`)
		}},
		{"subject", "statement.subject[0]", func(a, b string) []byte {
			return attEnvelopeJSON(`{"_type":"t","subject":[{` + a + `:"x",` + b + `:"y"}],"predicateType":"p","predicate":{}}`)
		}},
		{"predicate", "statement.predicate", func(a, b string) []byte {
			return attEnvelopeJSON(`{"_type":"t","subject":[],"predicateType":"p","predicate":{` + a + `:"x",` + b + `:"y"}}`)
		}},
	}
	for _, pair := range pairs {
		for _, depth := range depths {
			t.Run(pair.name+"/"+depth.name, func(t *testing.T) {
				_, err := core.ParseAttestation(depth.data(pair.first, pair.second))
				require.ErrorIs(t, err, core.ErrAttestationDuplicateMember)
				var pe *core.AttestationParseError
				require.ErrorAs(t, err, &pe)
				require.Equal(t, depth.path, pe.Path)
			})
		}
	}
}
