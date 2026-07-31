package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/mdfront"
)

func TestUpdateComplexity_PersistsAndPreservesBody(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)
	path := taskFilePath(root, id)

	rawBefore, err := os.ReadFile(path)
	require.NoError(t, err)
	mdBefore, err := mdfront.Decode(rawBefore)
	require.NoError(t, err)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--complexity", "high"})
	require.NoError(t, cmd.Execute())

	rawAfter, err := os.ReadFile(path)
	require.NoError(t, err)
	mdAfter, err := mdfront.Decode(rawAfter)
	require.NoError(t, err)

	cf, ok := mdAfter.Frontmatter["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must be present")
	assert.Equal(t, "high", cf["complexity"])
	assert.Equal(t, mdBefore.Body, mdAfter.Body, "body must be preserved byte-for-byte")
}

func TestUpdateComplexity_EmptyClearsField(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--complexity", "low"})
	require.NoError(t, cmd.Execute())

	clearCmd := cli.NewRootCommand()
	clearBuf := new(bytes.Buffer)
	clearCmd.SetOut(clearBuf)
	clearCmd.SetErr(clearBuf)
	clearCmd.SetArgs([]string{"--cwd", root, "update", id, "--complexity", ""})
	require.NoError(t, clearCmd.Execute())

	rawAfter, err := os.ReadFile(taskFilePath(root, id))
	require.NoError(t, err)
	mdAfter, err := mdfront.Decode(rawAfter)
	require.NoError(t, err)
	cf, _ := mdAfter.Frontmatter["custom_fields"].(map[string]any)
	assert.NotContains(t, cf, "complexity")
}

func TestUpdateComplexity_InvalidEnumErrors(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--complexity", "extreme"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "trivial")
}

func TestUpdateComplexity_MutualExclusionErrorsBeforeWrite(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)
	path := taskFilePath(root, id)

	rawBefore, err := os.ReadFile(path)
	require.NoError(t, err)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--complexity", "high", "--status", "done"})
	err = cmd.Execute()
	require.Error(t, err)

	rawAfter, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, rawBefore, rawAfter, "no write may occur when the mutual-exclusion guard fires")
}

func TestUpdateComplexity_RejectsSizeMix(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "update", id, "--complexity", "high", "--size", "M"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "--size")
}

func TestListComplexity_FiltersResults(t *testing.T) {
	root := setupCLIWorkspace(t)
	highID := createSizeTask(t, root)
	lowID := createSizeTask(t, root)

	for _, args := range [][]string{
		{"--cwd", root, "update", highID, "--complexity", "high"},
		{"--cwd", root, "update", lowID, "--complexity", "low"},
	} {
		cmd := cli.NewRootCommand()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
	}

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "list", "--complexity", "high", "--json"})
	require.NoError(t, cmd.Execute())

	var items []map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &items))
	require.Len(t, items, 1)
	assert.Equal(t, highID, items[0]["id"])
}

func TestListComplexity_InvalidFilterErrors(t *testing.T) {
	root := setupCLIWorkspace(t)

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "list", "--complexity", "bogus"})
	err := cmd.Execute()
	require.Error(t, err)
	assert.ErrorContains(t, err, "trivial")
}
