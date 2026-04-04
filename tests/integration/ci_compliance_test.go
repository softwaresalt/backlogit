package integration_test

// F013: Release Pipeline Fix - CI compliance characterization tests.
//
// These tests validate GitHub Actions workflow YAML files against repository
// standards. They use a characterization-first posture: tests read the actual
// workflow files and assert correct values. Tests fail against the current
// broken state and pass after the worker edits the YAML.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// findRepoRoot walks up from the test working directory to locate the
// repository root by finding go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "could not find repository root (go.mod)")
		dir = parent
	}
}

// ciWorkflow represents the subset of a GitHub Actions workflow file needed
// for CI compliance validation.
type ciWorkflow struct {
	Concurrency ciConcurrency    `yaml:"concurrency"`
	Jobs        map[string]ciJob `yaml:"jobs"`
}

type ciConcurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress bool   `yaml:"cancel-in-progress"`
}

type ciJob struct {
	Steps []ciStep `yaml:"steps"`
}

type ciStep struct {
	Name string         `yaml:"name"`
	Uses string         `yaml:"uses"`
	With map[string]any `yaml:"with"`
}

// readCIWorkflow parses a GitHub Actions workflow YAML file into a ciWorkflow.
func readCIWorkflow(t *testing.T, path string) ciWorkflow {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read workflow file: %s", path)
	var wf ciWorkflow
	require.NoError(t, yaml.Unmarshal(data, &wf), "parse workflow YAML: %s", path)
	return wf
}

// workflowPaths returns the absolute paths to the CI and release workflow files.
func workflowPaths(t *testing.T) (ciPath, releasePath string) {
	t.Helper()
	root := findRepoRoot(t)
	return filepath.Join(root, ".github", "workflows", "ci.yml"),
		filepath.Join(root, ".github", "workflows", "release.yml")
}

// --- F013.T001.ST001: Tag trigger uses glob, not regex ---

// TestReleaseTagTriggerUsesGlob validates that release.yml uses GitHub Actions
// glob syntax for the tag trigger, not regex character classes.
func TestReleaseTagTriggerUsesGlob(t *testing.T) {
	// Arrange
	_, releasePath := workflowPaths(t)
	data, err := os.ReadFile(releasePath)
	require.NoError(t, err)
	content := string(data)

	// Act & Assert - no regex character classes
	regexCharClass := regexp.MustCompile(`\[0-9\]`)
	matches := regexCharClass.FindAllString(content, -1)
	assert.Empty(t, matches,
		"Tag trigger should use glob syntax, not regex character classes like [0-9]")

	// Act & Assert - glob pattern present
	assert.Contains(t, content, `v*.*.*`,
		"Tag trigger should contain a glob pattern like v*.*.*")
}

// --- F013.T002.ST001: Go version matches go.mod ---

// TestWorkflowGoVersionMatchesMod validates that CI and release workflow files
// reference a Go version matching what go.mod requires.
func TestWorkflowGoVersionMatchesMod(t *testing.T) {
	// Arrange - extract Go version from go.mod
	root := findRepoRoot(t)
	modData, err := os.ReadFile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)

	var goVersion string
	scanner := bufio.NewScanner(strings.NewReader(string(modData)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			goVersion = strings.TrimPrefix(line, "go ")
			break
		}
	}
	require.NoError(t, scanner.Err(), "scan go.mod")
	require.NotEmpty(t, goVersion, "go.mod should declare a Go version")

	// Extract major.minor (e.g., "1.24.0" → "1.24")
	parts := strings.SplitN(goVersion, ".", 3)
	require.GreaterOrEqual(t, len(parts), 2, "Go version should have at least major.minor")
	modMinor := parts[0] + "." + parts[1]

	ciPath, releasePath := workflowPaths(t)

	t.Run("ci.yml includes go.mod version", func(t *testing.T) {
		// Act
		ciData, err := os.ReadFile(ciPath)
		require.NoError(t, err)

		// Assert
		assert.Contains(t, string(ciData), modMinor,
			"ci.yml should include Go %s from go.mod in its version matrix", modMinor)
	})

	t.Run("release.yml includes go.mod version", func(t *testing.T) {
		// Act
		releaseData, err := os.ReadFile(releasePath)
		require.NoError(t, err)

		// Assert
		assert.Contains(t, string(releaseData), modMinor,
			"release.yml should include Go %s from go.mod in its version matrix", modMinor)
	})

	t.Run("release build job uses go.mod version", func(t *testing.T) {
		// Arrange
		wf := readCIWorkflow(t, releasePath)
		buildJob, ok := wf.Jobs["build"]
		require.True(t, ok, "release.yml should have a 'build' job")

		// Act - find the setup-go step
		var foundSetupGo bool
		for _, step := range buildJob.Steps {
			if !strings.Contains(step.Uses, "setup-go") {
				continue
			}
			foundSetupGo = true
			goVer, hasGoVersion := step.With["go-version"]
			require.True(t, hasGoVersion, "setup-go step should have go-version")

			// Assert
			goVerStr := fmt.Sprintf("%v", goVer)
			assert.Equal(t, modMinor, goVerStr,
				"build job go-version should match go.mod (%s)", modMinor)
		}
		assert.True(t, foundSetupGo, "build job should use actions/setup-go")
	})
}

// --- F013.T003.ST002/ST003: All third-party actions use SHA pins ---

var shaPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

// TestAllActionsUseSHAPins validates that every third-party action reference
// in both CI and release workflows uses a full 40-character commit SHA.
func TestAllActionsUseSHAPins(t *testing.T) {
	ciPath, releasePath := workflowPaths(t)

	for _, wfPath := range []string{ciPath, releasePath} {
		t.Run(filepath.Base(wfPath), func(t *testing.T) {
			// Arrange
			wf := readCIWorkflow(t, wfPath)

			// Act & Assert
			for jobName, job := range wf.Jobs {
				for i, step := range job.Steps {
					if step.Uses == "" || strings.HasPrefix(step.Uses, "./") {
						continue
					}
					if strings.HasPrefix(step.Uses, "docker://") {
						continue
					}

					atIdx := strings.LastIndex(step.Uses, "@")
					require.NotEqual(t, -1, atIdx,
						"job %s step %d: uses %q missing @ separator", jobName, i, step.Uses)

					ref := strings.TrimSpace(step.Uses[atIdx+1:])
					assert.Truef(t, shaPattern.MatchString(ref),
						"job %s step %d: %s should use a 40-char SHA pin (got %q)",
						jobName, i, step.Uses, ref)
				}
			}
		})
	}
}

// --- F013.T003.ST002/ST003: Checkout steps disable credential persistence ---

// TestCheckoutStepsNoPersistCredentials validates that all actions/checkout
// steps set persist-credentials: false per workflows.instructions.md.
func TestCheckoutStepsNoPersistCredentials(t *testing.T) {
	ciPath, releasePath := workflowPaths(t)

	for _, wfPath := range []string{ciPath, releasePath} {
		t.Run(filepath.Base(wfPath), func(t *testing.T) {
			// Arrange
			wf := readCIWorkflow(t, wfPath)

			// Act & Assert
			for jobName, job := range wf.Jobs {
				for i, step := range job.Steps {
					if !strings.Contains(step.Uses, "actions/checkout") {
						continue
					}

					persistCreds, ok := step.With["persist-credentials"]
					assert.True(t, ok,
						"job %s step %d: checkout should set persist-credentials", jobName, i)
					if ok {
						persistCredsBool, ok := persistCreds.(bool)
						require.True(t, ok,
							"job %s step %d: checkout persist-credentials should be a bool", jobName, i)
						assert.False(t, persistCredsBool,
							"job %s step %d: checkout persist-credentials should be false", jobName, i)
					}
				}
			}
		})
	}
}

// TestWorkflowsDeclareConcurrency validates that both workflow files define a
// concurrency block per workflows.instructions.md.
func TestWorkflowsDeclareConcurrency(t *testing.T) {
	ciPath, releasePath := workflowPaths(t)

	testCases := []struct {
		name             string
		path             string
		cancelInProgress bool
	}{
		{name: "ci.yml", path: ciPath, cancelInProgress: true},
		{name: "release.yml", path: releasePath, cancelInProgress: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wf := readCIWorkflow(t, tc.path)

			assert.Equal(t, "${{ github.workflow }}-${{ github.ref }}", wf.Concurrency.Group,
				"%s should use the standard concurrency group", tc.name)
			assert.Equal(t, tc.cancelInProgress, wf.Concurrency.CancelInProgress,
				"%s should set cancel-in-progress to the expected value", tc.name)
		})
	}
}

// TestGolangciLintVersionPinned validates that workflow files do not use a
// floating golangci-lint binary version.
func TestGolangciLintVersionPinned(t *testing.T) {
	ciPath, releasePath := workflowPaths(t)
	versionPattern := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

	for _, wfPath := range []string{ciPath, releasePath} {
		t.Run(filepath.Base(wfPath), func(t *testing.T) {
			wf := readCIWorkflow(t, wfPath)

			for jobName, job := range wf.Jobs {
				for i, step := range job.Steps {
					if !strings.Contains(step.Uses, "golangci/golangci-lint-action") {
						continue
					}

					version, ok := step.With["version"]
					require.True(t, ok,
						"job %s step %d: golangci-lint-action should set a version", jobName, i)

					versionStr := fmt.Sprintf("%v", version)
					assert.Truef(t, versionPattern.MatchString(versionStr),
						"job %s step %d: golangci-lint version should be pinned, got %q", jobName, i, versionStr)
				}
			}
		})
	}
}
