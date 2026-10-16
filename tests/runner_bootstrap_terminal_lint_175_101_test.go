package tests_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	u101Runner = "scripts/verify-terminal-lint.ps1"
	u101Marker = "TERMINAL-LINT-OK:175-F"
)

type u101Finding struct {
	Path    string `json:"path"`
	Linter  string `json:"linter"`
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

func u101RepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod not found above test working directory")
		dir = parent
	}
}

func u101Pwsh(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("pwsh")
	require.NoError(t, err, "pwsh (PowerShell 7) is required; this harness never skips")
	return p
}

func u101Run(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	argv := append([]string{"-NoProfile", "-NonInteractive", "-File", filepath.Join(root, filepath.FromSlash(u101Runner))}, args...)
	cmd := exec.Command(u101Pwsh(t), argv...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var ee *exec.ExitError
	if assert.ErrorAs(t, err, &ee) {
		return string(out), ee.ExitCode()
	}
	return string(out), -1
}

// u101Fixture writes per-GOOS fixture files; a nil slice means a clean surface.
func u101Fixture(t *testing.T, windows, linux []u101Finding) string {
	t.Helper()
	dir := t.TempDir()
	for name, findings := range map[string][]u101Finding{"windows": windows, "linux": linux} {
		if findings == nil {
			findings = []u101Finding{}
		}
		data, err := json.Marshal(map[string]any{"goos": name, "findings": findings})
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+".json"), data, 0o600))
	}
	return dir
}

// u101GoosFixture writes clean surface fixtures with explicit goos values; an empty value omits the field.
func u101GoosFixture(t *testing.T, windowsGoos, linuxGoos string) string {
	t.Helper()
	dir := t.TempDir()
	for name, goos := range map[string]string{"windows": windowsGoos, "linux": linuxGoos} {
		doc := map[string]any{"findings": []u101Finding{}}
		if goos != "" {
			doc["goos"] = goos
		}
		data, err := json.Marshal(doc)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name+".json"), data, 0o600))
	}
	return dir
}

func u101MalformedFixture(t *testing.T) string {
	t.Helper()
	dir := u101Fixture(t, nil, nil)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "linux.json"), []byte("{not json"), 0o600))
	return dir
}

func TestU175_101_TerminalLintRunnerBootstrap(t *testing.T) {
	root := u101RepoRoot(t)
	script := filepath.Join(root, filepath.FromSlash(u101Runner))

	t.Run("static", func(t *testing.T) {
		info, err := os.Stat(script)
		require.NoError(t, err, "runner must exist")
		assert.Greater(t, info.Size(), int64(0), "runner must be non-empty")

		ign := exec.Command("git", "check-ignore", "-q", "--", u101Runner)
		ign.Dir = root
		ierr := ign.Run()
		var ee *exec.ExitError
		require.ErrorAs(t, ierr, &ee, "runner must not be git-ignored")
		assert.Equal(t, 1, ee.ExitCode(), "git check-ignore must report not ignored")

		trk := exec.Command("git", "ls-files", "--error-unmatch", "--", u101Runner)
		trk.Dir = root
		tout, terr := trk.CombinedOutput()
		require.NoError(t, terr, "runner must be git-tracked: %s", tout)

		const ps = `$e=$null;$t=$null;$ast=[System.Management.Automation.Language.Parser]::ParseFile($env:U175_RUNNER_PATH,[ref]$t,[ref]$e);` +
			`$n=@();if($ast.ParamBlock){$n=@($ast.ParamBlock.Parameters|ForEach-Object{$_.Name.VariablePath.UserPath})};` +
			`@{errors=@($e).Count;params=$n}|ConvertTo-Json -Compress`
		pc := exec.Command(u101Pwsh(t), "-NoProfile", "-NonInteractive", "-Command", ps)
		pc.Env = append(os.Environ(), "U175_RUNNER_PATH="+script)
		out, err := pc.Output()
		require.NoError(t, err)
		var res struct {
			Errors int      `json:"errors"`
			Params []string `json:"params"`
		}
		require.NoError(t, json.Unmarshal(out, &res))
		assert.Zero(t, res.Errors, "AST parse errors")
		assert.Contains(t, res.Params, "FeatureId")
	})

	t.Run("green_success", func(t *testing.T) {
		out, code := u101Run(t, root, "-FeatureId", "175-F", "-FixtureRoot", u101Fixture(t, nil, nil))
		assert.Zero(t, code, out)
		assert.Contains(t, out, u101Marker)
	})

	t.Run("red_single_surface", func(t *testing.T) {
		tests := []struct {
			name           string
			windows, linux []u101Finding
		}{
			{"windows_only_residual", []u101Finding{{Path: "internal/x_windows.go", Linter: "errcheck", Message: "unchecked", Line: 3, Column: 1}}, nil},
			{"linux_only_residual", nil, []u101Finding{{Path: "internal/x_unix.go", Linter: "errcheck", Message: "unchecked", Line: 3, Column: 1}}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				out, code := u101Run(t, root, "-FeatureId", "175-F", "-FixtureRoot", u101Fixture(t, tc.windows, tc.linux))
				assert.Equal(t, 1, code, out)
				assert.Contains(t, out, "SURFACE-RESIDUAL")
				assert.NotContains(t, out, u101Marker)
			})
		}
	})

	t.Run("fail_closed_input", func(t *testing.T) {
		tests := []struct {
			name string
			args []string
		}{
			{"missing_feature_id", []string{"-FixtureRoot", u101Fixture(t, nil, nil)}},
			{"missing_fixture_root", []string{"-FeatureId", "175-F"}},
			{"absent_fixture_files", []string{"-FeatureId", "175-F", "-FixtureRoot", t.TempDir()}},
			{"wrong_goos", []string{"-FeatureId", "175-F", "-FixtureRoot", u101GoosFixture(t, "windows", "darwin")}},
			{"missing_goos", []string{"-FeatureId", "175-F", "-FixtureRoot", u101GoosFixture(t, "", "linux")}},
			{"uppercase_goos", []string{"-FeatureId", "175-F", "-FixtureRoot", u101GoosFixture(t, "WINDOWS", "linux")}},
			{"malformed_surface_json", []string{"-FeatureId", "175-F", "-FixtureRoot", u101MalformedFixture(t)}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				out, code := u101Run(t, root, tc.args...)
				assert.Equal(t, 2, code, out)
				assert.Contains(t, out, "INVALID-INPUT")
				assert.NotContains(t, out, "TERMINAL-LINT-OK:")
			})
		}
	})
}
