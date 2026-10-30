package cli_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/mdfront"
)

func runUpdate(t *testing.T, root string, args ...string) error {
	t.Helper()
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(append([]string{"--cwd", root, "update"}, args...))
	return cmd.Execute()
}

func customFieldsOf(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	md, err := mdfront.Decode(raw)
	require.NoError(t, err)
	cf, _ := md.Frontmatter["custom_fields"].(map[string]any)
	return cf
}

func TestU3_ComplexityProvenanceCLI(t *testing.T) {
	root := setupCLIWorkspace(t)
	id := createSizeTask(t, root)
	path := taskFilePath(root, id)

	require.NoError(t, runUpdate(t, root, id, "--complexity", "high", "--complexity-source", "derived", "--complexity-ruleset-version", "ah-v1"))
	cf := customFieldsOf(t, path)
	assert.Equal(t, "derived", cf["complexity_source"])
	assert.Equal(t, "ah-v1", cf["complexity_ruleset_version"])

	// Ruleset alone defaults the source to human on the CLI.
	require.NoError(t, runUpdate(t, root, id, "--complexity", "medium", "--complexity-ruleset-version", "ah-v2"))
	cf = customFieldsOf(t, path)
	assert.Equal(t, "human", cf["complexity_source"])
	assert.Equal(t, "ah-v2", cf["complexity_ruleset_version"])

	// Value-only update clears stale provenance.
	require.NoError(t, runUpdate(t, root, id, "--complexity", "low"))
	cf = customFieldsOf(t, path)
	assert.Equal(t, "low", cf["complexity"])
	assert.NotContains(t, cf, "complexity_source")
	assert.NotContains(t, cf, "complexity_ruleset_version")

	// Provenance flags without --complexity, and mixes with other fields, fail.
	assert.Error(t, runUpdate(t, root, id, "--complexity-source", "human"))
	assert.Error(t, runUpdate(t, root, id, "--complexity", "low", "--complexity-source", "human", "--status", "done"))
	assert.Error(t, runUpdate(t, root, id, "--size", "M", "--complexity-source", "human"))
	assert.Error(t, runUpdate(t, root, id, "--complexity", "", "--complexity-source", "human", "--complexity-ruleset-version", "v1"))

	// Clear removes all three.
	require.NoError(t, runUpdate(t, root, id, "--complexity", "high", "--complexity-ruleset-version", "ah-v1"))
	require.NoError(t, runUpdate(t, root, id, "--complexity", ""))
	cf = customFieldsOf(t, path)
	assert.NotContains(t, cf, "complexity")
	assert.NotContains(t, cf, "complexity_source")
	assert.NotContains(t, cf, "complexity_ruleset_version")
}
