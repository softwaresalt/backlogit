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
	"gopkg.in/yaml.v3"
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

func TestU9_WorkspaceProfileFormatGateScopesSourceDirectories(t *testing.T) {
	type workspaceProfileDoc struct {
		Format struct {
			Tool         string `yaml:"tool"`
			Command      string `yaml:"command"`
			CheckCommand string `yaml:"check_command"`
			FixCommand   string `yaml:"fix_command"`
		} `yaml:"format"`
		Test struct {
			Runner    string `yaml:"runner"`
			Command   string `yaml:"command"`
			Directory string `yaml:"directory"`
		} `yaml:"test"`
	}

	repoRoot := testRepoRoot(t)

	t.Run("YAML key uniqueness", func(t *testing.T) {
		var dupNode yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte("a: 1\na: 2\n"), &dupNode))
		require.Error(t, checkUniqueYAMLMappingKeys(&dupNode), "checkUniqueYAMLMappingKeys must reject duplicate YAML mapping keys")

		raw := readGovernedSurface(t, repoRoot, ".gsd/workspace-profile.yaml")
		profile := decodeUniqueYAML[workspaceProfileDoc](t, ".gsd/workspace-profile.yaml", raw)
		assert.NotEmpty(t, profile.Format.Command, ".gsd/workspace-profile.yaml format.command must not be empty")
		assert.NotEmpty(t, profile.Test.Command, ".gsd/workspace-profile.yaml test.command must not be empty")
	})

	t.Run("format commands and test budget invariants", func(t *testing.T) {
		raw := readGovernedSurface(t, repoRoot, ".gsd/workspace-profile.yaml")
		profile := decodeUniqueYAML[workspaceProfileDoc](t, ".gsd/workspace-profile.yaml", raw)

		assert.Equal(t, "gofmt -l cmd internal tests scripts", profile.Format.Command, "format.command must scope gofmt to Go source directories")
		assert.Equal(t, "gofmt -l cmd internal tests scripts", profile.Format.CheckCommand, "format.check_command must scope gofmt to Go source directories")
		assert.Equal(t, "gofmt -w cmd internal tests scripts", profile.Format.FixCommand, "format.fix_command must scope gofmt to Go source directories")
		assert.Equal(t, "go test -timeout=30m ./...", profile.Test.Command, "test.command must enforce 30m timeout")
		assert.Equal(t, "go test", profile.Test.Runner, "test.runner must be go test")
	})
}

func TestU10_HarnessManifestFormatGateAndChecksumsReconciled(t *testing.T) {
	type manifestArtifact struct {
		Path         string `yaml:"path"`
		Checksum     string `yaml:"checksum"`
		DriftAllowed bool   `yaml:"drift_allowed"`
		DriftReason  string `yaml:"drift_reason"`
		Note         string `yaml:"note"`
	}

	type manifestDoc struct {
		ProfileHash   string             `yaml:"profile_hash"`
		VariablesUsed map[string]string  `yaml:"variables_used"`
		Artifacts     []manifestArtifact `yaml:"artifacts"`
	}

	repoRoot := testRepoRoot(t)

	indexArtifacts := func(t *testing.T, artifacts []manifestArtifact) map[string]manifestArtifact {
		t.Helper()
		byPath := make(map[string]manifestArtifact, len(artifacts))
		for _, entry := range artifacts {
			require.NotEmpty(t, entry.Path, "manifest artifact path must not be empty")
			_, dup := byPath[entry.Path]
			require.False(t, dup, "manifest must not contain duplicate artifact path %s", entry.Path)
			byPath[entry.Path] = entry
		}
		return byPath
	}

	t.Run("manifest format gate and drift records", func(t *testing.T) {
		var dupNode yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte("a: 1\na: 2\n"), &dupNode))
		require.Error(t, checkUniqueYAMLMappingKeys(&dupNode), "checkUniqueYAMLMappingKeys must reject duplicate YAML mapping keys")

		rawManifest := readGovernedSurface(t, repoRoot, ".gsd/harness-manifest.yaml")
		manifest := decodeUniqueYAML[manifestDoc](t, ".gsd/harness-manifest.yaml", rawManifest)

		rawProfile := readGovernedSurface(t, repoRoot, ".gsd/workspace-profile.yaml")
		assert.Equal(t, lfSHA256Hex(rawProfile), manifest.ProfileHash,
			"harness-manifest.yaml profile_hash must match LF-normalized .gsd/workspace-profile.yaml")

		assert.Equal(t, "gofmt -l cmd internal tests scripts", manifest.VariablesUsed["FORMAT_COMMAND"],
			"variables_used.FORMAT_COMMAND must scope gofmt to Go source directories")

		summary := manifest.VariablesUsed["HARNESS_ENFORCED_SUMMARY"]
		assert.Contains(t, summary, "(`gofmt -l cmd internal tests scripts`, `golangci-lint run`, `go vet ./...`)",
			"variables_used.HARNESS_ENFORCED_SUMMARY must reference scoped gofmt with golangci-lint and go vet")
		assert.Contains(t, summary, "go test -timeout=30m ./...",
			"variables_used.HARNESS_ENFORCED_SUMMARY must reference go test -timeout=30m ./...")
		assert.NotContains(t, summary, "gofmt -l .",
			"variables_used.HARNESS_ENFORCED_SUMMARY must not contain unscoped gofmt -l .")

		byPath := indexArtifacts(t, manifest.Artifacts)
		for _, path := range []string{
			".github/instructions/constitution.instructions.md",
			".github/instructions/go.instructions.md",
		} {
			entry, exists := byPath[path]
			require.True(t, exists, "manifest artifacts[] must contain %s", path)
			assert.True(t, entry.DriftAllowed, "drift_allowed must be true for %s", path)
			assert.Contains(t, entry.DriftReason, "Do not auto-revert.",
				"drift_reason for %s must contain \"Do not auto-revert.\"", path)
			assert.Contains(t, entry.DriftReason, "gofmt -l cmd internal tests scripts",
				"drift_reason for %s must reference \"gofmt -l cmd internal tests scripts\"", path)
		}

		preservedCitations := []string{
			"073-DL rev22",
			"PR #400",
			"227A2930",
			"B83081F5",
			"75E02C17",
			"F05661B1",
			"155-S",
		}
		for _, citation := range preservedCitations {
			assert.Contains(t, rawManifest, citation,
				"harness-manifest.yaml must preserve historical note/drift citation %q", citation)
		}
	})

	t.Run("manifest checksum parity", func(t *testing.T) {
		rawManifest := readGovernedSurface(t, repoRoot, ".gsd/harness-manifest.yaml")
		manifest := decodeUniqueYAML[manifestDoc](t, ".gsd/harness-manifest.yaml", rawManifest)
		require.NotEmpty(t, manifest.Artifacts, "manifest artifacts[] must not be empty")

		byPath := indexArtifacts(t, manifest.Artifacts)

		for _, entry := range manifest.Artifacts {
			diskContent := readGovernedSurface(t, repoRoot, entry.Path)
			assert.Equal(t, lfSHA256Hex(diskContent), entry.Checksum,
				"manifest checksum for %s must match disk LF SHA-256", entry.Path)
		}

		modifiedTracked := []string{
			"AGENTS.md",
			".github/copilot-instructions.md",
			".github/instructions/constitution.instructions.md",
			".github/instructions/go.instructions.md",
			".github/agents/_ship.agent.md",
			".github/agents/subagents/go-engineer.agent.md",
			".github/policies/workflow-policies.md",
			".github/instructions/copilot-code-review.instructions.md",
			".github/instructions/github-pr-automation.instructions.md",
		}
		for _, relPath := range modifiedTracked {
			entry, exists := byPath[relPath]
			require.True(t, exists, "manifest artifacts[] must include modified artifact %s", relPath)
			diskContent := readGovernedSurface(t, repoRoot, relPath)
			assert.Equal(t, lfSHA256Hex(diskContent), entry.Checksum,
				"manifest checksum for modified artifact %s must match disk LF SHA-256", relPath)
		}
	})

	t.Run("14-surface scoped gofmt parity", func(t *testing.T) {
		unscopedGofmtPattern := regexp.MustCompile(`\bgofmt\s+-[lw]\s+\.(?:$|\s|[;|&)"'` + "`" + `])`)
		unscopedGoimportsPattern := regexp.MustCompile(`\bgoimports\s+-l\s+\.(?:$|\s|[;|&)"'` + "`" + `])`)

		require.True(t, unscopedGofmtPattern.MatchString("`gofmt -l .`"), "unscopedGofmtPattern must match inline code span")
		require.True(t, unscopedGofmtPattern.MatchString("gofmt -w ."), "unscopedGofmtPattern must match gofmt -w .")
		require.False(t, unscopedGofmtPattern.MatchString("gofmt -l cmd internal tests scripts"), "unscopedGofmtPattern must not match scoped gofmt")
		require.True(t, unscopedGoimportsPattern.MatchString("goimports -l ."), "unscopedGoimportsPattern must match goimports -l .")
		require.False(t, unscopedGoimportsPattern.MatchString("goimports -l cmd internal tests scripts"), "unscopedGoimportsPattern must not match scoped goimports")

		surfaces := []struct {
			path     string
			required []string
		}{
			{path: "Makefile", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: "make.ps1", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: ".gsd/workspace-profile.yaml", required: []string{"gofmt -l cmd internal tests scripts", "gofmt -w cmd internal tests scripts"}},
			{path: "AGENTS.md", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: ".github/copilot-instructions.md", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: ".github/instructions/constitution.instructions.md", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: ".github/instructions/go.instructions.md", required: []string{"gofmt -l cmd internal tests scripts", "goimports -l cmd internal tests scripts"}},
			{path: ".github/agents/_ship.agent.md", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: ".github/agents/subagents/go-engineer.agent.md", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: ".github/skills/build-feature/SKILL.md", required: []string{"gofmt -l cmd internal tests scripts", "gofmt -w cmd internal tests scripts"}},
			{path: ".github/skills/fix-ci/SKILL.md", required: []string{"gofmt -l cmd internal tests scripts", "gofmt -w cmd internal tests scripts"}},
			{path: ".github/policies/workflow-policies.md", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: ".github/instructions/copilot-code-review.instructions.md", required: []string{"gofmt -l cmd internal tests scripts"}},
			{path: ".github/instructions/github-pr-automation.instructions.md", required: []string{"gofmt -l cmd internal tests scripts"}},
		}

		for _, s := range surfaces {
			t.Run(s.path, func(t *testing.T) {
				content := readGovernedSurface(t, repoRoot, s.path)
				for i, line := range strings.Split(content, "\n") {
					assert.False(t, unscopedGofmtPattern.MatchString(line),
						"%s line %d must not contain unscoped gofmt -l/-w .: %s", s.path, i+1, line)
					assert.False(t, unscopedGoimportsPattern.MatchString(line),
						"%s line %d must not contain unscoped goimports -l .: %s", s.path, i+1, line)
				}
				for _, req := range s.required {
					assert.True(t, strings.Contains(content, req), "%s must contain %q", s.path, req)
				}
			})
		}
	})
}
