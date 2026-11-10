package core

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type lf175062Group struct {
	path     string
	message  string
	residual int
}

var (
	lf175CoreOnce   sync.Once
	lf175CoreCounts map[string]int
)

// TestU175_062_LintFix fails while errcheck findings remain in the groups owned
// by task 175.062-T (line-stable group multiset; line and column are ignored).
func TestU175_062_LintFix(t *testing.T) {
	groups := []lf175062Group{
		{"internal/core/canonical_cache_test.go", "Error return value of `ws.Close` is not checked", 0},
		{"internal/core/delete_crashsafe_harness_042_test.go", "Error return value of `database.Close` is not checked", 0},
		{"internal/core/delete_crashsafe_harness_042_test.go", "Error return value of `ws.DB.Close` is not checked", 0},
	}
	counts := lf175062Findings(t, "./internal/core")
	for _, g := range groups {
		key := g.path + "|" + g.message
		require.Equalf(t, g.residual, counts[key], "owned errcheck group %q in %s: remaining findings must equal expected residual", g.message, g.path)
	}
}

func lf175062Findings(t *testing.T, pkg string) map[string]int {
	t.Helper()
	// Resolve availability before Once so an existing missing-v2 skip cannot
	// mark an uninitialized cache complete for the remaining callers.
	lint := lf175062Linter(t)
	if pkg == "./internal/core" {
		lf175CoreOnce.Do(func() {
			lf175CoreCounts = lf175062Scan(t, lint, pkg, "")
		})
		require.NotNil(t, lf175CoreCounts, "shared core lint scan must have completed")
		return lf175CoreCounts
	}
	return lf175062Scan(t, lint, pkg, "")
}

func lf175062Scan(t *testing.T, lint, pkg, goos string) map[string]int {
	t.Helper()
	root := lf175062Root(t)
	cmd := exec.Command(lint, "run", "--default=none", "--enable=errcheck",
		"--output.json.path=stdout", "--output.text.path=", "--issues-exit-code=0",
		"--max-issues-per-linter=0", "--max-same-issues=0", "--uniq-by-line=false", pkg)
	cmd.Dir = root
	if goos != "" {
		cmd.Env = append(os.Environ(), "GOOS="+goos)
	}
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

func lf175062Root(t *testing.T) string {
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

// lf175062Linter locates a golangci-lint v2 binary, the version the baseline was recorded with.
func lf175062Linter(t *testing.T) string {
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
