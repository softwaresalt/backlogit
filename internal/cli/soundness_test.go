package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const soundnessSoundPlan = `{"units":[{"id":"1-T","kind":"task","title":"Parse x",
"responsibilities":["parse"],"verification":["go test ./x"],"acceptance":["x returns 1"]}]}`

const soundnessUnsoundPlan = `{"units":[{"id":"1-T","kind":"task","title":"Parse x",
"responsibilities":["parse","render"],"verification":["go test ./x"],"acceptance":["x returns 1"]}]}`

func TestU160004_CLI_SoundnessLint_ReportOnlyDoesNotFail(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "plan.json", soundnessUnsoundPlan)
	out, err := runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--format", "json")
	require.NoError(t, err)
	var d struct {
		Mode     string `json:"mode"`
		Blocked  bool   `json:"blocked"`
		Findings []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &d), out)
	assert.Equal(t, "report-only", d.Mode)
	assert.False(t, d.Blocked)
	require.NotEmpty(t, d.Findings)
	assert.Equal(t, "one-responsibility", d.Findings[0].Rule)
}

func TestU160004_CLI_SoundnessLint_EnforceFailsOnUnsoundPlan(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "plan.json", soundnessUnsoundPlan)
	out, err := runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--mode", "enforce", "--format", "text")
	require.Error(t, err)
	assert.Contains(t, out, "one-responsibility")
	assert.Contains(t, out, "1-T")
}

func TestU160004_CLI_SoundnessLint_SoundPlanPassesEnforce(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "plan.json", soundnessSoundPlan)
	_, err := runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--mode", "enforce", "--format", "text")
	require.NoError(t, err)
}

func TestU160004_CLI_SoundnessLint_ConfigAndDisableFlag(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "plan.json", soundnessUnsoundPlan)
	writeFixtureDoc(t, root, "soundness.conf", "mode: enforce\nmax_responsibilities: 2\n")
	_, err := runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--config", "soundness.conf", "--format", "text")
	require.NoError(t, err, "budget raised by config")

	_, err = runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--mode", "enforce", "--disable-rule", "one-responsibility", "--format", "text")
	require.NoError(t, err, "rule disabled by flag")
}

func TestU160004_CLI_SoundnessLint_MisconfigAndEscapeAreErrors(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "plan.json", soundnessSoundPlan)
	_, err := runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--mode", "block")
	require.Error(t, err)
	_, err = runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--mode", "enforce", "--disable-rule", "nope")
	require.Error(t, err)
	_, err = runDocs(t, root, "soundness", "lint", "--plan", "../plan.json")
	require.Error(t, err)
	_, err = runDocs(t, root, "soundness", "lint", "--plan", "missing.json")
	require.Error(t, err)
}

func TestRemediation_CLI_TrailingJSONRejected(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "plan.json", soundnessSoundPlan+` {"units":[]}`)
	_, err := runDocs(t, root, "soundness", "lint", "--plan", "plan.json")
	require.Error(t, err)
}

func TestRemediation_CLI_EmptyPlanErrorsInEnforce(t *testing.T) {
	root := t.TempDir()
	for _, body := range []string{`{}`, `{"units":[]}`} {
		writeFixtureDoc(t, root, "plan.json", body)
		_, err := runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--mode", "enforce", "--format", "text")
		require.Error(t, err, body)
	}
}

func TestRemediation_CLI_ModeFlagCannotRelaxEnforceConfig(t *testing.T) {
	root := t.TempDir()
	writeFixtureDoc(t, root, "plan.json", soundnessUnsoundPlan)
	writeFixtureDoc(t, root, "enforce.conf", "mode: enforce\n")
	_, err := runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--config", "enforce.conf", "--mode", "report-only", "--format", "text")
	require.Error(t, err)
	assert.NotErrorIs(t, err, errSoundnessBlocked)

	writeFixtureDoc(t, root, "report.conf", "mode: report-only\n")
	_, err = runDocs(t, root, "soundness", "lint", "--plan", "plan.json", "--config", "report.conf", "--mode", "enforce", "--format", "text")
	require.ErrorIs(t, err, errSoundnessBlocked)
}

func TestRemediation_CLI_ReadWorkspaceFileConfined(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "x.json"), []byte("{}"), 0o600))
	writeFixtureDoc(t, root, "in.json", "{}")
	_, err := readWorkspaceFile(root, "in.json")
	require.NoError(t, err)
	_, err = readWorkspaceFile(root, filepath.Join(outside, "x.json"))
	require.Error(t, err)
	_, err = readWorkspaceFile(root, "../"+filepath.Base(outside)+"/x.json")
	require.Error(t, err)
}
