package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

// U1 (079.001-T): `backlogit link add|remove|list` must mirror the MCP
// add_link/remove_link/get_links tools over the shared core path
// (core.AddArtifactLink / core.RemoveArtifactLink / extracted core.GetLinks).
// Success JSON shapes must be isomorphic to the MCP handler results and the
// list `links` field must always be an array (never null, Rule 3).

// Scenario 1: add happy-path → {source_id, target_id, link_type} parity with
// handleAddLink.
func TestLinkAdd_HappyPath_OutputShapeParity(t *testing.T) {
	root := setupCLIWorkspace(t)
	src := cliAddFeature(t, root, "Link source")
	tgt := cliAddFeature(t, root, "Link target")

	var m map[string]any
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "link", "add", src, tgt, "related_to")), &m))
	assert.Equal(t, src, m["source_id"])
	assert.Equal(t, tgt, m["target_id"])
	assert.Equal(t, "related_to", m["link_type"])
}

// Scenario 2: remove → {source_id, target_id, link_type, status:"removed"}
// parity with handleRemoveLink.
func TestLinkRemove_OutputShapeParity(t *testing.T) {
	root := setupCLIWorkspace(t)
	src := cliAddFeature(t, root, "Rm source")
	tgt := cliAddFeature(t, root, "Rm target")
	_ = runCLIStdout(t, root, "link", "add", src, tgt, "related_to")

	var m map[string]any
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "link", "remove", src, tgt, "related_to")), &m))
	assert.Equal(t, src, m["source_id"])
	assert.Equal(t, tgt, m["target_id"])
	assert.Equal(t, "related_to", m["link_type"])
	assert.Equal(t, "removed", m["status"])
}

// Scenario 3a: list on an item with no links → {id, links: []} where links is an
// empty array, never null (Rule 3, inherited from core.GetLinks normalization).
func TestLinkList_EmptyIsArrayNotNull(t *testing.T) {
	root := setupCLIWorkspace(t)
	src := cliAddFeature(t, root, "Empty source")

	var payload struct {
		ID    string           `json:"id"`
		Links []map[string]any `json:"links"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "link", "list", src)), &payload))
	assert.Equal(t, src, payload.ID)
	require.NotNil(t, payload.Links, "links must decode to [] (non-nil), not null (Rule 3)")
	assert.Empty(t, payload.Links)
}

// Scenario 3b: list returns all links; --type filters to matching link_type,
// mirroring handleGetLinks with a link_type argument.
func TestLinkList_ReturnsLinks_AndTypeFilter(t *testing.T) {
	root := setupCLIWorkspace(t)
	src := cliAddFeature(t, root, "List source")
	tgt1 := cliAddFeature(t, root, "List target 1")
	tgt2 := cliAddFeature(t, root, "List target 2")
	_ = runCLIStdout(t, root, "link", "add", src, tgt1, "related_to")
	_ = runCLIStdout(t, root, "link", "add", src, tgt2, "duplicate_of")

	var all struct {
		ID    string `json:"id"`
		Links []struct {
			TargetID string `json:"target_id"`
			LinkType string `json:"link_type"`
		} `json:"links"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "link", "list", src)), &all))
	assert.Equal(t, src, all.ID)
	require.Len(t, all.Links, 2)

	var filtered struct {
		Links []struct {
			LinkType string `json:"link_type"`
		} `json:"links"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(runCLIStdout(t, root, "link", "list", src, "--type", "related_to")), &filtered))
	require.Len(t, filtered.Links, 1, "--type must filter to matching links only")
	assert.Equal(t, "related_to", filtered.Links[0].LinkType)
}

// U1: the link group and its subcommands are registered and discoverable.
func TestLink_RegisteredUnderRoot(t *testing.T) {
	root := cli.NewRootCommand()
	var linkCmd *cobra.Command
	for _, sub := range root.Commands() {
		if sub.Name() == "link" {
			linkCmd = sub
			break
		}
	}
	require.NotNil(t, linkCmd, "root must register the link command group")

	names := make([]string, 0)
	for _, sub := range linkCmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "add")
	assert.Contains(t, names, "remove")
	assert.Contains(t, names, "list")
}
