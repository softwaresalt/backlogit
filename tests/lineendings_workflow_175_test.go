package tests_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const u014WorkflowRel = ".github/workflows/line-endings.yml"

// u014CheckoutSHA is the parsed `uses:` value (the trailing "# v4.2.2" is a YAML
// comment and is not part of the parsed value).
const u014CheckoutSHA = "actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683"

const u014ValidWorkflow = `name: Line Endings
on:
  pull_request:
    branches:
      - main
  push:
    branches:
      - main
permissions:
  contents: read
concurrency:
  group: line-endings-${{ github.ref }}
  cancel-in-progress: true
jobs:
  line-endings:
    runs-on: windows-latest
    steps:
      - name: Checkout
        uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2
        with:
          persist-credentials: false
      - name: Guard
        shell: pwsh
        run: pwsh scripts/check-line-endings.ps1
`

type u014Trigger struct {
	Branches    []string `yaml:"branches"`
	Paths       []string `yaml:"paths"`
	PathsIgnore []string `yaml:"paths-ignore"`
}

type u014Step struct {
	Uses string         `yaml:"uses"`
	With map[string]any `yaml:"with"`
	Run  string         `yaml:"run"`
}

type u014Job struct {
	RunsOn string     `yaml:"runs-on"`
	Needs  any        `yaml:"needs"`
	If     string     `yaml:"if"`
	Steps  []u014Step `yaml:"steps"`
}

type u014Workflow struct {
	On struct {
		PullRequest *u014Trigger `yaml:"pull_request"`
		Push        *u014Trigger `yaml:"push"`
	} `yaml:"on"`
	Permissions map[string]string `yaml:"permissions"`
	Concurrency struct {
		Group            string `yaml:"group"`
		CancelInProgress bool   `yaml:"cancel-in-progress"`
	} `yaml:"concurrency"`
	Jobs map[string]u014Job `yaml:"jobs"`
}

func u014HasMain(tr *u014Trigger) bool {
	if tr == nil {
		return false
	}
	for _, b := range tr.Branches {
		if b == "main" {
			return true
		}
	}
	return false
}

// u014Validate returns an error describing the first violated contract element.
func u014Validate(raw []byte) error {
	var wf u014Workflow
	if err := yaml.Unmarshal(raw, &wf); err != nil {
		return fmt.Errorf("malformed workflow: %w", err)
	}
	if !u014HasMain(wf.On.PullRequest) || !u014HasMain(wf.On.Push) {
		return fmt.Errorf("pull_request and push triggers on main are required")
	}
	for _, tr := range []*u014Trigger{wf.On.PullRequest, wf.On.Push} {
		if len(tr.Paths) > 0 || len(tr.PathsIgnore) > 0 {
			return fmt.Errorf("trigger-level paths filter forbidden")
		}
	}
	if wf.Permissions["contents"] != "read" {
		return fmt.Errorf("permissions: contents: read required")
	}
	if wf.Concurrency.Group != "line-endings-${{ github.ref }}" || !wf.Concurrency.CancelInProgress {
		return fmt.Errorf("safe concurrency group with cancel-in-progress required")
	}
	if len(wf.Jobs) != 1 {
		return fmt.Errorf("exactly one job required, got %d", len(wf.Jobs))
	}
	for name, job := range wf.Jobs {
		if job.RunsOn != "windows-latest" {
			return fmt.Errorf("job %s must run on windows-latest", name)
		}
		if job.Needs != nil || job.If != "" {
			return fmt.Errorf("job %s must have no needs/if skip gate", name)
		}
		checkoutIdx, guardIdx := -1, -1
		for i, st := range job.Steps {
			if strings.HasPrefix(st.Uses, "actions/checkout@") {
				if st.Uses != u014CheckoutSHA {
					return fmt.Errorf("checkout must be pinned to %s, got %s", u014CheckoutSHA, st.Uses)
				}
				checkoutIdx = i
				if v, ok := st.With["persist-credentials"].(bool); !ok || v {
					return fmt.Errorf("checkout persist-credentials must be false")
				}
			}
			if strings.Contains(st.Run, "scripts/check-line-endings.ps1") {
				guardIdx = i
			}
		}
		if checkoutIdx < 0 || guardIdx < 0 || guardIdx < checkoutIdx {
			return fmt.Errorf("guard scripts/check-line-endings.ps1 must be invoked after checkout")
		}
	}
	if strings.Contains(string(raw), "needs.changes") {
		return fmt.Errorf("needs.changes skip filter forbidden")
	}
	return nil
}

func u014Root(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// TestU014_unused guards against accidental name drift of the fixture constant.
var _ = u014ValidWorkflow

// TestU175_014_Workflow covers the three contract scenarios plus the committed file.
func TestU175_014_Workflow(t *testing.T) {
	t.Run("valid_secure_always_run_green", func(t *testing.T) {
		if err := u014Validate([]byte(u014ValidWorkflow)); err != nil {
			t.Fatalf("valid fixture rejected: %v", err)
		}
	})
	t.Run("missing_or_malformed_invocation_red", func(t *testing.T) {
		mutants := map[string]string{
			"no_guard":     strings.Replace(u014ValidWorkflow, "pwsh scripts/check-line-endings.ps1", "echo skip", 1),
			"wrong_runner": strings.Replace(u014ValidWorkflow, "windows-latest", "ubuntu-latest", 1),
			"malformed":    "on: [unterminated\n",
		}
		for name, m := range mutants {
			if err := u014Validate([]byte(m)); err == nil {
				t.Errorf("mutant %s unexpectedly accepted", name)
			}
		}
		if err := u014Validate(nil); err == nil {
			t.Error("empty workflow unexpectedly accepted")
		}
	})
	t.Run("insecure_or_skippable_contract_red", func(t *testing.T) {
		mutants := map[string]string{
			"no_permissions":              strings.Replace(u014ValidWorkflow, "permissions:\n  contents: read\n", "", 1),
			"unpinned_old_pin_in_comment": strings.Replace(u014ValidWorkflow, "        uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2\n", "        # uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2\n        uses: actions/checkout@v4\n", 1),
			"unpinned":                    strings.Replace(u014ValidWorkflow, "11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2", "v4", 1),
			"persist_creds":               strings.Replace(u014ValidWorkflow, "persist-credentials: false", "persist-credentials: true", 1),
			"no_push":                     strings.Replace(u014ValidWorkflow, "  push:\n    branches:\n      - main\n", "", 1),
			"paths_filter":                strings.Replace(u014ValidWorkflow, "  pull_request:\n    branches:\n      - main\n", "  pull_request:\n    branches:\n      - main\n    paths:\n      - '**.go'\n", 1),
			"needs_changes":               strings.Replace(u014ValidWorkflow, "    runs-on: windows-latest\n", "    needs: changes\n    if: needs.changes.outputs.code == 'true'\n    runs-on: windows-latest\n", 1),
			"no_concurrency":              strings.Replace(u014ValidWorkflow, "concurrency:\n  group: line-endings-${{ github.ref }}\n  cancel-in-progress: true\n", "", 1),
			"no_cancel":                   strings.Replace(u014ValidWorkflow, "cancel-in-progress: true", "cancel-in-progress: false", 1),
		}
		for name, m := range mutants {
			if m == u014ValidWorkflow {
				t.Fatalf("mutant %s did not change the fixture", name)
			}
			if err := u014Validate([]byte(m)); err == nil {
				t.Errorf("mutant %s unexpectedly accepted", name)
			}
		}
	})
	t.Run("committed_workflow_satisfies_contract", func(t *testing.T) {
		raw, err := os.ReadFile(filepath.Join(u014Root(t), filepath.FromSlash(u014WorkflowRel)))
		if err != nil {
			t.Fatalf("committed workflow missing: %v", err)
		}
		if err := u014Validate(raw); err != nil {
			t.Fatalf("committed workflow violates contract: %v", err)
		}
	})
}
