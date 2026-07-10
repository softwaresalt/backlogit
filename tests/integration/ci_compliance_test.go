package integration_test

// F013/F090: GitHub Actions workflow compliance characterization tests.
//
// These tests validate repository CI workflow invariants that keep required
// status checks satisfiable while reducing duplicate GitHub Actions cost.

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	On          ciOn             `yaml:"on"`
	Concurrency ciConcurrency    `yaml:"concurrency"`
	Jobs        map[string]ciJob `yaml:"jobs"`
}

type ciOn struct {
	Push        ciEvent `yaml:"push"`
	PullRequest ciEvent `yaml:"pull_request"`
	MergeGroup  ciEvent `yaml:"merge_group"`
}

type ciEvent struct {
	Branches []string `yaml:"branches"`
	Tags     []string `yaml:"tags"`
}

type ciConcurrency struct {
	Group            string `yaml:"group"`
	CancelInProgress bool   `yaml:"cancel-in-progress"`
}

type ciJob struct {
	Name     string            `yaml:"name"`
	Needs    any               `yaml:"needs"`
	If       string            `yaml:"if"`
	Uses     string            `yaml:"uses"`
	RunsOn   string            `yaml:"runs-on"`
	Outputs  map[string]string `yaml:"outputs"`
	Strategy ciStrategy        `yaml:"strategy"`
	Steps    []ciStep          `yaml:"steps"`
}

type ciStrategy struct {
	Matrix map[string]any `yaml:"matrix"`
}

type ciStep struct {
	ID   string         `yaml:"id"`
	Name string         `yaml:"name"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	If   string         `yaml:"if"`
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

func requireYAMLString(t *testing.T, value any, label string) string {
	t.Helper()
	str, ok := value.(string)
	require.Truef(t, ok, "%s should be a string, got %T", label, value)
	return str
}

// workflowPaths returns absolute paths to workflow files involved in CI policy.
func workflowPaths(t *testing.T) (ciPath, releasePath, driftPath string) {
	t.Helper()
	root := findRepoRoot(t)
	workflowDir := filepath.Join(root, ".github", "workflows")
	return filepath.Join(workflowDir, "ci.yml"),
		filepath.Join(workflowDir, "release.yml"),
		filepath.Join(workflowDir, "cli-reference-drift.yml")
}

func existingWorkflowPaths(t *testing.T) []string {
	t.Helper()
	root := findRepoRoot(t)
	paths, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	require.NoError(t, err)
	sort.Strings(paths)
	return paths
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err, "read file: %s", path)
	return string(data)
}

func findStep(t *testing.T, job ciJob, name string) ciStep {
	t.Helper()
	for _, step := range job.Steps {
		if step.Name == name {
			return step
		}
	}
	require.FailNowf(t, "missing step", "step %q not found", name)
	return ciStep{}
}

func findStepByID(t *testing.T, job ciJob, id string) ciStep {
	t.Helper()
	for _, step := range job.Steps {
		if step.ID == id {
			return step
		}
	}
	require.FailNowf(t, "missing step", "step id %q not found", id)
	return ciStep{}
}

func findSetupGoStep(t *testing.T, job ciJob) ciStep {
	t.Helper()
	for _, step := range job.Steps {
		if strings.Contains(step.Uses, "actions/setup-go") {
			return step
		}
	}
	require.FailNow(t, "setup-go step not found")
	return ciStep{}
}

func setupGoVersion(t *testing.T, step ciStep) string {
	t.Helper()
	value, ok := step.With["go-version"]
	require.True(t, ok, "setup-go step should set go-version")
	return requireYAMLString(t, value, "setup-go go-version")
}

func extractTopLevelBlock(content, key string) string {
	var builder strings.Builder
	prefix := key + ":"
	inBlock := false
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if !inBlock {
			if line == prefix {
				inBlock = true
				builder.WriteString(line)
				builder.WriteByte('\n')
			}
			continue
		}
		if trimmed != "" && !strings.HasPrefix(line, " ") && strings.HasSuffix(trimmed, ":") {
			break
		}
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func assertNoTriggerPathFiltering(t *testing.T, path string) {
	t.Helper()
	onBlock := extractTopLevelBlock(readFileString(t, path), "on")
	require.NotEmpty(t, onBlock, "workflow should have a parseable on: block")
	pathKey := regexp.MustCompile(`(?m)^\s+paths(-ignore)?:\s*`)
	assert.False(t, pathKey.MatchString(onBlock), "required workflows must not use trigger-level paths filters")
}

func assertNoTriggerEvent(t *testing.T, path, event string) {
	t.Helper()
	onBlock := extractTopLevelBlock(readFileString(t, path), "on")
	require.NotEmpty(t, onBlock, "workflow should have a parseable on: block")
	eventKey := regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(event) + `:\s*(?:$|\{)`)
	assert.Falsef(t, eventKey.MatchString(onBlock), "workflow must not define on.%s", event)
}

func assertContainsAll(t *testing.T, content string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		assert.Contains(t, content, fragment)
	}
}

func countActionUses(wf ciWorkflow, action string) int {
	count := 0
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(step.Uses, action) {
				count++
			}
		}
	}
	return count
}

// --- F013.T001.ST001: Tag trigger uses glob, not regex ---

// TestReleaseTagTriggerUsesGlob validates that release.yml uses GitHub Actions
// glob syntax for the tag trigger, not regex character classes.
func TestReleaseTagTriggerUsesGlob(t *testing.T) {
	_, releasePath, _ := workflowPaths(t)
	content := readFileString(t, releasePath)

	regexCharClass := regexp.MustCompile(`\[0-9\]`)
	matches := regexCharClass.FindAllString(content, -1)
	assert.Empty(t, matches,
		"Tag trigger should use glob syntax, not regex character classes like [0-9]")
	assert.Contains(t, content, `v*.*.*`,
		"Tag trigger should contain a glob pattern like v*.*.*")
}

// --- F013.T002.ST001 / F090: Go version matches go.mod and CI target ---

// TestWorkflowGoVersionMatchesMod validates that CI and release workflow files
// reference the Go support target from go.mod without a duplicate CI matrix.
func TestWorkflowGoVersionMatchesMod(t *testing.T) {
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

	parts := strings.SplitN(goVersion, ".", 3)
	require.GreaterOrEqual(t, len(parts), 2, "Go version should have at least major.minor")
	modMinor := parts[0] + "." + parts[1]

	ciPath, releasePath, _ := workflowPaths(t)

	t.Run("ci test job uses support target without matrix", func(t *testing.T) {
		wf := readCIWorkflow(t, ciPath)
		testJob, ok := wf.Jobs["test"]
		require.True(t, ok, "ci.yml should have a 'test' job")
		assert.Empty(t, testJob.Strategy.Matrix, "test must be non-matrix so the required context is bare 'test'")
		assert.Equal(t, modMinor, setupGoVersion(t, findSetupGoStep(t, testJob)))
	})

	t.Run("release quality gate uses support target without matrix", func(t *testing.T) {
		wf := readCIWorkflow(t, releasePath)
		testJob, ok := wf.Jobs["test"]
		require.True(t, ok, "release.yml should have a 'test' job")
		assert.Empty(t, testJob.Strategy.Matrix, "release quality gate should not duplicate a Go-version matrix")
		assert.Equal(t, modMinor, setupGoVersion(t, findSetupGoStep(t, testJob)))
	})

	t.Run("release build job uses go.mod version", func(t *testing.T) {
		wf := readCIWorkflow(t, releasePath)
		buildJob, ok := wf.Jobs["build"]
		require.True(t, ok, "release.yml should have a 'build' job")
		assert.Equal(t, modMinor, setupGoVersion(t, findSetupGoStep(t, buildJob)))
	})
}

// --- F013.T003.ST002/ST003: All third-party actions use SHA pins ---

var shaPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

// TestAllActionsUseSHAPins validates that every third-party action reference in
// every workflow uses a full 40-character commit SHA.
func TestAllActionsUseSHAPins(t *testing.T) {
	for _, wfPath := range existingWorkflowPaths(t) {
		t.Run(filepath.Base(wfPath), func(t *testing.T) {
			wf := readCIWorkflow(t, wfPath)

			for jobName, job := range wf.Jobs {
				if job.Uses != "" && !strings.HasPrefix(job.Uses, "./") {
					atIdx := strings.LastIndex(job.Uses, "@")
					require.NotEqual(t, -1, atIdx,
						"job %s reusable workflow %q missing @ separator", jobName, job.Uses)

					ref := strings.TrimSpace(job.Uses[atIdx+1:])
					assert.Truef(t, shaPattern.MatchString(ref),
						"job %s reusable workflow %s should use a 40-char SHA pin (got %q)",
						jobName, job.Uses, ref)
				}

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
	for _, wfPath := range existingWorkflowPaths(t) {
		t.Run(filepath.Base(wfPath), func(t *testing.T) {
			wf := readCIWorkflow(t, wfPath)

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

// TestWorkflowsDeclareConcurrency validates workflow concurrency settings.
func TestWorkflowsDeclareConcurrency(t *testing.T) {
	ciPath, releasePath, _ := workflowPaths(t)

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
	versionPattern := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

	for _, wfPath := range existingWorkflowPaths(t) {
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

					versionStr := requireYAMLString(t, version, "golangci-lint version")
					assert.Truef(t, versionPattern.MatchString(versionStr),
						"job %s step %d: golangci-lint version should be pinned, got %q", jobName, i, versionStr)
				}
			}
		})
	}
}

// TestCITriggerModelAvoidsPushDoubleRun validates that required PR checks are
// not duplicated by a push-to-main CI trigger.
func TestCITriggerModelAvoidsPushDoubleRun(t *testing.T) {
	ciPath, _, driftPath := workflowPaths(t)
	wf := readCIWorkflow(t, ciPath)

	assert.Contains(t, wf.On.PullRequest.Branches, "main", "CI must report required contexts on PRs")
	assert.Empty(t, wf.On.Push.Branches, "CI must not run the same validation again on push to main")
	assert.Empty(t, wf.On.Push.Tags, "CI push trigger must remain absent")
	assertNoTriggerEvent(t, ciPath, "push")
	assertNoTriggerPathFiltering(t, ciPath)

	_, err := os.Stat(driftPath)
	assert.True(t, os.IsNotExist(err), "CLI Reference Drift must be consolidated into ci.yml, not a separate workflow")
}

// TestRequiredPRContextsStillReport validates that branch-protection contexts
// remain backed by always-reporting jobs on pull requests.
func TestRequiredPRContextsStillReport(t *testing.T) {
	ciPath, _, _ := workflowPaths(t)
	wf := readCIWorkflow(t, ciPath)

	changes, ok := wf.Jobs["changes"]
	require.True(t, ok, "ci.yml should define the changes job")
	assert.Equal(t, "Detect code changes", changes.Name)
	assert.Empty(t, changes.If, "Detect code changes must not be job-gated")

	testJob, ok := wf.Jobs["test"]
	require.True(t, ok, "ci.yml should define the required test job")
	assert.Equal(t, "${{ always() && !cancelled() }}", testJob.If, "test must report even if change detection fails")
	assert.Empty(t, testJob.Name, "test job should emit the bare required 'test' context")

	docsLint, ok := wf.Jobs["docs-lint"]
	require.True(t, ok, "ci.yml should define the docs-lint job")
	assert.Equal(t, "Docline frontmatter gate", docsLint.Name)
	assert.Equal(t, "${{ always() && !cancelled() }}", docsLint.If, "docs-lint must report even if change detection fails")

	drift, ok := wf.Jobs["cli-reference-drift"]
	require.True(t, ok, "ci.yml should define the consolidated CLI drift job")
	assert.Equal(t, "CLI Reference Drift", drift.Name)
	assert.Equal(t, "${{ always() && !cancelled() }}", drift.If, "CLI drift must report even if change detection fails")
}

// TestCIChangeDetectorFailSafeOutputs validates the shared change detector that
// gates heavy steps without suppressing required job contexts.
func TestCIChangeDetectorFailSafeOutputs(t *testing.T) {
	ciPath, _, _ := workflowPaths(t)
	wf := readCIWorkflow(t, ciPath)
	changes := wf.Jobs["changes"]

	assert.Equal(t, "${{ steps.classify.outputs.code }}", changes.Outputs["code"])
	assert.Equal(t, "${{ steps.classify.outputs.docline_required }}", changes.Outputs["docline_required"])
	assert.Equal(t, "${{ steps.cli.outputs.cli_reference }}", changes.Outputs["cli_reference"])

	classify := findStepByID(t, changes, "classify")
	require.Contains(t, classify.Uses, "dorny/paths-filter")
	assert.Equal(t, "every", requireYAMLString(t, classify.With["predicate-quantifier"], "predicate-quantifier"))
	classifyFilters := requireYAMLString(t, classify.With["filters"], "classify filters")
	assertContainsAll(t, classifyFilters,
		"code:", "- '**'", "- '!**/*.md'", "- '!docs/**'", "- '!.backlogit/**'",
		"docline_required:", "- '!internal/db/**'", "- '!internal/mcp/**'",
		"- '!internal/models/**'", "- '!internal/telemetry/**'", "- '!internal/version/**'", "- '!tests/**'",
		"- '!.backlogit/**'",
	)
	assert.NotContains(t, classifyFilters, "!internal/core/**", "core changes can affect docline SafeResolve behavior")
	assert.NotContains(t, classifyFilters, "!internal/docline/**", "docline implementation changes must run docline lint")
	assert.NotContains(t, classifyFilters, "!internal/mdfront/**", "frontmatter codec changes must run docline lint")
	assert.NotContains(t, classifyFilters, "!internal/cli/docs.go", "docs command changes must run docline lint")

	cli := findStepByID(t, changes, "cli")
	require.Contains(t, cli.Uses, "dorny/paths-filter")
	cliFilters := requireYAMLString(t, cli.With["filters"], "cli filters")
	assertContainsAll(t, cliFilters,
		"cli_reference:", "- '**/*.go'", "- 'go.mod'", "- 'go.sum'", "- 'cmd/gen-docs/**'",
		"- 'docs/cli-reference/**'", "- '.github/workflows/ci.yml'",
	)
}

// TestHeavyStepsAreFailSafeGated validates that expensive CI work runs unless
// a change is provably irrelevant, while required job contexts still succeed.
func TestHeavyStepsAreFailSafeGated(t *testing.T) {
	ciPath, _, _ := workflowPaths(t)
	wf := readCIWorkflow(t, ciPath)

	testJob := wf.Jobs["test"]
	assert.Equal(t, "needs.changes.outputs.code == 'false'", findStep(t, testJob, "Skip Go gates for docs/backlog-only changes").If)
	for _, stepName := range []string{"Checkout", "Setup Go 1.24", "Install dependencies", "Lint", "Vet", "Test", "Coverage report"} {
		step := findStep(t, testJob, stepName)
		assert.Equal(t, "needs.changes.outputs.code != 'false'", step.If, "%s should fail safe toward running", stepName)
	}
	assert.Equal(t, 1, countActionUses(wf, "golangci/golangci-lint-action"), "golangci-lint should run at most once per CI workflow")

	docsLint := wf.Jobs["docs-lint"]
	assert.Equal(t, "needs.changes.outputs.docline_required == 'false'", findStep(t, docsLint, "Skip docline lint for code-only changes").If)
	for _, stepName := range []string{"Checkout", "Setup Go for docline lint", "Lint documentation frontmatter"} {
		step := findStep(t, docsLint, stepName)
		assert.Equal(t, "needs.changes.outputs.docline_required != 'false'", step.If, "%s should run on docs, frontmatter, unknown, or detector failure", stepName)
	}

	drift := wf.Jobs["cli-reference-drift"]
	assert.Equal(t,
		"needs.changes.outputs.cli_reference == 'false'",
		findStep(t, drift, "Skip CLI reference drift for irrelevant changes").If)
	for _, stepName := range []string{"Checkout", "Setup Go for CLI reference", "Generate CLI reference", "Check for drift"} {
		step := findStep(t, drift, stepName)
		assert.Equal(t,
			"needs.changes.outputs.cli_reference != 'false'",
			step.If, "%s should run for CLI-relevant changes or detector failure", stepName)
	}
}

// TestReleaseWorkflowDropsRaceMatrix validates that release tags keep a light
// quality gate without re-running the protected-main race and coverage signal.
func TestReleaseWorkflowDropsRaceMatrix(t *testing.T) {
	_, releasePath, _ := workflowPaths(t)
	wf := readCIWorkflow(t, releasePath)
	testJob, ok := wf.Jobs["test"]
	require.True(t, ok, "release.yml should keep a lightweight test job")

	assert.Empty(t, testJob.Strategy.Matrix, "release quality gate should not fan out a Go-version matrix")
	provenance := findStep(t, testJob, "Verify tag commit is on protected main")
	assert.Contains(t, provenance.Run, "git merge-base --is-ancestor")
	assert.Contains(t, provenance.Run, "origin/main")
	for _, step := range testJob.Steps {
		assert.NotContains(t, step.Run, "-race", "release quality gate should not duplicate race testing")
		assert.NotContains(t, step.Run, "coverprofile", "release quality gate should not duplicate coverage generation")
	}
	assert.Contains(t, readFileString(t, releasePath), "Race and coverage run in protected PR CI", "release workflow should document the signal source")

	for jobName, job := range wf.Jobs {
		assert.Equal(t, "ubuntu-latest", job.RunsOn, "release job %s must not introduce macOS or Windows runners", jobName)
	}
}
