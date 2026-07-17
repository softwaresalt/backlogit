package docline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// schemaContractRepoRoot resolves the repository root from this test file's
// location so the JSON schema files can be read regardless of the working
// directory the test runner uses.
func schemaContractRepoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "resolve repo root: runtime caller unavailable")

	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// readSchemaJSON reads and decodes a repo-relative JSON schema file into a
// generic map for structural assertions.
func readSchemaJSON(t *testing.T, relativePath string) map[string]any {
	t.Helper()

	absPath := filepath.Join(schemaContractRepoRoot(t), filepath.FromSlash(relativePath))
	data, err := os.ReadFile(absPath) //nolint:gosec // repo-relative test fixture path
	require.NoError(t, err, "read schema %s", relativePath)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc), "decode schema %s", relativePath)
	return doc
}

const (
	baseSchemaPath = "schemas/docline/base-frontmatter-v1.schema.json"
	extSchemaPath  = "schemas/docline/ext/backlogit-v1.schema.json"
	baseSchemaID   = "https://docline.softwaresalt.dev/schema/base-frontmatter/v1.json"
)

// TestBaseSchemaTopLevelStaysClosed pins Model A: the docline base contract top
// level remains additionalProperties:false (the base schema is NOT opened,
// reversing Model-B task 107.011-T).
func TestBaseSchemaTopLevelStaysClosed(t *testing.T) {
	base := readSchemaJSON(t, baseSchemaPath)

	addProps, ok := base["additionalProperties"]
	require.True(t, ok, "base schema must declare top-level additionalProperties")
	assert.Equal(t, false, addProps, "base top-level additionalProperties must stay false")

	// The docline namespace is the sanctioned extension bag (open map).
	doclineProp, ok := base["properties"].(map[string]any)["docline"].(map[string]any)
	require.True(t, ok, "base schema must define a docline property")
	assert.Contains(t, doclineProp, "anyOf", "docline remains the open extension namespace")

	assert.Equal(t, baseSchemaID, base["$id"], "base schema $id is the composition anchor")
}

// TestExtSchemaComposesBaseV1 pins that the layered extension schema exists and
// allOf-composes the docline base-frontmatter v1 contract.
func TestExtSchemaComposesBaseV1(t *testing.T) {
	ext := readSchemaJSON(t, extSchemaPath)

	allOf, ok := ext["allOf"].([]any)
	require.True(t, ok, "ext schema must declare an allOf composition")
	require.NotEmpty(t, allOf, "ext schema allOf must reference the base contract")

	first, ok := allOf[0].(map[string]any)
	require.True(t, ok, "ext schema allOf[0] must be a schema object")
	assert.Equal(t, baseSchemaID, first["$ref"],
		"ext schema must allOf-compose docline base-frontmatter v1")
}

// TestExtSchemaConstrainsOnlyDoclineBacklogit pins that the extension schema
// scopes its own constraints to the docline.backlogit subtree only: it does not
// redefine top-level contract fields and does not re-open the closed top level.
func TestExtSchemaConstrainsOnlyDoclineBacklogit(t *testing.T) {
	ext := readSchemaJSON(t, extSchemaPath)

	// The ext schema must not re-open the closed base top level.
	if addProps, ok := ext["additionalProperties"]; ok {
		assert.NotEqual(t, true, addProps,
			"ext schema must not re-open the closed base top level")
	}

	props, ok := ext["properties"].(map[string]any)
	require.True(t, ok, "ext schema must declare properties")
	assert.Equal(t, []string{"docline"}, sortedKeys(props),
		"ext schema must constrain only the docline namespace (no top-level contract-field redefinition)")

	doclineProp, ok := props["docline"].(map[string]any)
	require.True(t, ok, "ext schema docline property must be an object schema")

	doclineProps, ok := doclineProp["properties"].(map[string]any)
	require.True(t, ok, "ext schema must declare docline.properties")
	assert.Equal(t, []string{"backlogit"}, sortedKeys(doclineProps),
		"ext schema must constrain only the docline.backlogit owner subtree")

	backlogit, ok := doclineProps["backlogit"].(map[string]any)
	require.True(t, ok, "docline.backlogit must be an object schema")
	assert.Contains(t, backlogit, "properties", "docline.backlogit owner profile declares properties")

	backlogitProps, ok := backlogit["properties"].(map[string]any)
	require.True(t, ok, "docline.backlogit.properties must be a map")
	assert.Contains(t, backlogitProps, "schema_version",
		"docline.backlogit carries a per-owner schema_version")
}

// sortedKeys returns the map keys in deterministic order for stable assertions.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
