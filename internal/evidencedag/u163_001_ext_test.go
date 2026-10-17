package evidencedag

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type strictDoc struct {
	SchemaVersion int      `json:"schema_version" yaml:"schema_version"`
	Names         []string `json:"names" yaml:"names"`
}

func TestU163_001_FamilyWorkflowPolicyIsKnown(t *testing.T) {
	assert.True(t, knownFamily(FamilyWorkflowPolicy))
	assert.Contains(t, Families(), FamilyWorkflowPolicy)
}

func TestU163_001_DecodeStrictAcceptsJSONAndYAML(t *testing.T) {
	var j, y strictDoc
	require.NoError(t, DecodeStrict([]byte(`{"schema_version":1,"names":["a"]}`), &j))
	require.NoError(t, DecodeStrict([]byte("schema_version: 1\nnames: [a]\n"), &y))
	assert.Equal(t, j, y)
}

func TestU163_001_DecodeStrictRejectsAmbiguousDocuments(t *testing.T) {
	cases := map[string]string{
		"empty":            "",
		"unknown field":    `{"schema_version":1,"names":[],"extra":1}`,
		"duplicate key":    `{"schema_version":1,"schema_version":1,"names":[]}`,
		"fold duplicate":   "{\"schema_version\":1,\"names\":[],\"NAMES\":[]}",
		"trailing data":    `{"schema_version":1,"names":[]} {}`,
		"yaml anchor":      "schema_version: 1\nnames: &a [x]\n",
		"yaml two docs":    "schema_version: 1\nnames: []\n---\nschema_version: 1\n",
		"yaml merge key":   "base: &b {schema_version: 1}\nnames: []\n<<: *b\n",
		"oversized":        `{"names":["` + strings.Repeat("a", MaxDeclarationBytes) + `"]}`,
		"yaml dup key":     "schema_version: 1\nschema_version: 1\nnames: []\n",
		"yaml unknown key": "schema_version: 1\nnames: []\nextra: 1\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			var d strictDoc
			err := DecodeStrict([]byte(doc), &d)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidDeclaration)
		})
	}
}

func TestU163_001_DecodeStrictRejectsNilTarget(t *testing.T) {
	require.Error(t, DecodeStrict([]byte(`{}`), nil))
}
