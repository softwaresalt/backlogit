package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotR2_Execute_SoundnessBlockedJSONRPCCarriesDiagnostics(t *testing.T) {
	root := t.TempDir()
	plan := `{"units":[{"id":"1-T","kind":"task","title":"Parse x",
"responsibilities":["parse","render"],"verification":["go test ./x"],"acceptance":["x returns 1"]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(root, "plan.json"), []byte(plan), 0o600))

	out, err := u2ExecuteWithCapturedStdout(t, "--jsonrpc", "--cwd", root,
		"soundness", "lint", "--plan", "plan.json", "--mode", "enforce", "--format", "json")
	require.Error(t, err)

	var resp struct {
		Error struct {
			Data struct {
				Blocked  bool `json:"blocked"`
				Findings []struct {
					Rule   string `json:"rule"`
					UnitID string `json:"unit_id"`
				} `json:"findings"`
			} `json:"data"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &resp), out)
	assert.True(t, resp.Error.Data.Blocked, out)
	require.NotEmpty(t, resp.Error.Data.Findings, out)
	assert.Equal(t, "one-responsibility", resp.Error.Data.Findings[0].Rule)
	assert.Equal(t, "1-T", resp.Error.Data.Findings[0].UnitID)
}
