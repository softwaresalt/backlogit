package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type lf175060Group struct {
	path     string
	message  string
	residual int
}

// TestU175_060_LintFix fails while errcheck findings remain in the groups owned
// by task 175.060-T (line-stable group multiset; line and column are ignored).
func TestU175_060_LintFix(t *testing.T) {
	groups := []lf175060Group{
		{"internal/cli/shipment_list_items_test.go", "Error return value of `ws.Close` is not checked", 0},
		{"internal/cli/size_composition_columns_test.go", "Error return value of `ws.Close` is not checked", 0},
	}
	counts := lf175060Findings(t, "./internal/cli")
	for _, g := range groups {
		key := g.path + "|" + g.message
		require.Equalf(t, g.residual, counts[key], "owned errcheck group %q in %s: remaining findings must equal expected residual", g.message, g.path)
	}
}

func lf175060Findings(t *testing.T, pkg string) map[string]int {
	t.Helper()
	root := lf175060Root(t)
	lint := lf175060Linter(t)
	cmd := exec.Command(lint, "run", "--default=none", "--enable=errcheck",
		"--output.json.path=stdout", "--output.text.path=", "--issues-exit-code=0", pkg)
	cmd.Dir = root
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	require.NoError(t, cmd.Run(), "golangci-lint run failed")
	line := stdout.Bytes()
	if i := bytes.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	var report struct {
		Issues []struct {
			Text string
			Pos  struct{ Filename string }
		}
	}
	require.NoError(t, json.Unmarshal(line, &report), "parse golangci-lint JSON")
	counts := map[string]int{}
	for _, is := range report.Issues {
		counts[filepath.ToSlash(is.Pos.Filename)+"|"+is.Text]++
	}
	return counts
}

func lf175060Root(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "go.mod not found")
		dir = parent
	}
}

// lf175060Linter locates a golangci-lint v2 binary, the version the baseline was recorded with.
func lf175060Linter(t *testing.T) string {
	t.Helper()
	var candidates []string
	if p, err := exec.LookPath("golangci-lint"); err == nil {
		candidates = append(candidates, p)
	}
	if out, err := exec.Command("go", "env", "GOPATH").Output(); err == nil {
		name := "golangci-lint"
		if os.PathSeparator == '\\' {
			name += ".exe"
		}
		candidates = append(candidates, filepath.Join(strings.TrimSpace(string(out)), "bin", name))
	}
	for _, c := range candidates {
		out, err := exec.Command(c, "--version").Output()
		if err == nil && strings.Contains(string(out), "version 2.") {
			return c
		}
	}
	t.Skip("golangci-lint v2 not available; lint harness skipped")
	return ""
}
