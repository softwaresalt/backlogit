package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func TestDeliberateCommand_CreatesLinkedArtifact(t *testing.T) {
	root := setupCLIWorkspace(t)
	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	cmd.SetArgs([]string{"--cwd", root, "stash", "add", "Investigate audit split", "--kind", "feature", "--priority", "high"})
	require.NoError(t, cmd.Execute())

	var added map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &added))
	stashID, _ := added["id"].(string)
	require.NotEmpty(t, stashID)

	buf.Reset()
	cmd.SetArgs([]string{"--cwd", root, "deliberate", stashID, "--chosen-direction", "Split the dashboard work into a separate feature"})
	require.NoError(t, cmd.Execute())

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result))
	entry := result["entry"].(map[string]any)
	artifact := result["artifact"].(map[string]any)
	assert.Equal(t, stashID, entry["id"])
	assert.Equal(t, artifact["id"], entry["deliberation_id"])
	assert.Equal(t, "deliberation", artifact["artifact_type"])
	assert.Equal(t, "high", artifact["priority"])
}
