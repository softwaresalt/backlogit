package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/evidence"
	"github.com/softwaresalt/backlogit/internal/evidencedag"
	"github.com/softwaresalt/backlogit/internal/policyengine"
	"github.com/softwaresalt/backlogit/internal/redhonesty"
)

const policyTestRegistry = `schema_version: 1
mode: enforce
nodes:
  - {id: harness-scaffold, enabled: true, overridable: true}
  - {id: red-phase-verify, enabled: true, overridable: true}
  - {id: green-phase-verify, enabled: true, overridable: false}
  - {id: review-complete, enabled: true, overridable: false}
  - {id: ci-pass, enabled: true, overridable: false}
  - {id: merge-ready, enabled: true, overridable: false}
edges:
  - {node: red-phase-verify, depends_on: harness-scaffold}
  - {node: green-phase-verify, depends_on: red-phase-verify}
  - {node: review-complete, depends_on: green-phase-verify}
  - {node: ci-pass, depends_on: review-complete}
  - {node: merge-ready, depends_on: ci-pass}
`

type policyFakeDeriv struct{}

func (policyFakeDeriv) ReviewEvidence(context.Context, int) evidence.ReviewResult {
	return evidence.ReviewResult{Status: evidence.StatusVerified, PR: 7, HeadSHA: policySHA(5), ReviewedCurrent: true}
}

func (policyFakeDeriv) CIEvidence(context.Context, int) evidence.CIResult {
	return evidence.CIResult{Status: evidence.StatusVerified, PR: 7, HeadSHA: policySHA(5), AllPassed: true}
}

func policySHA(n int) string { return fmt.Sprintf("%040x", n) }

var policyT0 = time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)

type policyFixture struct {
	t       *testing.T
	root    string
	mem     *policyengine.MemGit
	records []policyengine.Record
	task    string
}

func newPolicyFixture(t *testing.T, registry string) *policyFixture {
	t.Helper()
	f := &policyFixture{t: t, root: t.TempDir(), mem: policyengine.NewMemGit(), task: "163.003-T"}
	for i := 1; i <= 5; i++ {
		if i == 1 {
			f.mem.AddCommit(policySHA(i), policyT0.Add(time.Duration(i)*time.Minute))
		} else {
			f.mem.AddCommit(policySHA(i), policyT0.Add(time.Duration(i)*time.Minute), policySHA(i-1))
		}
	}
	f.mem.SetRef("refs/remotes/origin/feat/x", policySHA(5))
	rec := func(node policyengine.NodeType, c int, art json.RawMessage) policyengine.Record {
		r, err := policyengine.NewRecord(f.task, node, policySHA(c), policyT0.Add(time.Duration(c)*time.Minute+30*time.Second), policyengine.OutcomePass, art)
		require.NoError(t, err)
		return r
	}
	a, err := redhonesty.BuildArtifact(f.task, policySHA(1), policySHA(3), []redhonesty.Verdict{{
		Test:     redhonesty.NewTest{Package: ".", File: "x_test.go", Name: "TestX"},
		Baseline: redhonesty.ResultFail, Head: redhonesty.ResultPass,
	}})
	require.NoError(t, err)
	raw, err := json.Marshal(a)
	require.NoError(t, err)
	f.records = []policyengine.Record{
		rec(policyengine.NodeHarnessScaffold, 2, nil),
		rec(policyengine.NodeRedPhaseVerify, 3, raw),
		rec(policyengine.NodeGreenPhaseVerify, 4, nil),
		rec(policyengine.NodeMergeReady, 5, nil),
	}
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "registry.yaml"), []byte(registry), 0o600))
	return f
}

func (f *policyFixture) writeBundle(name string, mutate func(*policyengine.Bundle)) {
	b := policyengine.Bundle{SchemaVersion: policyengine.EvidenceSchemaVersion, TaskID: f.task, HeadCommit: policySHA(5), PR: 7, Records: f.records}
	if mutate != nil {
		mutate(&b)
	}
	raw, err := json.Marshal(b)
	require.NoError(f.t, err)
	require.NoError(f.t, os.WriteFile(filepath.Join(f.root, name), raw, 0o600))
}

func installPolicySeams(t *testing.T, mem *policyengine.MemGit, interactive bool) {
	t.Helper()
	pg, pd, pi, pn := policyGit, policyDerivation, policyInteractive, policyNow
	policyGit = func(string) policyengine.Git { return mem }
	policyDerivation = func(string) policyengine.Derivation { return policyFakeDeriv{} }
	policyInteractive = func(io.Writer) bool { return interactive }
	policyNow = func() time.Time { return policyT0.Add(time.Hour) }
	t.Cleanup(func() { policyGit, policyDerivation, policyInteractive, policyNow = pg, pd, pi, pn })
}

func runPolicy(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(append([]string{"--cwd", root, "--no-update-check"}, args...))
	err := cmd.Execute()
	return out.String(), err
}

func TestU163_003_PolicyCheckPassesASatisfiedTask(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)

	out, err := runPolicy(t, f.root, "policy", "check", "--registry", "registry.yaml", "--evidence", "bundle.json",
		"--protected-ref", "refs/remotes/origin/feat/x", "--repo", "o/r", "--format", "json")
	require.NoError(t, err, out)
	var d policyengine.Decision
	require.NoError(t, json.Unmarshal([]byte(out), &d), out)
	assert.Equal(t, policyengine.DispositionPass, d.Disposition)
	assert.True(t, d.Allowed)
	assert.Equal(t, policyengine.SurfaceCLI, d.Surface)
}

func TestU163_003_PolicyCheckBlocksAndExitsNonZero(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.records = f.records[1:] // harness evidence absent
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)

	out, err := runPolicy(t, f.root, "policy", "check", "--registry", "registry.yaml", "--evidence", "bundle.json",
		"--protected-ref", "refs/remotes/origin/feat/x", "--repo", "o/r", "--format", "json")
	require.Error(t, err)
	var blockedErr *policyBlockedError
	require.True(t, errors.As(err, &blockedErr))
	var d policyengine.Decision
	require.NoError(t, json.Unmarshal([]byte(out), &d), out)
	assert.False(t, d.Allowed)
	found := false
	for _, v := range d.Errors {
		if v.Node == "harness-scaffold" && v.Code == policyengine.CodeEvidenceAbsent {
			found = true
		}
	}
	assert.True(t, found, "%+v", d.Errors)
}

func TestU163_003_PolicyCheckMatchesTheSharedEngineOnEveryScenario(t *testing.T) {
	scenarios := map[string]func(*policyFixture){
		"satisfied":      func(*policyFixture) {},
		"absent harness": func(f *policyFixture) { f.records = f.records[1:] },
		"absent merge":   func(f *policyFixture) { f.records = f.records[:3] },
	}
	for name, mut := range scenarios {
		t.Run(name, func(t *testing.T) {
			f := newPolicyFixture(t, policyTestRegistry)
			mut(f)
			f.writeBundle("bundle.json", nil)
			installPolicySeams(t, f.mem, false)

			reg, err := policyengine.LoadRegistry([]byte(policyTestRegistry))
			require.NoError(t, err)
			eng, err := policyengine.NewEngine(reg, policyengine.Options{
				Git: f.mem, Derivation: policyFakeDeriv{}, ProtectedRefs: []string{"refs/remotes/origin/feat/x"},
				Now: func() time.Time { return policyT0.Add(time.Hour) },
			})
			require.NoError(t, err)
			raw, err := os.ReadFile(filepath.Join(f.root, "bundle.json"))
			require.NoError(t, err)

			out, _ := runPolicy(t, f.root, "policy", "check", "--registry", "registry.yaml", "--evidence", "bundle.json",
				"--protected-ref", "refs/remotes/origin/feat/x", "--repo", "o/r", "--format", "json")
			var cliDecision policyengine.Decision
			require.NoError(t, json.Unmarshal([]byte(out), &cliDecision), out)
			for _, s := range []policyengine.Surface{policyengine.SurfaceMCP, policyengine.SurfaceAgent} {
				d, _ := eng.CheckBundle(context.Background(), s, raw)
				d.Surface = cliDecision.Surface
				want, _ := json.Marshal(d)
				got, _ := json.Marshal(cliDecision)
				assert.JSONEq(t, string(want), string(got), "CLI and %s must return the same decision", s)
			}
		})
	}
}

func TestU163_003_PolicyCheckReportOnlyAllowsButReports(t *testing.T) {
	f := newPolicyFixture(t, strings.Replace(policyTestRegistry, "mode: enforce", "mode: report-only", 1))
	f.records = f.records[1:]
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)
	args := []string{"policy", "check", "--registry", "registry.yaml", "--evidence", "bundle.json",
		"--protected-ref", "refs/remotes/origin/feat/x", "--repo", "o/r", "--format", "json"}

	out, err := runPolicy(t, f.root, args...)
	require.NoError(t, err, out)
	var d policyengine.Decision
	require.NoError(t, json.Unmarshal([]byte(out), &d))
	assert.Equal(t, policyengine.DispositionReportOnly, d.Disposition)
	assert.NotEmpty(t, d.Errors)

	_, err = runPolicy(t, f.root, append(args, "--mode", "enforce")...)
	require.Error(t, err, "--mode may tighten report-only to enforce")
}

func TestU163_003_PolicyCheckModeFlagNeverRelaxes(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)
	_, err := runPolicy(t, f.root, "policy", "check", "--registry", "registry.yaml", "--evidence", "bundle.json",
		"--protected-ref", "refs/remotes/origin/feat/x", "--mode", "report-only", "--format", "json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "relax")
}

func TestU163_003_PolicyCheckRejectsUnsafePathsAndMissingFlags(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.writeBundle("bundle.json", nil)
	installPolicySeams(t, f.mem, false)
	outside := filepath.Join(t.TempDir(), "bundle.json")
	require.NoError(t, os.WriteFile(outside, []byte("{}"), 0o600))
	base := []string{"policy", "check", "--protected-ref", "refs/remotes/origin/feat/x", "--format", "json"}
	cases := map[string][]string{
		"no evidence":        append(append([]string{}, base...), "--registry", "registry.yaml"),
		"traversal":          append(append([]string{}, base...), "--registry", "registry.yaml", "--evidence", "sub/../bundle.json"),
		"parent escape":      append(append([]string{}, base...), "--registry", "registry.yaml", "--evidence", "../bundle.json"),
		"absolute":           append(append([]string{}, base...), "--registry", "registry.yaml", "--evidence", outside),
		"missing registry":   append(append([]string{}, base...), "--registry", "nope.yaml", "--evidence", "bundle.json"),
		"empty registry arg": append(append([]string{}, base...), "--registry", "", "--evidence", "bundle.json"),
		"local protected ref": {"policy", "check", "--registry", "registry.yaml", "--evidence", "bundle.json",
			"--protected-ref", "refs/heads/main", "--format", "json"},
		"unknown format": append(append([]string{}, base[:len(base)-2]...), "--registry", "registry.yaml", "--evidence", "bundle.json", "--format", "xml"),
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := runPolicy(t, f.root, args...)
			require.Error(t, err)
		})
	}
}

func TestU163_003_PolicyCheckOversizedInputsAreRefused(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	installPolicySeams(t, f.mem, false)
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "big.json"), bytes.Repeat([]byte("a"), policyengine.MaxBundleBytes+10), 0o600))
	_, err := runPolicy(t, f.root, "policy", "check", "--registry", "registry.yaml", "--evidence", "big.json",
		"--protected-ref", "refs/remotes/origin/feat/x", "--format", "json")
	require.Error(t, err)
}

func TestU163_003_PolicyCheckOverridesNeedAnInteractiveContext(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	f.records = f.records[1:]
	f.writeBundle("bundle.json", nil)
	require.NoError(t, os.WriteFile(filepath.Join(f.root, "token.json"), []byte(`{"schema_version":1}`), 0o600))
	args := []string{"policy", "check", "--registry", "registry.yaml", "--evidence", "bundle.json",
		"--protected-ref", "refs/remotes/origin/feat/x", "--repo", "o/r", "--format", "json", "--override-token", "token.json"}

	for _, interactive := range []bool{false, true} {
		installPolicySeams(t, f.mem, interactive)
		out, err := runPolicy(t, f.root, args...)
		require.Error(t, err, "an override can never pass without a verified signed token")
		var d policyengine.Decision
		require.NoError(t, json.Unmarshal([]byte(out), &d), out)
		require.Len(t, d.OverrideDenials, 1)
		if interactive {
			assert.Contains(t, d.OverrideDenials[0].Reason, "not enabled")
		} else {
			assert.Contains(t, d.OverrideDenials[0].Reason, "non-interactive")
		}
	}
}

func TestU163_003_PolicyVerifyOverridesDetectsTampering(t *testing.T) {
	f := newPolicyFixture(t, policyTestRegistry)
	outside := t.TempDir()
	lpath := filepath.Join(outside, "overrides.jsonl")
	apath := filepath.Join(outside, "anchor.json")

	out, err := runPolicy(t, f.root, "policy", "verify-overrides", "--ledger", lpath, "--anchor", apath, "--format", "json")
	require.NoError(t, err, out)
	var log policyengine.OverrideLog
	require.NoError(t, json.Unmarshal([]byte(out), &log), out)
	assert.Zero(t, log.Records)

	ledger, err := evidencedag.OpenLedger(lpath, f.root)
	require.NoError(t, err)
	_, err = ledger.Append(evidencedag.LedgerEntry{
		AcceptedAt: policyT0.Unix(), ShipmentID: f.task, NodeID: "red-phase-verify", HeadCommit: policySHA(5),
		Principal: "alice", KeyID: "alice-key", Family: "workflow-policy", Kind: "waiver",
		Nonce: strings.Repeat("a", 32), Reason: "harness-exempt", TokenDigest: strings.Repeat("b", 64),
	})
	require.NoError(t, err)
	// The ledger now extends a genesis anchor: that is a forged tail.
	_, err = runPolicy(t, f.root, "policy", "verify-overrides", "--ledger", lpath, "--anchor", apath, "--format", "json")
	require.Error(t, err)

	inside := filepath.Join(f.root, "overrides.jsonl")
	_, err = runPolicy(t, f.root, "policy", "verify-overrides", "--ledger", inside, "--anchor", apath, "--format", "json")
	require.Error(t, err, "a ledger inside the agent-writable workspace is refused")
	_, err = runPolicy(t, f.root, "policy", "verify-overrides", "--ledger", "relative.jsonl", "--anchor", apath, "--format", "json")
	require.Error(t, err, "ledger paths must be absolute")
}
