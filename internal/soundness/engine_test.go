package soundness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// soundUnit returns a task that satisfies every wave-1 rule.
func soundUnit(id, title string) Unit {
	return Unit{
		ID: id, Kind: KindTask, Title: title,
		Files:            []string{"internal/x/a.go"},
		Functions:        []string{"A"},
		Scenarios:        []string{"happy"},
		Responsibilities: []string{"parse"},
		Verification:     []string{"go test ./internal/x"},
		Acceptance:       []string{"A returns the parsed value"},
	}
}

func rulesOf(r Report) map[string][]Finding {
	m := map[string][]Finding{}
	for _, f := range r.Findings {
		m[f.Rule] = append(m[f.Rule], f)
	}
	return m
}

func runPlan(t *testing.T, plan Plan, cfg Config) Report {
	t.Helper()
	r, err := Run(plan, cfg)
	require.NoError(t, err)
	return r
}

func TestU160001_SoundPlanPasses(t *testing.T) {
	cfg := DefaultConfig()
	r := runPlan(t, Plan{Units: []Unit{soundUnit("1-T", "Parse x")}}, cfg)
	assert.Empty(t, r.Findings)
	assert.NotEmpty(t, r.RulesRun, "engine must actually run rules")
}

func TestU160001_OverBudgetFlagged(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*Unit)
	}{
		{"files", func(u *Unit) { u.Files = []string{"a", "b", "c", "d"} }},
		{"functions", func(u *Unit) { u.Functions = []string{"1", "2", "3", "4", "5", "6"} }},
		{"scenarios", func(u *Unit) { u.Scenarios = []string{"1", "2", "3", "4", "5"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := soundUnit("1-T", "Parse x")
			tc.mut(&u)
			r := runPlan(t, Plan{Units: []Unit{u}}, DefaultConfig())
			got := rulesOf(r)["budget"]
			require.Len(t, got, 1)
			assert.Equal(t, "1-T", got[0].UnitID)
			assert.Contains(t, got[0].Message, tc.name)
		})
	}
}

func TestU160001_BudgetsConfigurable(t *testing.T) {
	u := soundUnit("1-T", "Parse x")
	u.Files = []string{"a", "b", "c", "d"}
	cfg := DefaultConfig()
	cfg.Budgets.MaxFiles = 4
	assert.Empty(t, rulesOf(runPlan(t, Plan{Units: []Unit{u}}, cfg))["budget"])
	cfg.Budgets.MaxFiles = 2
	assert.NotEmpty(t, rulesOf(runPlan(t, Plan{Units: []Unit{u}}, cfg))["budget"])
}

func TestU160001_MultiResponsibilityFlagged(t *testing.T) {
	u := soundUnit("1-T", "Parse x")
	u.Responsibilities = []string{"parse", "render"}
	got := rulesOf(runPlan(t, Plan{Units: []Unit{u}}, DefaultConfig()))["one-responsibility"]
	require.Len(t, got, 1)
	assert.Equal(t, "1-T", got[0].UnitID)
}

func TestU160001_ReferencesResolved(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "a.md"), []byte("x\ny\n"), 0o600))
	cfg := DefaultConfig()
	cfg.Root = root

	ok := soundUnit("1-T", "Parse x")
	ok.References = []string{"docs/a.md", "docs/a.md:2"}
	assert.Empty(t, rulesOf(runPlan(t, Plan{Units: []Unit{ok}}, cfg))["references"])

	for name, ref := range map[string]string{
		"missing":      "docs/none.md",
		"out-of-root":  "../secret.md",
		"absolute":     filepath.Join(root, "docs", "a.md"),
		"line-too-far": "docs/a.md:99",
	} {
		t.Run(name, func(t *testing.T) {
			bad := soundUnit("1-T", "Parse x")
			bad.References = []string{ref}
			assert.Len(t, rulesOf(runPlan(t, Plan{Units: []Unit{bad}}, cfg))["references"], 1)
		})
	}
}

func TestU160001_ReferencesWithoutRootFailClosed(t *testing.T) {
	u := soundUnit("1-T", "Parse x")
	u.References = []string{"docs/a.md"}
	got := rulesOf(runPlan(t, Plan{Units: []Unit{u}}, DefaultConfig()))["references"]
	require.Len(t, got, 1)
	assert.Equal(t, SeverityError, got[0].Severity)
}

func TestU160001_VerificationSurfaceRequired(t *testing.T) {
	u := soundUnit("1-T", "Parse x")
	u.Verification = nil
	got := rulesOf(runPlan(t, Plan{Units: []Unit{u}}, DefaultConfig()))["verification-surface"]
	require.Len(t, got, 1)
	assert.Equal(t, "1-T", got[0].UnitID)
}

func TestU160001_UnverifiableUnitFailsClosed(t *testing.T) {
	cases := map[string]Unit{
		"empty-id":     {Kind: KindTask, Title: "t"},
		"unknown-kind": {ID: "9-X", Kind: "epic", Title: "t"},
	}
	for name, u := range cases {
		t.Run(name, func(t *testing.T) {
			got := rulesOf(runPlan(t, Plan{Units: []Unit{u}}, DefaultConfig()))["unverifiable-unit"]
			require.Len(t, got, 1)
			assert.Equal(t, SeverityError, got[0].Severity)
		})
	}
	t.Run("duplicate-id", func(t *testing.T) {
		p := Plan{Units: []Unit{soundUnit("1-T", "a"), soundUnit("1-T", "b")}}
		assert.NotEmpty(t, rulesOf(runPlan(t, p, DefaultConfig()))["unverifiable-unit"])
	})
	t.Run("cannot-be-disabled", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Rules["unverifiable-unit"] = false
		_, err := Run(Plan{Units: []Unit{{Kind: KindTask}}}, cfg)
		assert.Error(t, err)
	})
}

func TestU160001_ConfigFromText(t *testing.T) {
	cfg, err := ParseConfig("# budgets\nmax_files: 7\nmax_functions = 8\nmax_scenarios: 9\nmax_responsibilities: 2\nmode: enforce\nrule.budget: off\ncanonical_procedures: build, review\n")
	require.NoError(t, err)
	assert.Equal(t, Budgets{MaxFiles: 7, MaxFunctions: 8, MaxScenarios: 9, MaxResponsibilities: 2}, cfg.Budgets)
	assert.Equal(t, ModeEnforce, cfg.Mode)
	assert.False(t, cfg.Rules["budget"])
	assert.Equal(t, []string{"build", "review"}, cfg.CanonicalProcedures)

	def, err := ParseConfig("")
	require.NoError(t, err)
	assert.Equal(t, DefaultConfig().Budgets, def.Budgets)
	assert.Equal(t, ModeReportOnly, def.Mode)
}

func TestU160001_ConfigRejectsBadInput(t *testing.T) {
	for name, text := range map[string]string{
		"unknown-key":   "bogus: 1",
		"bad-int":       "max_files: many",
		"zero-budget":   "max_files: 0",
		"bad-mode":      "mode: warn",
		"bad-flag":      "rule.budget: maybe",
		"unknown-rule":  "rule.nope: off",
		"duplicate-key": "max_files: 1\nmax_files: 2",
		"no-separator":  "max_files",
	} {
		t.Run(name, func(t *testing.T) {
			cfg, err := ParseConfig(text)
			if err == nil {
				_, err = Run(Plan{}, cfg)
			}
			assert.Error(t, err)
		})
	}
}

func TestU160001_RuleFlagsDisableRules(t *testing.T) {
	u := soundUnit("1-T", "Parse x")
	u.Responsibilities = []string{"a", "b"}
	cfg := DefaultConfig()
	cfg.Rules["one-responsibility"] = false
	r := runPlan(t, Plan{Units: []Unit{u}}, cfg)
	assert.Empty(t, rulesOf(r)["one-responsibility"])
	assert.Contains(t, r.RulesSkipped, "one-responsibility")
}
