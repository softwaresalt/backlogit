package tests_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const u100Runner = "scripts/verify-baseline-lint.ps1"

type u100Finding struct {
	Path    string `json:"path"`
	Linter  string `json:"linter"`
	Message string `json:"message"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

type u100Owner struct {
	Task      string `json:"task"`
	Resolves  int    `json:"resolves"`
	Completed bool   `json:"completed"`
}

type u100Group struct {
	Path          string      `json:"path"`
	Linter        string      `json:"linter"`
	Message       string      `json:"message"`
	BaselineCount int         `json:"baseline_count"`
	Owners        []u100Owner `json:"owners"`
}

type u100Inventory struct {
	Schema    string        `json:"schema"`
	Groups    []u100Group   `json:"groups"`
	Remaining []u100Finding `json:"remaining"`
}

func u100RepoRoot(t *testing.T) string {
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

func u100Pwsh(t *testing.T) string {
	t.Helper()
	p, err := exec.LookPath("pwsh")
	require.NoError(t, err, "pwsh (PowerShell 7) is required; this harness never skips")
	return p
}

func u100Run(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	argv := append([]string{"-NoProfile", "-NonInteractive", "-File", filepath.Join(root, filepath.FromSlash(u100Runner))}, args...)
	cmd := exec.Command(u100Pwsh(t), argv...)
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

func u100Sum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// u100Write stores raw inventory bytes in a temp dir and returns the path and true SHA-256.
func u100Write(t *testing.T, data []byte) (string, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "inventory.json")
	require.NoError(t, os.WriteFile(p, data, 0o600))
	return p, u100Sum(data)
}

func u100Marshal(t *testing.T, inv u100Inventory) []byte {
	t.Helper()
	data, err := json.Marshal(inv)
	require.NoError(t, err)
	return data
}

const u100Msg = "Error return value is not checked"

// u100Base: group of 2 occurrences, completed owner resolves 1, so expected residual is 1.
func u100Base() u100Inventory {
	return u100Inventory{
		Schema: "baseline-inventory/v1",
		Groups: []u100Group{{
			Path: "internal/a.go", Linter: "errcheck", Message: u100Msg, BaselineCount: 2,
			Owners: []u100Owner{
				{Task: "175.001-T", Resolves: 1, Completed: true},
				{Task: "175.002-T", Resolves: 1, Completed: false},
			},
		}},
		Remaining: []u100Finding{
			{Path: "internal/a.go", Linter: "errcheck", Message: u100Msg, Line: 50, Column: 3},
		},
	}
}

func u100Args(inv, sum string) []string {
	return []string{"-Inventory", inv, "-InventorySha256", sum, "-Shipment", "176-S", "-FeatureId", "175-F", "-TerminalTask", "175.012-T"}
}

func TestU175_100_BaselineLintRunnerBootstrap(t *testing.T) {
	root := u100RepoRoot(t)
	script := filepath.Join(root, filepath.FromSlash(u100Runner))

	t.Run("static", func(t *testing.T) {
		info, err := os.Stat(script)
		require.NoError(t, err, "runner must exist")
		assert.Greater(t, info.Size(), int64(0), "runner must be non-empty")

		ign := exec.Command("git", "check-ignore", "-q", "--", u100Runner)
		ign.Dir = root
		ierr := ign.Run()
		var ee *exec.ExitError
		require.ErrorAs(t, ierr, &ee, "runner must not be git-ignored")
		assert.Equal(t, 1, ee.ExitCode(), "git check-ignore must report not ignored")

		trk := exec.Command("git", "ls-files", "--error-unmatch", "--", u100Runner)
		trk.Dir = root
		tout, terr := trk.CombinedOutput()
		require.NoError(t, terr, "runner must be git-tracked: %s", tout)

		const ps = `$e=$null;$t=$null;$ast=[System.Management.Automation.Language.Parser]::ParseFile($env:U175_RUNNER_PATH,[ref]$t,[ref]$e);` +
			`$n=@();if($ast.ParamBlock){$n=@($ast.ParamBlock.Parameters|ForEach-Object{$_.Name.VariablePath.UserPath})};` +
			`@{errors=@($e).Count;params=$n}|ConvertTo-Json -Compress`
		pc := exec.Command(u100Pwsh(t), "-NoProfile", "-NonInteractive", "-Command", ps)
		pc.Env = append(os.Environ(), "U175_RUNNER_PATH="+script)
		out, err := pc.Output()
		require.NoError(t, err)
		var res struct {
			Errors int      `json:"errors"`
			Params []string `json:"params"`
		}
		require.NoError(t, json.Unmarshal(out, &res))
		assert.Zero(t, res.Errors, "AST parse errors")
		for _, p := range []string{"Inventory", "InventorySha256", "Shipment", "FeatureId", "TerminalTask"} {
			assert.Contains(t, res.Params, p)
		}
	})

	t.Run("green_success", func(t *testing.T) {
		inv, sum := u100Write(t, u100Marshal(t, u100Base()))
		out, code := u100Run(t, root, u100Args(inv, sum)...)
		assert.Zero(t, code, out)
		assert.Contains(t, out, "BASELINE-LINT-OK:176-S")

		// Uppercase hash and a shifted line both remain green: identity ignores line/column.
		shifted := u100Base()
		shifted.Remaining[0].Line = 999
		inv, sum = u100Write(t, u100Marshal(t, shifted))
		out, code = u100Run(t, root, u100Args(inv, strings.ToUpper(sum))...)
		assert.Zero(t, code, out)
		assert.Contains(t, out, "BASELINE-LINT-OK:176-S")

		// Composed and decomposed forms are the same group under NFC.
		nfc := u100Base()
		nfc.Groups[0].Message = "caf\u00e9  unchecked"
		nfc.Remaining[0].Message = "cafe\u0301 unchecked"
		inv, sum = u100Write(t, u100Marshal(t, nfc))
		out, code = u100Run(t, root, u100Args(inv, sum)...)
		assert.Zero(t, code, out)
		assert.Contains(t, out, "BASELINE-LINT-OK:176-S")
	})

	t.Run("red_integrity_monotonicity", func(t *testing.T) {
		tests := []struct {
			name   string
			mutate func(*u100Inventory)
			badSum bool
			reason string
		}{
			{name: "tampered_hash", mutate: func(*u100Inventory) {}, badSum: true, reason: "HASH-MISMATCH"},
			{name: "shifted_descendant_unresolved", mutate: func(i *u100Inventory) {
				i.Remaining = append(i.Remaining, u100Finding{Path: "internal/a.go", Linter: "errcheck", Message: u100Msg, Line: 77, Column: 1})
			}, reason: "MONOTONICITY-VIOLATION"},
			{name: "resolved_by_no_completed_owner", mutate: func(i *u100Inventory) {
				i.Remaining = []u100Finding{}
			}, reason: "MONOTONICITY-VIOLATION"},
			{name: "new_fingerprint", mutate: func(i *u100Inventory) {
				i.Remaining = append(i.Remaining, u100Finding{Path: "internal/new.go", Linter: "revive", Message: "brand new", Line: 1, Column: 1})
			}, reason: "MONOTONICITY-VIOLATION"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				inv := u100Base()
				tc.mutate(&inv)
				path, sum := u100Write(t, u100Marshal(t, inv))
				if tc.badSum {
					sum = strings.Repeat("0", 64)
				}
				out, code := u100Run(t, root, u100Args(path, sum)...)
				assert.Equal(t, 1, code, out)
				assert.Contains(t, out, tc.reason)
			})
		}
	})

	t.Run("fail_closed_input", func(t *testing.T) {
		malformed, msum := u100Write(t, []byte("{not json"))
		wrongSchema := u100Base()
		wrongSchema.Schema = "baseline-inventory/v0"
		wspath, wssum := u100Write(t, u100Marshal(t, wrongSchema))
		okPath, _ := u100Write(t, u100Marshal(t, u100Base()))
		tests := []struct {
			name string
			path string
			sum  string
		}{
			{"wrong_inventory_schema", wspath, wssum},
			{"malformed_sha256_short", okPath, "abc123"},
			{"malformed_sha256_non_hex", okPath, strings.Repeat("g", 64)},
			{"missing_inventory", filepath.Join(t.TempDir(), "absent.json"), strings.Repeat("0", 64)},
			{"malformed_inventory_with_matching_hash", malformed, msum},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				out, code := u100Run(t, root, u100Args(tc.path, tc.sum)...)
				assert.Equal(t, 2, code, out)
				assert.Contains(t, out, "INVALID-INPUT")
			})
		}
	})

	t.Run("fail_closed_shape_and_case", func(t *testing.T) {
		const g = `{"path":"internal/a.go","linter":"errcheck","message":"Error return value is not checked","baseline_count":1,"owners":[]}`
		const r = `{"path":"internal/a.go","linter":"errcheck","message":"Error return value is not checked","line":1,"column":1}`
		shapes := map[string]string{
			"null_groups_remaining": `{"schema":"baseline-inventory/v1","groups":null,"remaining":null}`,
			"null_members":          `{"schema":"baseline-inventory/v1","groups":[null],"remaining":[null]}`,
			"null_group_member":     `{"schema":"baseline-inventory/v1","groups":[null],"remaining":[` + r + `]}`,
			"null_remaining_member": `{"schema":"baseline-inventory/v1","groups":[` + g + `],"remaining":[null]}`,
			"null_owners":           `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":1,"owners":null}],"remaining":[]}`,
			"null_owner_member":     `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":1,"owners":[null]}],"remaining":[]}`,
			"baseline_null":         `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":null,"owners":[]}],"remaining":[]}`,
			"baseline_negative":     `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":-1,"owners":[]}],"remaining":[]}`,
			"baseline_string":       `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":"1","owners":[]}],"remaining":[]}`,
			"baseline_fractional":   `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":1.5,"owners":[]}],"remaining":[]}`,
			"baseline_too_large":    `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":2147483648,"owners":[]}],"remaining":[]}`,
			"resolves_negative":     `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":1,"owners":[{"task":"175.001-T","resolves":-1,"completed":true}]}],"remaining":[{"path":"a","linter":"l","message":"m","line":1,"column":1},{"path":"a","linter":"l","message":"m","line":2,"column":1}]}`,
			"resolves_null":         `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":1,"owners":[{"task":"175.001-T","resolves":null,"completed":false}]}],"remaining":[]}`,
			"object_groups":         `{"schema":"baseline-inventory/v1","groups":{},"remaining":[]}`,
		}
		for name, body := range shapes {
			t.Run(name, func(t *testing.T) {
				path, sum := u100Write(t, []byte(body))
				out, code := u100Run(t, root, u100Args(path, sum)...)
				assert.Equal(t, 2, code, out)
				assert.Contains(t, out, "INVALID-INPUT")
				assert.NotContains(t, out, "BASELINE-LINT-OK")
			})
		}

		cases := map[string]func(*u100Inventory){
			"path_case":    func(i *u100Inventory) { i.Remaining[0].Path = "internal/A.go" },
			"message_case": func(i *u100Inventory) { i.Remaining[0].Message = "error return value is not checked" },
		}
		for name, mutate := range cases {
			t.Run(name, func(t *testing.T) {
				inv := u100Base()
				mutate(&inv)
				path, sum := u100Write(t, u100Marshal(t, inv))
				out, code := u100Run(t, root, u100Args(path, sum)...)
				assert.Equal(t, 1, code, out)
				assert.Contains(t, out, "MONOTONICITY-VIOLATION")
			})
		}
	})

	t.Run("completed_must_be_boolean", func(t *testing.T) {
		owners := map[string]string{
			"string_true": `{"task":"175.001-T","resolves":1,"completed":"true"}`,
			"numeric_one": `{"task":"175.001-T","resolves":1,"completed":1}`,
			"missing":     `{"task":"175.001-T","resolves":1}`,
		}
		for name, o := range owners {
			t.Run(name, func(t *testing.T) {
				body := `{"schema":"baseline-inventory/v1","groups":[{"path":"a","linter":"l","message":"m","baseline_count":1,"owners":[` + o + `]}],"remaining":[]}`
				path, sum := u100Write(t, []byte(body))
				out, code := u100Run(t, root, u100Args(path, sum)...)
				assert.Equal(t, 2, code, out)
				assert.Contains(t, out, "INVALID-INPUT")
				assert.NotContains(t, out, "BASELINE-LINT-OK")
			})
		}
	})

	t.Run("pipe_collision_fingerprints_distinct", func(t *testing.T) {
		inv := u100Inventory{
			Schema: "baseline-inventory/v1",
			Groups: []u100Group{{
				Path: "a.go", Linter: "errcheck", Message: "b.go|errcheck|unchecked", BaselineCount: 1,
				Owners: []u100Owner{{Task: "175.001-T", Resolves: 0, Completed: false}},
			}},
			Remaining: []u100Finding{
				{Path: "a.go|errcheck|b.go", Linter: "errcheck", Message: "unchecked", Line: 1, Column: 1},
			},
		}
		path, sum := u100Write(t, u100Marshal(t, inv))
		out, code := u100Run(t, root, u100Args(path, sum)...)
		assert.Equal(t, 1, code, out)
		assert.Contains(t, out, "MONOTONICITY-VIOLATION")
		assert.NotContains(t, out, "BASELINE-LINT-OK")
	})

	t.Run("validation_sweep", func(t *testing.T) {
		const okOwner = `{"task":"175.001-T","resolves":0,"completed":false}`
		grp := func(path, linter, msg string) string {
			return `{"path":` + path + `,"linter":` + linter + `,"message":` + msg + `,"baseline_count":1,"owners":[` + okOwner + `]}`
		}
		rem := func(path, linter, msg string) string {
			return `{"path":` + path + `,"linter":` + linter + `,"message":` + msg + `,"line":1,"column":1}`
		}
		inv := func(groups, remaining string) string {
			return `{"schema":"baseline-inventory/v1","groups":[` + groups + `],"remaining":[` + remaining + `]}`
		}
		okGroup := grp(`"a"`, `"l"`, `"123"`)
		bodies := map[string]string{
			"numeric_remaining_message_vs_string_group": inv(okGroup, rem(`"a"`, `"l"`, `123`)),
			"numeric_group_message_vs_string_remaining": inv(grp(`"a"`, `"l"`, `123`), rem(`"a"`, `"l"`, `"123"`)),
			"group_path_number":                         inv(grp(`1`, `"l"`, `"m"`), ``),
			"group_linter_null":                         inv(grp(`"a"`, `null`, `"m"`), ``),
			"group_path_empty":                          inv(grp(`""`, `"l"`, `"m"`), ``),
			"group_message_missing":                     inv(`{"path":"a","linter":"l","baseline_count":1,"owners":[]}`, ``),
			"remaining_empty_object":                    inv(okGroup, `{}`),
			"remaining_linter_array":                    inv(okGroup, rem(`"a"`, `["l"]`, `"123"`)),
			"remaining_path_missing":                    inv(okGroup, `{"linter":"l","message":"123"}`),
			"duplicate_group_fingerprint":               inv(okGroup+`,`+grp(`"a"`, `"l"`, `" 123 "`), ``),
			"duplicate_owner_task":                      inv(`{"path":"a","linter":"l","message":"m","baseline_count":2,"owners":[`+okOwner+`,`+okOwner+`]}`, ``),
			"owner_task_number":                         inv(`{"path":"a","linter":"l","message":"m","baseline_count":1,"owners":[{"task":1,"resolves":0,"completed":false}]}`, ``),
			"owner_task_missing":                        inv(`{"path":"a","linter":"l","message":"m","baseline_count":1,"owners":[{"resolves":0,"completed":false}]}`, ``),
			"owner_task_bad_shape":                      inv(`{"path":"a","linter":"l","message":"m","baseline_count":1,"owners":[{"task":"t","resolves":0,"completed":false}]}`, ``),
			"schema_number":                             `{"schema":1,"groups":[],"remaining":[]}`,
			"schema_wrong_case":                         `{"schema":"BASELINE-INVENTORY/V1","groups":[],"remaining":[]}`,
			"duplicate_top_level_key":                   `{"schema":"baseline-inventory/v1","groups":[],"remaining":[],"remaining":[]}`,
			"top_level_array":                           `[{"schema":"baseline-inventory/v1","groups":[],"remaining":[]}]`,
			"top_level_scalar":                          `42`,
			"top_level_null":                            `null`,
			"empty_file":                                ``,
		}
		for name, body := range bodies {
			t.Run(name, func(t *testing.T) {
				path, sum := u100Write(t, []byte(body))
				out, code := u100Run(t, root, u100Args(path, sum)...)
				assert.Equal(t, 2, code, out)
				assert.Contains(t, out, "INVALID-INPUT")
				assert.NotContains(t, out, "BASELINE-LINT-OK")
			})
		}

		t.Run("utf8_bom_and_date_like_message_pass", func(t *testing.T) {
			body := "\xef\xbb\xbf" + inv(grp(`"a"`, `"l"`, `"2020-01-01T00:00:00Z"`), rem(`"a"`, `"l"`, `"2020-01-01T00:00:00Z"`))
			path, sum := u100Write(t, []byte(body))
			out, code := u100Run(t, root, u100Args(path, sum)...)
			assert.Zero(t, code, out)
			assert.Contains(t, out, "BASELINE-LINT-OK:176-S")
		})

		t.Run("invalid_parameters", func(t *testing.T) {
			path, sum := u100Write(t, u100Marshal(t, u100Base()))
			mk := func(key, val string) []string {
				m := map[string]string{"-Inventory": path, "-InventorySha256": sum, "-Shipment": "176-S", "-FeatureId": "175-F", "-TerminalTask": "175.012-T"}
				m[key] = val
				var a []string
				for k, v := range m {
					a = append(a, k, v)
				}
				return a
			}
			tests := map[string][]string{
				"shipment_feature_shape": mk("-Shipment", "176-F"),
				"shipment_empty":         mk("-Shipment", ""),
				"shipment_trailing_nl":   mk("-Shipment", "176-S\n"),
				"feature_subtask_shape":  mk("-FeatureId", "175.001-F"),
				"feature_wrong_suffix":   mk("-FeatureId", "175-S"),
				"terminal_feature_shape": mk("-TerminalTask", "175-F"),
				"terminal_trailing_nl":   mk("-TerminalTask", "175.012-T\n"),
				"inventory_empty":        mk("-Inventory", ""),
				"sha_empty":              mk("-InventorySha256", ""),
				"sha_trailing_nl":        mk("-InventorySha256", sum+"\n"),
			}
			for name, args := range tests {
				t.Run(name, func(t *testing.T) {
					out, code := u100Run(t, root, args...)
					assert.Equal(t, 2, code, out)
					assert.Contains(t, out, "INVALID-INPUT")
					assert.NotContains(t, out, "BASELINE-LINT-OK")
				})
			}
		})
	})
}

// TestU175_100_RelativeInventoryUsesPowerShellLocation guards the PowerShell-location vs process-cwd split.
func TestU175_100_RelativeInventoryUsesPowerShellLocation(t *testing.T) {
	root := u100RepoRoot(t)
	script := filepath.Join(root, filepath.FromSlash(u100Runner))
	data := u100Marshal(t, u100Base())
	inv, sum := u100Write(t, data)
	dir := filepath.Dir(inv)

	// Arguments travel through the environment, never through command-string interpolation.
	const ps = `Set-Location -LiteralPath $env:U175_LOC; & $env:U175_RUNNER -Inventory $env:U175_INV -InventorySha256 $env:U175_SUM -Shipment 176-S -FeatureId 175-F -TerminalTask 175.012-T; exit $LASTEXITCODE`
	run := func(t *testing.T, sumArg string) (string, int) {
		cmd := exec.Command(u100Pwsh(t), "-NoProfile", "-NonInteractive", "-Command", ps)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "U175_LOC="+dir, "U175_RUNNER="+script, "U175_INV=inventory.json", "U175_SUM="+sumArg)
		out, err := cmd.CombinedOutput()
		if err == nil {
			return string(out), 0
		}
		var ee *exec.ExitError
		require.ErrorAs(t, err, &ee)
		return string(out), ee.ExitCode()
	}

	t.Run("relative_path_resolves_against_powershell_location", func(t *testing.T) {
		out, code := run(t, sum)
		assert.Zero(t, code, out)
		assert.Contains(t, out, "BASELINE-LINT-OK:176-S")
	})
	t.Run("hash_still_enforced", func(t *testing.T) {
		out, code := run(t, strings.Repeat("0", 64))
		assert.Equal(t, 1, code, out)
		assert.NotContains(t, out, "BASELINE-LINT-OK:")
	})
}

// TestU175_100_InventorySingleRead guards against hashing one snapshot and parsing another.
func TestU175_100_InventorySingleRead(t *testing.T) {
	root := u100RepoRoot(t)
	script := filepath.Join(root, filepath.FromSlash(u100Runner))

	t.Run("static_single_read", func(t *testing.T) {
		const ps = `$e=$null;$t=$null;$ast=[System.Management.Automation.Language.Parser]::ParseFile($env:U175_RUNNER_PATH,[ref]$t,[ref]$e);` +
			`$c=$ast.FindAll({param($n) $n -is [System.Management.Automation.Language.CommandAst]},$true)|ForEach-Object{$_.GetCommandName()};` +
			`$m=$ast.FindAll({param($n) $n -is [System.Management.Automation.Language.InvokeMemberExpressionAst]},$true)|ForEach-Object{$_.Member.Extent.Text};` +
			`@{commands=@($c|Where-Object{$_});members=@($m)}|ConvertTo-Json -Compress`
		pc := exec.Command(u100Pwsh(t), "-NoProfile", "-NonInteractive", "-Command", ps)
		pc.Env = append(os.Environ(), "U175_RUNNER_PATH="+script)
		out, err := pc.Output()
		require.NoError(t, err)
		var res struct {
			Commands []string `json:"commands"`
			Members  []string `json:"members"`
		}
		require.NoError(t, json.Unmarshal(out, &res))
		reads := 0
		for _, c := range res.Commands {
			assert.NotEqual(t, "get-filehash", strings.ToLower(c), "hashing must use the captured bytes, not reopen the file")
			if lc := strings.ToLower(c); lc == "get-content" || lc == "gc" || lc == "cat" || lc == "type" {
				reads++
			}
		}
		for _, m := range res.Members {
			if strings.HasPrefix(m, "ReadAll") || m == "OpenRead" || m == "Open" {
				reads++
			}
		}
		assert.Equal(t, 1, reads, "inventory must be read from disk exactly once")
		assert.Contains(t, res.Members, "ReadAllBytes")
	})

	t.Run("concurrent_replacement", func(t *testing.T) {
		a := u100Marshal(t, u100Base())
		bad := u100Base()
		bad.Remaining = append(bad.Remaining, u100Finding{Path: "internal/a.go", Linter: "errcheck", Message: u100Msg, Line: 77, Column: 1})
		b := u100Marshal(t, bad)
		require.False(t, bytes.Equal(a, b))
		inv, sum := u100Write(t, a)

		stop := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				data := a
				if i%2 == 1 {
					data = b
				}
				_ = os.WriteFile(inv, data, 0o600)
			}
		}()
		defer func() {
			close(stop)
			wg.Wait()
		}()

		// Hash of A is pinned: any run must either see A (OK) or fail the hash check;
		// a monotonicity verdict would mean B was parsed after A was hashed.
		for i := 0; i < 8; i++ {
			out, code := u100Run(t, root, u100Args(inv, sum)...)
			switch code {
			case 0:
				assert.Contains(t, out, "BASELINE-LINT-OK:176-S")
			case 1:
				assert.Contains(t, out, "HASH-MISMATCH", out)
				assert.NotContains(t, out, "MONOTONICITY-VIOLATION", out)
			default:
				assert.Contains(t, out, "BASELINE-LINT-FAIL", out)
			}
		}
	})
}
