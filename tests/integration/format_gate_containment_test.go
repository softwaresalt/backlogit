package integration_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var unscopedGofmtLine = regexp.MustCompile(`\bgofmt\s+-l\s+\.(?:$|\s|[;|&)"'])`)

func TestU1_MakefileFormatGateScopesSourceDirectoriesAndCoversTrackedGo(t *testing.T) {
	repoRoot := testRepoRoot(t)

	t.Run("Makefile fmt target", func(t *testing.T) {
		content := readGovernedSurface(t, repoRoot, "Makefile")
		recipe := localMakeRecipeLines(content, "fmt")
		require.NotEmpty(t, recipe, "Makefile fmt target must have at least one recipe line")

		var hasScopedGofmt bool
		for _, line := range recipe {
			if strings.Contains(line, "gofmt -l cmd internal tests scripts") {
				hasScopedGofmt = true
				break
			}
		}
		assert.True(t, hasScopedGofmt, "Makefile fmt recipe must invoke `gofmt -l cmd internal tests scripts`")

		for i, line := range strings.Split(content, "\n") {
			assert.False(t, unscopedGofmtLine.MatchString(line), "Makefile line %d must not contain unscoped `gofmt -l .`: %s", i+1, line)
		}
	})

	t.Run("Go source directory completeness", func(t *testing.T) {
		var goFiles []string
		_, statErr := os.Stat(filepath.Join(repoRoot, ".git"))
		switch {
		case statErr == nil:
			cmd := exec.CommandContext(t.Context(), "git", "-C", repoRoot, "ls-files", "-z", "--", "*.go")
			out, err := cmd.Output()
			require.NoError(t, err, "git ls-files for *.go must succeed")
			for _, raw := range bytes.Split(out, []byte{0}) {
				if len(raw) == 0 {
					continue
				}
				goFiles = append(goFiles, filepath.ToSlash(string(raw)))
			}
		case errors.Is(statErr, os.ErrNotExist):
			err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(d.Name(), ".go") {
					rel, relErr := filepath.Rel(repoRoot, p)
					if relErr != nil {
						return relErr
					}
					goFiles = append(goFiles, filepath.ToSlash(rel))
				}
				return nil
			})
			require.NoError(t, err, "walking repoRoot for .go files must succeed")
		default:
			require.NoError(t, statErr)
		}

		require.NotEmpty(t, goFiles, "repository must contain tracked .go files")

		allowedDirs := map[string]bool{
			"cmd":      true,
			"internal": true,
			"tests":    true,
			"scripts":  true,
		}

		for _, rel := range goFiles {
			assert.NotEqual(t, ".", path.Dir(rel), "root-level Go file %q is not allowed", rel)
			top := strings.SplitN(rel, "/", 2)[0]
			assert.True(t, allowedDirs[top], "Go file %q resides in top-level directory %q outside {cmd, internal, tests, scripts}", rel, top)
		}
	})
}

func TestU2_MakePS1FormatGateScopesSourceDirectories(t *testing.T) {
	repoRoot := testRepoRoot(t)

	t.Run("make.ps1 all and fmt targets", func(t *testing.T) {
		content := readGovernedSurface(t, repoRoot, "make.ps1")

		for i, line := range strings.Split(content, "\n") {
			assert.False(t, unscopedGofmtLine.MatchString(line), "make.ps1 line %d must not contain unscoped `gofmt -l .`: %s", i+1, line)
		}

		const scopedGofmtAssign = "$bad = gofmt -l cmd internal tests scripts"
		assert.GreaterOrEqual(t, strings.Count(content, scopedGofmtAssign), 2,
			"make.ps1 must contain at least 2 occurrences of %q (covering both all and fmt targets)", scopedGofmtAssign)

		extractSwitchBlock := func(src, target string) string {
			lines := strings.Split(src, "\n")
			headerPattern := regexp.MustCompile(`^\s*"` + regexp.QuoteMeta(target) + `"\s*\{`)
			var collected []string
			depth := 0
			inBlock := false
			for _, line := range lines {
				if !inBlock {
					if !headerPattern.MatchString(line) {
						continue
					}
					inBlock = true
				}
				collected = append(collected, line)
				depth += strings.Count(line, "{") - strings.Count(line, "}")
				if depth <= 0 {
					break
				}
			}
			return strings.Join(collected, "\n")
		}

		for _, target := range []string{"all", "fmt"} {
			block := extractSwitchBlock(content, target)
			require.NotEmpty(t, block, "make.ps1 must contain %q switch block", target)
			assert.Contains(t, block, scopedGofmtAssign,
				"make.ps1 %q switch block must contain %q", target, scopedGofmtAssign)
			for i, line := range strings.Split(block, "\n") {
				assert.False(t, unscopedGofmtLine.MatchString(line),
					"make.ps1 %q switch block line %d must not contain unscoped `gofmt -l .`: %s", target, i+1, line)
			}
		}
	})

	t.Run("copilot isolation probe", func(t *testing.T) {
		probeDir := t.TempDir()

		seedFiles := map[string]string{
			"cmd/probe/main.go":         "package main\n\nfunc main() {}\n",
			"internal/probe/probe.go":   "package probe\n",
			"tests/probe/probe_test.go": "package probe\n",
			"scripts/probe/probe.go":    "package probe\n",
			".copilot/invalid.go":       "package broken\nfunc (\n",
		}
		for rel, src := range seedFiles {
			dst := filepath.Join(probeDir, filepath.FromSlash(rel))
			require.NoError(t, os.MkdirAll(filepath.Dir(dst), 0o750), "create parent dir for %s", rel)
			require.NoError(t, os.WriteFile(dst, []byte(src), 0o600), "write probe file %s", rel)
		}

		cmd := exec.CommandContext(t.Context(), "gofmt", "-l", "cmd", "internal", "tests", "scripts")
		cmd.Dir = probeDir
		out, err := cmd.CombinedOutput()
		assert.NoError(t, err, "scoped gofmt must succeed when .copilot contains invalid Go; output:\n%s", string(out))
		assert.Empty(t, strings.TrimSpace(string(out)), "scoped gofmt must report no unformatted files")

		if _, lookErr := exec.LookPath("pwsh"); lookErr == nil {
			pwshCmd := exec.CommandContext(t.Context(), "pwsh", "-NoProfile", "-NonInteractive", "-File", filepath.Join(repoRoot, "make.ps1"), "fmt")
			pwshCmd.Dir = probeDir
			pwshOut, pwshErr := pwshCmd.CombinedOutput()
			outStr := string(pwshOut)
			assert.NoError(t, pwshErr, "pwsh make.ps1 fmt must succeed with isolated .copilot dir; output:\n%s", outStr)
			assert.Contains(t, outStr, "All files formatted.", "pwsh make.ps1 fmt must report all files formatted")
			assert.NotContains(t, outStr, "FAILED:", "pwsh make.ps1 fmt must not report failure")
			assert.NotContains(t, outStr, ".copilot", "pwsh make.ps1 fmt must not inspect .copilot")
			assert.NotContains(t, outStr, ".go", "pwsh make.ps1 fmt must not report any .go files")
		} else {
			t.Logf("pwsh not found on PATH (%v); skipping pwsh make.ps1 fmt execution", lookErr)
		}
	})
}
