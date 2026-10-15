package tests_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	u099Runner  = "scripts/verify-task-lint.ps1"
	u099Feature = "175-F"
)

type u099Finding struct {
	Path    string `json:"path"`
	Linter  string `json:"linter"`
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Owner   string `json:"owner"`
}

type u099Expected struct {
	Path    string `json:"path"`
	Linter  string `json:"linter"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

type u099Task struct {
	Expected []u099Expected `json:"expected_group_residual_after_task"`
}

type u099Fixture struct {
	Tasks    map[string]u099Task `json:"tasks"`
	Findings []u099Finding       `json:"findings"`
}

func u099RepoRoot(t *testing.T) string {
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

func u099Pwsh(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("pwsh")
	require.NoError(t, err, "pwsh (PowerShell 7) is required; this harness never skips")
	return p
}

// u099Run invokes the runner with a fixed argv and returns combined output and exit code.
func u099Run(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	argv := append([]string{"-NoProfile", "-NonInteractive", "-File", filepath.Join(root, filepath.FromSlash(u099Runner))}, args...)
	cmd := exec.Command(u099Pwsh(t), argv...)
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

func u099WriteFixture(t *testing.T, fx u099Fixture) string {
	t.Helper()
	dir := t.TempDir()
	data, err := json.Marshal(fx)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "task-lint.json"), data, 0o600))
	return dir
}

func u099Base(id string) u099Fixture {
	return u099Fixture{
		Tasks: map[string]u099Task{id: {Expected: []u099Expected{
			{Path: "internal/a.go", Linter: "errcheck", Message: "Error return value is not checked", Count: 1},
		}}},
		Findings: []u099Finding{
			{Path: "internal/a.go", Linter: "errcheck", Message: "Error return value is not checked", Line: 10, Column: 2, Owner: id},
			{Path: "internal/other.go", Linter: "revive", Message: "unrelated", Line: 1, Column: 1, Owner: "999.001-T"},
		},
	}
}

func TestU175_099_TaskLintRunnerBootstrap(t *testing.T) {
	root := u099RepoRoot(t)
	script := filepath.Join(root, filepath.FromSlash(u099Runner))

	t.Run("static", func(t *testing.T) {
		info, err := os.Stat(script)
		require.NoError(t, err, "runner must exist")
		assert.Greater(t, info.Size(), int64(0), "runner must be non-empty")

		ign := exec.Command("git", "check-ignore", "-q", "--", u099Runner)
		ign.Dir = root
		ierr := ign.Run()
		var ee *exec.ExitError
		require.ErrorAs(t, ierr, &ee, "runner must not be git-ignored")
		assert.Equal(t, 1, ee.ExitCode(), "git check-ignore must report not ignored")

		const ps = `$e=$null;$t=$null;$ast=[System.Management.Automation.Language.Parser]::ParseFile($args[0],[ref]$t,[ref]$e);` +
			`$n=@();if($ast.ParamBlock){$n=@($ast.ParamBlock.Parameters|ForEach-Object{$_.Name.VariablePath.UserPath})};` +
			`@{errors=@($e).Count;params=$n}|ConvertTo-Json -Compress`
		out, err := exec.Command(u099Pwsh(t), "-NoProfile", "-NonInteractive", "-Command", ps, script).Output()
		require.NoError(t, err)
		var res struct {
			Errors int      `json:"errors"`
			Params []string `json:"params"`
		}
		require.NoError(t, json.Unmarshal(out, &res))
		assert.Zero(t, res.Errors, "AST parse errors")
		assert.Contains(t, res.Params, "TaskId")
		assert.Contains(t, res.Params, "FeatureId")
	})

	const id = "175.001-T"
	marker := "TASK-LINT-OK:" + id

	t.Run("green_success", func(t *testing.T) {
		for _, phase := range []string{"Schema", "PreCommit", "PostCommit"} {
			dir := u099WriteFixture(t, u099Base(id))
			out, code := u099Run(t, root, "-TaskId", id, "-FeatureId", u099Feature, "-Phase", phase, "-FixtureRoot", dir)
			assert.Zero(t, code, "phase %s: %s", phase, out)
			assert.Contains(t, out, marker, "phase %s", phase)
		}
	})

	t.Run("red_owned_residual", func(t *testing.T) {
		tests := []struct {
			name   string
			mutate func(*u099Fixture)
		}{
			{"retained_owned_finding", func(f *u099Fixture) {
				f.Tasks[id] = u099Task{}
			}},
			{"line_shift_stays_above_expected", func(f *u099Fixture) {
				f.Tasks[id] = u099Task{}
				f.Findings[0].Line = 99
			}},
			{"duplicate_resolved_below_expected", func(f *u099Fixture) {
				f.Tasks[id] = u099Task{Expected: []u099Expected{
					{Path: "internal/a.go", Linter: "errcheck", Message: "Error return value is not checked", Count: 2},
				}}
			}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				fx := u099Base(id)
				tc.mutate(&fx)
				out, code := u099Run(t, root, "-TaskId", id, "-FeatureId", u099Feature, "-FixtureRoot", u099WriteFixture(t, fx))
				assert.NotZero(t, code, out)
				assert.NotContains(t, out, marker)
			})
		}
	})

	t.Run("fail_closed_input", func(t *testing.T) {
		dir := u099WriteFixture(t, u099Base(id))
		tests := []struct {
			name string
			args []string
		}{
			{"missing_task_id", []string{"-FeatureId", u099Feature, "-FixtureRoot", dir}},
			{"empty_task_id", []string{"-TaskId", "", "-FeatureId", u099Feature, "-FixtureRoot", dir}},
			{"unknown_task_id", []string{"-TaskId", "175.777-T", "-FeatureId", u099Feature, "-FixtureRoot", dir}},
			{"missing_feature_id", []string{"-TaskId", id, "-FixtureRoot", dir}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				out, code := u099Run(t, root, tc.args...)
				assert.NotZero(t, code, out)
				assert.False(t, strings.Contains(out, "TASK-LINT-OK:"), out)
			})
		}
	})
}
