package integration_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract pins the caller/leaf
// boundary without importing an undeclared Go API or changing the old evaluator.
func TestU2j_RedDeliverablePreDispatchSnapshotZeroDeltaContract(t *testing.T) {
	root := testRepoRoot(t)
	const accepted = "RED_DELIVERABLE_ACCEPTED"
	const delta = "RED_DELIVERABLE_DELTA_OUT_OF_SURFACE"
	const production = "RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED"
	const unresolved = "WAVE_RED_MAPPING_UNRESOLVED"
	caller := []u2jSnapshotPath{
		u2jPath(".backlogit/queue/211.015-T.md", "active\n"),
		u2jPath(".backlogit/hooks_queue.jsonl", "{\"event\":\"WORK_STARTED\"}\n"),
		u2jPath("docs/memory/caller.md", "caller documentation\n"),
	}
	base := func(id, outcome string) u2jBranchControl {
		pre := u2jSnapshot(root, caller)
		post := u2jSnapshot(root, caller)
		return u2jBranchControl{
			ID: id, RedDeliverable: true,
			HarnessCommand:  "go test -count=1 -run '^TestU2i_' ./internal/cli",
			SelectorCommand: "go test -count=1 -run '^TestU2i_' ./internal/cli",
			Baseline:        u2jBaseline(), Head: u2jBaseline(), WorkspaceRoot: root, TaskID: "211.015-T",
			CompileOK: true, SelectorSignal: "assertion_fail", EvidenceComplete: true,
			ChangedFiles:   []string{caller[0].Path, caller[1].Path, caller[2].Path},
			UntrackedFiles: []string{}, PreDispatch: &pre, PostLeaf: &post,
			ExpectOutcome: outcome,
		}
	}

	t.Run("caller_claim_bookkeeping_unchanged_by_leaf_accepted", func(t *testing.T) {
		c := base("caller-claim-bookkeeping-zero-leaf-write", accepted)
		// The first two paths, baseline, and zero-write observation are the
		// 30908ecb counterexample recorded in _owned9d3d6df3/leaf-gate-result.json.
		c.ChangedFiles = c.ChangedFiles[:2]
		c.PreDispatch.Paths = c.PreDispatch.Paths[:2]
		c.PostLeaf.Paths = c.PostLeaf.Paths[:2]
		docs := base("caller-docs-zero-leaf-write", accepted)
		clean := base("clean-zero-leaf-write", accepted)
		clean.ChangedFiles = []string{}
		clean.PreDispatch.Paths = []u2jSnapshotPath{}
		clean.PostLeaf.Paths = []u2jSnapshotPath{}
		u2jReplay(t, root, []u2jBranchControl{c, docs, clean})
		t.Run("paused_amendment_resume_and_coupled_sources", func(t *testing.T) {
			u2jPausedResume(t, root)
			for _, source := range []struct {
				path, start, end string
				terms            []string
			}{
				{".github/skills/build-feature/SKILL.md", "### Step 0.5:", "### The Harness Loop",
					[]string{"red_predispatch_snapshot", "post-leaf", "red_baseline_sha"}},
				{".github/agents/_ship.agent.md", "#### Step 4.0:", "#### Step 4.1a:",
					[]string{"paused-red-deliverable"}},
				{".github/agents/_ship.agent.md", "#### Step 4.2:", "#### Step 4.3:",
					[]string{"red_predispatch_snapshot", "reload", "red_baseline_sha"}},
				{".github/agents/_ship.agent.md", "#### Step 4.5:", "#### Step 4.6:",
					[]string{"post-leaf", "red_predispatch_snapshot"}},
				{".github/policies/workflow-policies.md", "### P-002.6", "## P-003:",
					[]string{"red_predispatch_snapshot", "pre_dispatch_state", "post_leaf_state"}},
			} {
				raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source.path)))
				require.NoError(t, err)
				start := strings.Index(string(raw), source.start)
				require.NotEqual(t, -1, start, "existing contract section %s", source.start)
				section := string(raw)[start:]
				end := strings.Index(section, source.end)
				require.NotEqual(t, -1, end, "existing contract boundary %s", source.end)
				for _, term := range source.terms {
					assert.Contains(t, section[:end], term, "%s must align its dispatch boundary", source.path)
				}
			}
		})
	})

	t.Run("leaf_mutates_caller_modified_path_refused", func(t *testing.T) {
		controls := make([]u2jBranchControl, 0, 14)
		for _, pathIndex := range []int{0, 1} {
			for _, field := range []string{"byte", "index-mode", "worktree-mode", "index-blob", "staged", "unstaged", "delete"} {
				c := base(fmt.Sprintf("caller-%d-%s", pathIndex, field), delta)
				p := &c.PostLeaf.Paths[pathIndex]
				switch field {
				case "byte":
					p.WorktreeSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte("one-byte mutation")))
				case "index-mode":
					p.IndexMode = "100755"
				case "worktree-mode":
					p.WorktreeMode = "100755"
				case "index-blob":
					p.IndexBlobOID = strings.Repeat("b", 40)
				case "staged":
					p.StagedState = "M"
				case "unstaged":
					p.UnstagedState = ""
				case "delete":
					p.WorktreeExists = false
					p.WorktreeSHA256 = ""
					p.WorktreeMode = ""
					p.UnstagedState = "D"
				}
				controls = append(controls, c)
			}
		}
		u2jReplay(t, root, controls)
		t.Run("real_git_capture_compare", func(t *testing.T) { u2jRealGit(t, root) })
	})

	t.Run("leaf_adds_new_deleted_staged_untracked_config_or_docs_refused", func(t *testing.T) {
		controls := make([]u2jBranchControl, 0, 8)
		for _, path := range []string{".gitignore", ".backlogit/config.yaml", "docs/leaf.md", "tests/leaf_test.go"} {
			for _, state := range []string{"untracked", "staged"} {
				c := base(strings.ReplaceAll(path, "/", "-")+"-"+state, delta)
				p := u2jPath(path, "leaf\n")
				p.Untracked = state == "untracked"
				if p.Untracked {
					p.IndexMode, p.IndexBlobOID = "", ""
					p.UnstagedState = "??"
					c.UntrackedFiles = append(c.UntrackedFiles, path)
				} else {
					p.StagedState = "A"
					c.ChangedFiles = append(c.ChangedFiles, path)
				}
				c.PostLeaf.Paths = append(c.PostLeaf.Paths, p)
				controls = append(controls, c)
			}
		}
		// Removing an existing caller-untracked path must not be mistaken for
		// a clean post-leaf tree merely because no current status reports it.
		deleted := base("deleted-caller-untracked", delta)
		p := u2jPath("docs/caller-untracked.md", "caller\n")
		p.Untracked, p.UnstagedState, p.IndexMode, p.IndexBlobOID = true, "??", "", ""
		deleted.PreDispatch.Paths = append(deleted.PreDispatch.Paths, p)
		controls = append(controls, deleted)
		u2jReplay(t, root, controls)
	})

	t.Run("leaf_mutates_production_go_refused", func(t *testing.T) {
		controls := make([]u2jBranchControl, 0, 4)
		for _, origin := range []string{"leaf-tracked", "leaf-untracked", "predispatch", "mixed"} {
			c := base(origin, production)
			p := u2jPath("internal/leaf.go", "package leaf\n")
			c.PostLeaf.Paths = append(c.PostLeaf.Paths, p)
			if origin == "predispatch" || origin == "mixed" {
				c.PreDispatch.Paths = append(c.PreDispatch.Paths, p)
				// Do not rely on the obsolete baseline changed_files pass.
				c.ChangedFiles = []string{}
			} else if origin == "leaf-untracked" {
				c.UntrackedFiles = []string{p.Path}
				c.PostLeaf.Paths[len(c.PostLeaf.Paths)-1].Untracked = true
				c.PostLeaf.Paths[len(c.PostLeaf.Paths)-1].IndexMode = ""
				c.PostLeaf.Paths[len(c.PostLeaf.Paths)-1].IndexBlobOID = ""
			} else {
				c.ChangedFiles = append(c.ChangedFiles, p.Path)
			}
			if origin == "mixed" {
				c.PostLeaf.Paths = append(c.PostLeaf.Paths, u2jPath("docs/leaf.md", "leaf\n"))
			}
			controls = append(controls, c)
		}
		u2jReplay(t, root, controls)
	})

	t.Run("missing_or_mismatched_predispatch_snapshot_refused", func(t *testing.T) {
		controls := make([]u2jBranchControl, 0, 22)
		for _, defect := range []string{
			"missing-clean", "missing-dirty", "schema", "workspace_root", "task_id",
			"red_baseline_sha", "head_sha", "sha-short", "sha-nonhex", "index-mode",
			"index-oid", "escape-dotdot", "escape-slash", "escape-backslash", "duplicate-path",
			"post-missing", "post-task", "post-head", "post-sha", "drive-path", "unc-path", "leading-traversal",
		} {
			c := base(defect, unresolved)
			// Metadata refusals must be load-bearing even on an otherwise clean
			// baseline pass (the old evaluator silently accepts these records).
			c.ChangedFiles = []string{}
			switch defect {
			case "missing-clean", "missing-dirty":
				c.PreDispatch = nil
				if defect == "missing-dirty" {
					c.ChangedFiles = []string{caller[0].Path, caller[1].Path}
				} else {
					c.PostLeaf.Paths = []u2jSnapshotPath{}
				}
			case "schema":
				c.PreDispatch.Schema = "wrong/v0"
			case "workspace_root":
				c.PreDispatch.WorkspaceRoot = filepath.Join(root, "other")
			case "task_id":
				c.PreDispatch.TaskID = "211.016-T"
			case "red_baseline_sha":
				c.PreDispatch.Baseline = strings.Repeat("a", 40)
			case "head_sha":
				c.PreDispatch.Head = strings.Repeat("a", 40)
			case "sha-short":
				c.PreDispatch.Paths[0].WorktreeSHA256 = "abc"
			case "sha-nonhex":
				c.PreDispatch.Paths[0].WorktreeSHA256 = strings.Repeat("z", 64)
			case "index-mode":
				c.PreDispatch.Paths[0].IndexMode = "invalid"
			case "index-oid":
				c.PreDispatch.Paths[0].IndexBlobOID = "not-an-oid"
			case "escape-dotdot":
				c.PreDispatch.Paths[0].Path = "../outside"
			case "escape-slash":
				c.PreDispatch.Paths[0].Path = "/outside"
			case "escape-backslash":
				c.PreDispatch.Paths[0].Path = `\outside`
			case "duplicate-path":
				c.PreDispatch.Paths = append(c.PreDispatch.Paths, c.PreDispatch.Paths[0])
			case "post-missing":
				c.PostLeaf = nil
			case "post-task":
				c.PostLeaf.TaskID = "211.016-T"
			case "post-head":
				c.PostLeaf.Head = strings.Repeat("b", 40)
			case "post-sha":
				c.PostLeaf.Paths[0].WorktreeSHA256 = "bad"
			case "drive-path":
				c.PreDispatch.Paths[0].Path = "C:/outside"
			case "unc-path":
				c.PreDispatch.Paths[0].Path = `\\server\share`
			case "leading-traversal":
				c.PreDispatch.Paths[0].Path = "../../outside"
			}
			controls = append(controls, c)
		}
		u2jReplay(t, root, controls)
	})
}

type u2jSnapshotPath struct {
	Path           string `json:"path"`
	WorktreeExists bool   `json:"worktree_exists"`
	WorktreeSHA256 string `json:"worktree_sha256"`
	WorktreeMode   string `json:"worktree_mode"`
	IndexMode      string `json:"index_mode"`
	IndexBlobOID   string `json:"index_blob_oid"`
	UnstagedState  string `json:"unstaged_state"`
	StagedState    string `json:"staged_state"`
	Untracked      bool   `json:"untracked"`
}

type u2jSnapshotRecord struct {
	Schema        string            `json:"schema"`
	WorkspaceRoot string            `json:"workspace_root"`
	TaskID        string            `json:"task_id"`
	Baseline      string            `json:"red_baseline_sha"`
	Head          string            `json:"head_sha"`
	Paths         []u2jSnapshotPath `json:"paths"`
}

type u2jBranchControl struct {
	ID               string             `json:"id"`
	RedDeliverable   bool               `json:"red_deliverable"`
	HarnessCommand   string             `json:"harness_cmd"`
	SelectorCommand  string             `json:"red_selector_command"`
	Baseline         string             `json:"red_baseline_sha"`
	Head             string             `json:"head_sha"`
	WorkspaceRoot    string             `json:"workspace_root"`
	TaskID           string             `json:"task_id"`
	CompileOK        bool               `json:"compile_ok"`
	SelectorSignal   string             `json:"selector_signal"`
	ChangedFiles     []string           `json:"changed_files"`
	UntrackedFiles   []string           `json:"untracked_files"`
	EvidenceComplete bool               `json:"evidence_report_complete"`
	PreDispatch      *u2jSnapshotRecord `json:"predispatch_snapshot,omitempty"`
	PostLeaf         *u2jSnapshotRecord `json:"post_leaf_state,omitempty"`
	ExpectOutcome    string             `json:"expect_outcome"`
	ExpectGeneric    bool               `json:"expect_enters_generic_loop"`
}

func u2jBaseline() string { return "30908ecb8438c0b3e7fe8bd514fa3680e35d90e5" }

func u2jPath(path, contents string) u2jSnapshotPath {
	return u2jSnapshotPath{
		Path: path, WorktreeExists: true,
		WorktreeSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(contents))),
		WorktreeMode:   "100644", IndexMode: "100644",
		IndexBlobOID: strings.Repeat("a", 40), UnstagedState: "M",
	}
}

func u2jSnapshot(root string, paths []u2jSnapshotPath) u2jSnapshotRecord {
	return u2jSnapshotRecord{
		Schema:        "red-deliverable-predispatch-snapshot/v1",
		WorkspaceRoot: root, TaskID: "211.015-T", Baseline: u2jBaseline(), Head: u2jBaseline(),
		Paths: append([]u2jSnapshotPath{}, paths...),
	}
}

// RawMessage retains unrelated fixture sections losslessly without untyped APIs.
func u2jFixture(t *testing.T, root string) map[string]json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "tests", "simulation", "wave-scheduler-contract.json"))
	require.NoError(t, err)
	var fixture map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fixture))
	return fixture
}

func u2jReplay(t *testing.T, root string, controls []u2jBranchControl) {
	t.Helper()
	fixture := u2jFixture(t, root)
	var existing []json.RawMessage
	require.NoError(t, json.Unmarshal(fixture["red_deliverable_branch_controls"], &existing))
	assert.GreaterOrEqual(t, len(existing), 18, "all original branch controls must remain")
	for _, c := range controls {
		raw, err := json.Marshal(c)
		require.NoError(t, err)
		existing = append(existing, raw)
	}
	raw, err := json.Marshal(existing)
	require.NoError(t, err)
	fixture["red_deliverable_branch_controls"] = raw
	var scenarios []json.RawMessage
	require.NoError(t, json.Unmarshal(fixture["scenarios"], &scenarios))
	assert.GreaterOrEqual(t, len(scenarios), 26, "all original scenarios must remain")
	u2jRunFixture(t, root, fixture, controls)
}

func u2jRunFixture(t *testing.T, root string, fixture map[string]json.RawMessage, controls []u2jBranchControl) {
	t.Helper()
	raw, err := json.MarshalIndent(fixture, "", "  ")
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "u2j.json")
	require.NoError(t, os.WriteFile(path, append(raw, '\n'), 0o600))
	output, code := u2jPowerShell(t, root, "-NoProfile", "-File",
		filepath.Join(root, "scripts", "wave-scheduler-sim.ps1"), "-Fixture", path)
	for _, c := range controls {
		assert.False(t, strings.Contains(output, "FAIL red_branch/"+c.ID+" :: branch outcome"),
			"red_branch/%s must produce %s", c.ID, c.ExpectOutcome)
		assert.False(t, strings.Contains(output, "FAIL red_branch/"+c.ID+" :: enters generic loop"),
			"red deliverable must never enter the generic loop")
	}
	assert.Equal(t, 0, code, "snapshot contract assertions must pass (raw output logged above)")
	assert.False(t, strings.Contains(output, "WAVE_SIM_FAIL:"), "evaluator must honor the declared snapshot expectations")
	assert.True(t, strings.Contains(output, "WAVE_SIM_OK:"), "simulation must finish and exercise its assertions")
}

func u2jPowerShell(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	pwsh, err := exec.LookPath("pwsh")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, pwsh, args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	raw, err := cmd.CombinedOutput()
	require.NoError(t, ctx.Err(), "PowerShell timeout:\n%s", raw)
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		require.True(t, errors.As(err, &exitErr), "PowerShell launch failed: %v", err)
		code = exitErr.ExitCode()
	}
	output := strings.ReplaceAll(string(raw), "\r\n", "\n")
	t.Logf("pwsh %s\nnative_exit=%d\n%s", strings.Join(args, " "), code, output)
	return output, code
}

func u2jPausedResume(t *testing.T, root string) {
	t.Helper()
	fixture := u2jFixture(t, root)
	// Two serial prefix members put this bounded transition at Waves 3/4.
	// Nine already-done members must not be re-scheduled. The started flag is
	// carried across the amendment, not removed to make admission succeed.
	fixture["source"] = json.RawMessage(`{"shipment":"212-S","paused_red_amendment":{
		"stage_reviewed":true,"shipment":"212-S","task":"211.015-T","prerequisite":"211.016-T",
		"start_count":1,"red_baseline_sha":"30908ecb8438c0b3e7fe8bd514fa3680e35d90e5",
		"source_reload_required":true
	}}`)
	fixture["manifest"] = json.RawMessage(`{
		"members":[
			{"id":"211.001-T","unit":"U1","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"211.002-T","unit":"U2","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"211.008-T","unit":"U1b","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"211.009-T","unit":"U1c","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"211.010-T","unit":"U1d","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"211.011-T","unit":"U1e","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"211.012-T","unit":"U1f","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"211.013-T","unit":"U1g","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"211.014-T","unit":"U1h","status":"done","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"prefix.1-T","unit":"prefix1","status":"queued","harness_exempt":false,"red_deliverable":false,"deps":[]},
			{"id":"prefix.2-T","unit":"prefix2","status":"queued","harness_exempt":false,"red_deliverable":false,"deps":["prefix.1-T"]},
			{"id":"211.016-T","unit":"U2j","status":"queued","harness_exempt":false,"red_deliverable":false,"deps":["prefix.2-T","211.001-T","211.002-T","211.013-T","211.014-T"]},
			{"id":"211.015-T","unit":"U2i","status":"active","harness_exempt":false,"red_deliverable":true,"deps":["211.016-T"]},
			{"id":"211.003-T","unit":"U3","status":"queued","harness_exempt":false,"red_deliverable":false,"deps":["211.015-T"]},
			{"id":"211.004-T","unit":"U4","status":"queued","harness_exempt":false,"red_deliverable":false,"deps":["211.003-T"]},
			{"id":"211.005-T","unit":"U5","status":"queued","harness_exempt":false,"red_deliverable":false,"deps":["211.004-T"]},
			{"id":"211.006-T","unit":"U6","status":"queued","harness_exempt":false,"red_deliverable":false,"deps":["211.005-T"]},
			{"id":"211.007-T","unit":"U7","status":"queued","harness_exempt":false,"red_deliverable":false,"deps":["211.006-T"]}
		]}`)
	fixture["red_deliverables"] = json.RawMessage(`[{
		"task":"211.015-T","unit":"U2i","red_deliverable":true,
		"red_deliverable_reason":"Preserved started RED deliverable resumes after U2j",
		"red_selector_command":"go test -count=1 -run '^TestU2i_' ./internal/cli",
		"red_baseline_sha":"30908ecb8438c0b3e7fe8bd514fa3680e35d90e5",
		"green_maker_tasks":["211.003-T","211.004-T","211.005-T","211.006-T","211.007-T"],
		"green_maker_closes_wave":9
	}]`)
	fixture["scenarios"] = json.RawMessage(`[{
		"id":"u2j-paused-red-deliverable-amendment-resume",
		"mutations":{
			"claim_overrides":{"claim_all":"212-S","started":["211.015-T"]},
			"set_status_at_wave":[
				{"wave":1,"id":"prefix.1-T","status":"queued"},
				{"wave":1,"id":"prefix.2-T","status":"queued"},
				{"wave":1,"id":"211.016-T","status":"queued"},
				{"wave":1,"id":"211.003-T","status":"queued"},
				{"wave":1,"id":"211.004-T","status":"queued"},
				{"wave":1,"id":"211.005-T","status":"queued"},
				{"wave":1,"id":"211.006-T","status":"queued"},
				{"wave":1,"id":"211.007-T","status":"queued"}
			]
		},
		"expect":{
			"outcome":"COMPLETE","waves":9,"scheduled":9,"stalls":0,
			"wave_sizes":[1,1,1,1,1,1,1,1,1],"compile_order_violations":0,
			"open_red_after_wave_4":["211.015-T"],
			"open_red_reconfirmed_waves":[4,5,6,7,8],
			"open_red_closed_at_wave":{"211.015-T":9},
			"deferred_waves":[4,5,6,7,8],"full_suite_waves":[1,2,3,9],
			"final_open_red":[],"completion_claimed":true
		}
	}]`)
	u2jRunFixture(t, root, fixture, nil)
	// An amendment is explicit evidence, not a blanket started-active waiver.
	validSource := append(json.RawMessage{}, fixture["source"]...)
	validScenario := string(fixture["scenarios"])
	for _, defect := range []string{"missing-amendment", "not-reviewed", "wrong-prerequisite", "restart", "no-reload", "wrong-baseline"} {
		t.Run(defect, func(t *testing.T) {
			source := string(validSource)
			switch defect {
			case "missing-amendment":
				source = `{"shipment":"212-S"}`
			case "not-reviewed":
				source = strings.Replace(source, `"stage_reviewed":true`, `"stage_reviewed":false`, 1)
			case "wrong-prerequisite":
				source = strings.Replace(source, `"prerequisite":"211.016-T"`, `"prerequisite":"outside-T"`, 1)
			case "restart":
				source = strings.Replace(source, `"start_count":1`, `"start_count":2`, 1)
			case "no-reload":
				source = strings.Replace(source, `"source_reload_required":true`, `"source_reload_required":false`, 1)
			case "wrong-baseline":
				source = strings.Replace(source, u2jBaseline(), strings.Repeat("a", 40), 1)
			}
			fixture["source"] = json.RawMessage(source)
			// Keep admission unchanged; only the negative expectation differs.
			var scenarios []struct {
				ID        string          `json:"id"`
				Mutations json.RawMessage `json:"mutations"`
				Expect    json.RawMessage `json:"expect"`
			}
			require.NoError(t, json.Unmarshal([]byte(validScenario), &scenarios))
			scenarios[0].ID += "-" + defect
			scenarios[0].Expect = json.RawMessage(`{"outcome":"WAVE_NO_PROGRESS","halt_wave":1,"halt_detail":"active residual","active_ids":["211.015-T"]}`)
			raw, err := json.Marshal(scenarios)
			require.NoError(t, err)
			fixture["scenarios"] = raw
			u2jRunFixture(t, root, fixture, nil)
		})
	}
}

func u2jGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-pager", "-c", "core.autocrlf=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "fixture git %s: %s", strings.Join(args, " "), out)
	return strings.TrimSpace(string(out))
}

func u2jRealGit(t *testing.T, root string) {
	t.Helper()
	// Load only top-level function ASTs, not the script body (which exits).
	// Missing helpers produce a named assertion, not a missing Go symbol or
	// a PowerShell command-not-found error. Once declared, these same tests
	// execute the real implementation against independent Git/byte oracles.
	const load = `
$ErrorActionPreference = 'Stop'
$tokens = $null; $parseErrors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($env:U2J_SCRIPT, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count -ne 0) { throw 'wave simulator must parse' }
foreach ($s in $ast.EndBlock.Statements) {
    if ($s -is [System.Management.Automation.Language.FunctionDefinitionAst]) {
        Invoke-Expression $s.Extent.Text
    }
}
if (-not (Get-Command Get-RedDeliverablePreDispatchSnapshot -ErrorAction SilentlyContinue) -or
    -not (Get-Command Compare-RedDeliverablePreDispatchSnapshot -ErrorAction SilentlyContinue)) {
    Write-Output 'U2J_SNAPSHOT_HELPERS_NOT_DECLARED'
    exit 0
}
`
	for _, mutation := range []string{"zero", "task-byte", "hook-byte", "task-stage", "index-mode", "hook-delete", "untracked-delete", "new-test", "new-config", "new-docs", "production-leaf", "production-predispatch", "clean-tracked-byte", "ignored-scratch", "head-change", "raw-binary-byte", "reparse-escape"} {
		t.Run(mutation, func(t *testing.T) {
			dir := t.TempDir()
			taskPath := filepath.Join(dir, ".backlogit", "queue", "211.015-T.md")
			hookPath := filepath.Join(dir, ".backlogit", "hooks_queue.jsonl")
			docsPath := filepath.Join(dir, "docs", "caller.md")
			write := func(path, contents string) {
				t.Helper()
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
				require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
			}
			u2jGit(t, dir, "init", "--quiet")
			write(taskPath, "queued\n")
			write(hookPath, "claim\n")
			write(filepath.Join(dir, "docs", "clean.md"), "clean\n")
			write(filepath.Join(dir, ".gitignore"), "scratch/\n")
			u2jGit(t, dir, "add", ".")
			u2jGit(t, dir, "-c", "user.name=U2j Fixture", "-c", "user.email=u2j@example.invalid",
				"-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "fixture baseline")
			baseline := u2jGit(t, dir, "rev-parse", "HEAD")
			write(taskPath, "active\n")
			write(hookPath, "start\n")
			write(docsPath, "caller\n")
			if mutation == "production-predispatch" {
				write(filepath.Join(dir, "leaf.go"), "package leaf\n")
			}
			indexOID := u2jGit(t, dir, "rev-parse", ":.backlogit/queue/211.015-T.md")
			modeLine := u2jGit(t, dir, "ls-files", "--stage", "--", ".backlogit/queue/211.015-T.md")
			require.Contains(t, modeLine, "100644 "+indexOID)
			// Keep all fixture paths and tool I/O under the owned TEMP root.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			run := func(body string) string {
				t.Helper()
				pwsh, err := exec.LookPath("pwsh")
				require.NoError(t, err)
				cmd := exec.CommandContext(ctx, pwsh, "-NoProfile", "-Command", load+body)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(),
					"NO_COLOR=1", "U2J_SCRIPT="+filepath.Join(root, "scripts", "wave-scheduler-sim.ps1"),
					"U2J_ROOT="+dir, "U2J_BASELINE="+baseline)
				raw, err := cmd.CombinedOutput()
				require.NoError(t, err, "snapshot helper invocation:\n%s", raw)
				return strings.TrimSpace(string(raw))
			}
			const capture = `
$snapshot = Get-RedDeliverablePreDispatchSnapshot -WorkspaceRoot $env:U2J_ROOT -TaskID '211.015-T' -RedBaselineSHA $env:U2J_BASELINE
$snapshot | ConvertTo-Json -Depth 30 -Compress
`
			rawPre := run(capture)
			if strings.Contains(rawPre, "U2J_SNAPSHOT_HELPERS_NOT_DECLARED") {
				assert.Fail(t, "Get-RedDeliverablePreDispatchSnapshot / Compare-RedDeliverablePreDispatchSnapshot are not declared in scripts/wave-scheduler-sim.ps1")
				return
			}
			var pre u2jSnapshotRecord
			require.NoError(t, json.Unmarshal([]byte(rawPre), &pre), "capture must return the snapshot schema")
			assert.Equal(t, "red-deliverable-predispatch-snapshot/v1", pre.Schema)
			assert.Equal(t, dir, pre.WorkspaceRoot)
			assert.Equal(t, "211.015-T", pre.TaskID)
			assert.Equal(t, baseline, pre.Baseline)
			assert.Equal(t, baseline, pre.Head)
			find := func(path string) u2jSnapshotPath {
				t.Helper()
				for _, p := range pre.Paths {
					if p.Path == path {
						return p
					}
				}
				require.FailNow(t, "caller dirty path absent from snapshot", path)
				return u2jSnapshotPath{}
			}
			task := find(".backlogit/queue/211.015-T.md")
			assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte("active\n"))), task.WorktreeSHA256)
			assert.Equal(t, indexOID, task.IndexBlobOID)
			assert.Equal(t, "100644", task.IndexMode)
			assert.Equal(t, "100644", task.WorktreeMode)
			assert.Equal(t, "M", task.UnstagedState)
			assert.Empty(t, task.StagedState)
			assert.True(t, task.WorktreeExists)
			assert.False(t, task.Untracked)
			assert.Equal(t, fmt.Sprintf("%x", sha256.Sum256([]byte("start\n"))),
				find(".backlogit/hooks_queue.jsonl").WorktreeSHA256)
			assert.True(t, find("docs/caller.md").Untracked)

			want := "RED_DELIVERABLE_DELTA_OUT_OF_SURFACE"
			switch mutation {
			case "zero":
				want = "RED_DELIVERABLE_ACCEPTED"
			case "task-byte":
				write(taskPath, "Active\n")
			case "hook-byte":
				write(hookPath, "Start\n")
			case "task-stage":
				u2jGit(t, dir, "add", "--", ".backlogit/queue/211.015-T.md")
			case "index-mode":
				u2jGit(t, dir, "update-index", "--chmod=+x", "--", ".backlogit/queue/211.015-T.md")
				assert.Contains(t, u2jGit(t, dir, "ls-files", "--stage"), "100755")
			case "hook-delete":
				require.NoError(t, os.Remove(hookPath))
			case "untracked-delete":
				require.NoError(t, os.Remove(docsPath))
			case "new-test":
				write(filepath.Join(dir, "leaf_test.go"), "package leaf\n")
			case "new-config":
				write(filepath.Join(dir, ".backlogit", "config.yaml"), "leaf: true\n")
			case "new-docs":
				write(filepath.Join(dir, "docs", "leaf.md"), "leaf\n")
			case "production-leaf":
				write(filepath.Join(dir, "leaf.go"), "package leaf\n")
				want = "RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED"
			case "production-predispatch":
				want = "RED_DELIVERABLE_PRODUCTION_DELTA_REFUSED"
			case "clean-tracked-byte":
				write(filepath.Join(dir, "docs", "clean.md"), "Clean\n")
			case "ignored-scratch":
				write(filepath.Join(dir, "scratch", "leaf.md"), "ignored\n")
				want = "RED_DELIVERABLE_ACCEPTED"
			case "head-change":
				u2jGit(t, dir, "-c", "user.name=U2j Fixture", "-c", "user.email=u2j@example.invalid",
					"-c", "commit.gpgsign=false", "commit", "--quiet", "--allow-empty", "-m", "leaf HEAD mutation")
				want = "WAVE_RED_MAPPING_UNRESOLVED"
			case "raw-binary-byte":
				write(taskPath, "active\x00\n")
			case "reparse-escape":
				if os.PathSeparator == '\\' {
					out := run(`
$target = Join-Path $env:U2J_ROOT '.git/owned-target'
$null = New-Item -ItemType Directory -Path $target
[IO.File]::WriteAllText((Join-Path $target 'leaf.md'), 'leaf')
$null = New-Item -ItemType Junction -Path (Join-Path $env:U2J_ROOT 'escape') -Target $target
try {
    $null = Get-RedDeliverablePreDispatchSnapshot -WorkspaceRoot $env:U2J_ROOT -TaskID '211.015-T' -RedBaselineSHA $env:U2J_BASELINE
    throw 'capture accepted a junction escape'
} catch {
    if ($_.Exception.Message -notlike 'WAVE_RED_MAPPING_UNRESOLVED:*') { throw }
    Write-Output 'WAVE_RED_MAPPING_UNRESOLVED'
}
`)
					assert.Equal(t, "WAVE_RED_MAPPING_UNRESOLVED", out)
				} else {
					require.NoError(t, os.Symlink(filepath.Join(dir, ".git"), filepath.Join(dir, "escape")))
					out := run(`
try {
    $null = Get-RedDeliverablePreDispatchSnapshot -WorkspaceRoot $env:U2J_ROOT -TaskID '211.015-T' -RedBaselineSHA $env:U2J_BASELINE
    throw 'capture accepted a symlink escape'
} catch {
    if ($_.Exception.Message -notlike 'WAVE_RED_MAPPING_UNRESOLVED:*') { throw }
    Write-Output 'WAVE_RED_MAPPING_UNRESOLVED'
}
`)
					assert.Equal(t, "WAVE_RED_MAPPING_UNRESOLVED", out)
				}
				return
			}
			rawPost := run(capture)
			preFile, postFile := filepath.Join(dir, ".git", "u2j-pre.json"), filepath.Join(dir, ".git", "u2j-post.json")
			// Snapshot evidence lives inside .git, not in the measured surface.
			write(preFile, rawPre)
			write(postFile, rawPost)
			result := run(`
$pre = Get-Content (Join-Path $env:U2J_ROOT '.git/u2j-pre.json') -Raw | ConvertFrom-Json
$post = Get-Content (Join-Path $env:U2J_ROOT '.git/u2j-post.json') -Raw | ConvertFrom-Json
Compare-RedDeliverablePreDispatchSnapshot -PreDispatchSnapshot $pre -PostLeafSnapshot $post |
    ConvertTo-Json -Depth 30 -Compress
`)
			var outcome struct {
				Outcome string `json:"Outcome"`
			}
			require.NoError(t, json.Unmarshal([]byte(result), &outcome))
			assert.Equal(t, want, outcome.Outcome, "real Git %s must compare independent pre/post captures", mutation)
			expectedHead := baseline
			if mutation == "head-change" {
				expectedHead = pre.Head
				assert.NotEqual(t, expectedHead, u2jGit(t, dir, "rev-parse", "HEAD"), "oracle HEAD mutation executed")
			} else {
				assert.Equal(t, expectedHead, u2jGit(t, dir, "rev-parse", "HEAD"), "fixture comparison must not commit")
			}
		})
	}
}

func TestUCS3_WaveSimClaimStart(t *testing.T) {
	type scenario struct {
		ID     string `json:"id"`
		Expect struct {
			Outcome                  string   `json:"outcome"`
			Waves                    int      `json:"waves"`
			Scheduled                int      `json:"scheduled"`
			HaltDetail               string   `json:"halt_detail"`
			HaltWave                 *int     `json:"halt_wave"`
			ActiveIDs                []string `json:"active_ids"`
			ClaimAssignedAtAdmission *int     `json:"claim_assigned_at_admission"`
		} `json:"expect"`
	}
	type simulationFixture struct {
		Scenarios []scenario `json:"scenarios"`
	}

	repoRoot := testRepoRoot(t)
	var fixture simulationFixture

	t.Run("Preserved", func(t *testing.T) {
		fixturePath := filepath.Join(repoRoot, filepath.FromSlash("tests/simulation/wave-scheduler-contract.json"))
		contents, err := os.ReadFile(fixturePath)
		require.NoError(t, err, "read wave scheduler simulation fixture")
		require.NoError(t, json.Unmarshal(contents, &fixture), "decode wave scheduler simulation fixture")

		present := make(map[string]struct{}, len(fixture.Scenarios))
		for _, item := range fixture.Scenarios {
			present[item.ID] = struct{}{}
		}
		existingScenarioIDs := []string{
			"baseline",
			"persistent_red_mapping",
			"blocked_injection",
			"blocked_mid_run",
			"active_residual",
			"unsupported_status_review",
			"unsupported_status_abandoned",
			"unsupported_status_off_catalog",
			"status_catalog_unavailable",
			"status_catalog_disagrees",
			"cycle_injection",
			"sibling_red_wave4",
			"non_frozen_m_control",
			"missing_green_maker",
			"ambiguous_green_maker",
			"missing_red_selector",
			"wrong_green_maker_close_wave",
			"green_maker_descoped",
			"open_red_early_green_carried_in",
			"open_red_closed_entry_not_reconfirmed",
			"green_maker_lands_but_selector_stays_red",
		}
		for _, id := range existingScenarioIDs {
			_, ok := present[id]
			require.True(t, ok, "existing scenario %q must remain present", id)
		}
	})

	t.Run("Fixture", func(t *testing.T) {
		findByID := func(t *testing.T, id string) *scenario {
			t.Helper()

			var found *scenario
			for i := range fixture.Scenarios {
				if fixture.Scenarios[i].ID == id {
					found = &fixture.Scenarios[i]
					break
				}
			}
			require.NotNil(t, found, "scenario %q must exist", id)
			return found
		}

		baseline := findByID(t, "baseline")
		claimAssigned := findByID(t, "claim_assigned_all")
		residuals := findByID(t, "claim_state_residuals")
		indeterminate := findByID(t, "claim_state_indeterminate")

		assert.Equal(t, "COMPLETE", claimAssigned.Expect.Outcome)
		assert.Equal(t, baseline.Expect.Waves, claimAssigned.Expect.Waves,
			"claim-assigned scenario must preserve baseline waves")
		assert.Equal(t, baseline.Expect.Scheduled, claimAssigned.Expect.Scheduled,
			"claim-assigned scenario must preserve baseline scheduled count")
		if assert.NotNil(t, claimAssigned.Expect.ClaimAssignedAtAdmission,
			"claim_assigned_all must expect claim_assigned_at_admission") {
			assert.Equal(t, claimAssigned.Expect.Scheduled, *claimAssigned.Expect.ClaimAssignedAtAdmission,
				"claim_assigned_at_admission must equal scheduled")
		}

		assert.Equal(t, "WAVE_NO_PROGRESS", residuals.Expect.Outcome)
		assert.Equal(t, "active residual", residuals.Expect.HaltDetail)
		assert.Len(t, residuals.Expect.ActiveIDs, 3,
			"claim_state_residuals must list exactly the residual active IDs")

		assert.Equal(t, "WAVE_CLAIM_STATE_INDETERMINATE", indeterminate.Expect.Outcome)
		if assert.NotNil(t, indeterminate.Expect.HaltWave,
			"claim_state_indeterminate must expect a halt wave") {
			assert.Equal(t, 1, *indeterminate.Expect.HaltWave)
		}
	})

	t.Run("Replay", func(t *testing.T) {
		pwshPath, err := exec.LookPath("pwsh")
		if err != nil {
			t.Fatalf("locate pwsh for wave scheduler simulation: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, pwshPath, "-NoProfile", "-File", "scripts/wave-scheduler-sim.ps1")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "NO_COLOR=1")
		rawOutput, runErr := cmd.CombinedOutput()

		ansiEscapePattern := regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\)|[@-_])`)
		output := ansiEscapePattern.ReplaceAllString(string(rawOutput), "")
		output = strings.ReplaceAll(output, "\r\n", "\n")

		if ctx.Err() != nil {
			assert.Fail(t, "wave scheduler simulation timed out: %v\noutput:\n%s", ctx.Err(), output)
			return
		}

		exitCode := 0
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				assert.Fail(t, "run wave scheduler simulation: %v\noutput:\n%s", runErr, output)
				return
			}
			exitCode = exitErr.ExitCode()
		}
		assert.Equal(t, 0, exitCode, "wave scheduler simulation must exit successfully\noutput:\n%s", output)

		assert.NotRegexp(t, regexp.MustCompile(`(?m)^.*WAVE_SIM_FAIL.*$`), output,
			"simulation output must not contain a WAVE_SIM_FAIL line")
		trimmedOutput := strings.TrimSpace(output)
		assert.NotEmpty(t, trimmedOutput, "simulation output must not be empty")
		if trimmedOutput != "" {
			outputLines := strings.Split(trimmedOutput, "\n")
			assert.Regexp(t, regexp.MustCompile(`^WAVE_SIM_OK:`), strings.TrimSpace(outputLines[len(outputLines)-1]),
				"the final simulation output line must begin with WAVE_SIM_OK:")
		}

		outcomeLinePattern := regexp.MustCompile(`(?m)^[ \t]*([^ \t\r\n]+)[ \t]+outcome=[^\r\n]*$`)
		outcomeLineMatches := outcomeLinePattern.FindAllStringSubmatch(output, -1)
		outcomeIDs := make([]string, 0, len(outcomeLineMatches))
		for _, match := range outcomeLineMatches {
			outcomeIDs = append(outcomeIDs, match[1])
		}
		assert.GreaterOrEqual(t, len(fixture.Scenarios), 24,
			"fixture must contain at least the 21 preserved and 3 claim-start scenarios")
		assert.GreaterOrEqual(t, len(outcomeLineMatches), 24,
			"simulation must report at least the 21 preserved and 3 claim-start scenarios")
		assert.Len(t, outcomeLineMatches, len(fixture.Scenarios),
			"simulation outcome-line count must match the fixture scenario count")
		fixtureIDs := make([]string, 0, len(fixture.Scenarios))
		for _, item := range fixture.Scenarios {
			fixtureIDs = append(fixtureIDs, item.ID)
		}
		assert.ElementsMatch(t, fixtureIDs, outcomeIDs,
			"simulation outcome-line IDs must match all fixture scenario IDs, including multiplicity")

		expectedOutcomes := []struct {
			id      string
			outcome string
		}{
			{id: "claim_assigned_all", outcome: "COMPLETE"},
			{id: "claim_state_residuals", outcome: "WAVE_NO_PROGRESS"},
			{id: "claim_state_indeterminate", outcome: "WAVE_CLAIM_STATE_INDETERMINATE"},
		}
		for _, expected := range expectedOutcomes {
			linePattern := regexp.MustCompile(
				`(?m)^[ \t]*` + regexp.QuoteMeta(expected.id) + `[ \t]+outcome=` +
					regexp.QuoteMeta(expected.outcome) + `([ \t]|$)`,
			)
			assert.Len(t, linePattern.FindAllString(output, -1), 1,
				"simulation must emit exactly one %s outcome=%s line", expected.id, expected.outcome)
		}
	})
}
