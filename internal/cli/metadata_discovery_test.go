package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

// U5 (079.005-T): `backlogit metadata types|wit|templates` must mirror the MCP
// list_types / get_wit_metadata / list_templates tools over the shared core /
// templates functions. Array outputs are never null (Rule 3).

// Scenario 1: `metadata types` returns a JSON array of WIT metadata (never null),
// mirroring handleListTypes.
func TestMetadataTypes_ReturnsTypeArray(t *testing.T) {
	root := setupCLIWorkspace(t)
	var types []struct {
		TypeName       string `json:"type"`
		HierarchyLevel int    `json:"hierarchy_level"`
	}
	require.NoError(t, json.Unmarshal([]byte(runCLIStdout(t, root, "metadata", "types")), &types))
	require.NotNil(t, types, "types must decode to an array, never null (Rule 3)")

	names := map[string]bool{}
	for _, ty := range types {
		names[ty.TypeName] = true
	}
	assert.True(t, names["feature"], "default types must include feature")
	assert.True(t, names["task"], "default types must include task")
	assert.True(t, names["subtask"], "default types must include subtask")
}

// Scenario 2: `metadata wit <type>` returns the WIT metadata object for a known
// type and surfaces an error for an unknown type, mirroring handleGetWITMetadata.
func TestMetadataWit_KnownAndUnknownType(t *testing.T) {
	root := setupCLIWorkspace(t)
	var meta struct {
		TypeName string         `json:"type"`
		Fields   map[string]any `json:"fields"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "metadata", "wit", "feature")), &meta))
	assert.Equal(t, "feature", meta.TypeName)

	require.Error(t, runCLIErr(t, root, "metadata", "wit", "nonexistent-type"),
		"an unknown artifact type must error, mirroring handleGetWITMetadata")
}

// Scenario 3: `metadata templates` returns a JSON array (never null), mirroring
// handleListTemplates.
func TestMetadataTemplates_ReturnsArray(t *testing.T) {
	root := setupCLIWorkspace(t)
	var templates []struct {
		TypeName string `json:"type"`
	}
	require.NoError(t, json.Unmarshal([]byte(runCLIStdout(t, root, "metadata", "templates")), &templates))
	require.NotNil(t, templates, "templates must decode to an array, never null (Rule 3)")
	assert.NotEmpty(t, templates, "default workspace declares templates")
}

// U5: the discovery subcommands are registered under the metadata group.
func TestMetadataDiscovery_RegisteredUnderMetadata(t *testing.T) {
	cwd := "."
	cmd := cli.NewMetadataCmd(&cwd)
	names := make([]string, 0)
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "types")
	assert.Contains(t, names, "wit")
	assert.Contains(t, names, "templates")
}
