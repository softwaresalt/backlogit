package soundness

import (
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func unsoundPlan() Plan {
	u := soundUnit("1-T", "A")
	u.Responsibilities = []string{"a", "b"}
	return Plan{Units: []Unit{u}}
}

func soundPlan() Plan { return Plan{Units: []Unit{soundUnit("1-T", "A")}} }

func TestU160004_ReportOnlyIsDefaultAndWarnsWithoutBlocking(t *testing.T) {
	assert.Equal(t, ModeReportOnly, DefaultConfig().Mode)
	for name, mode := range map[string]Mode{"explicit": ModeReportOnly, "empty": ""} {
		t.Run(name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Mode = mode
			d, err := Assess(unsoundPlan(), cfg)
			require.NoError(t, err)
			assert.False(t, d.Blocked)
			assert.Equal(t, ModeReportOnly, d.Mode)
			require.NotEmpty(t, d.Findings)
			assert.Equal(t, "one-responsibility", d.Findings[0].Rule)
			assert.Equal(t, SeverityWarn, d.Findings[0].Severity)
		})
	}
}

func TestU160004_EnforceBlocksWithSpecificFindings(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeEnforce
	d, err := Assess(unsoundPlan(), cfg)
	require.NoError(t, err)
	assert.True(t, d.Blocked)
	require.NotEmpty(t, d.Findings)
	assert.Equal(t, "1-T", d.Findings[0].UnitID)
	assert.Equal(t, SeverityError, d.Findings[0].Severity)
}

func TestU160004_SoundPlanProceedsInBothModes(t *testing.T) {
	for _, mode := range []Mode{ModeReportOnly, ModeEnforce} {
		cfg := DefaultConfig()
		cfg.Mode = mode
		d, err := Assess(soundPlan(), cfg)
		require.NoError(t, err)
		assert.False(t, d.Blocked, mode)
		assert.Empty(t, d.Findings, mode)
	}
}

func TestU160004_PerRuleFlagsApplyInEnforce(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeEnforce
	cfg.Rules["one-responsibility"] = false
	d, err := Assess(unsoundPlan(), cfg)
	require.NoError(t, err)
	assert.False(t, d.Blocked)
	assert.Contains(t, d.Report.RulesSkipped, "one-responsibility")
}

func TestU160004_EnforceNeverFailsOpenOnMisconfig(t *testing.T) {
	cases := map[string]func(*Config){
		"unknown-rule-flag": func(c *Config) { c.Rules["nope"] = false },
		"bad-mode":          func(c *Config) { c.Mode = "block" },
		"bad-budget":        func(c *Config) { c.Budgets.MaxFiles = 0 },
		"all-rules-off": func(c *Config) {
			for _, id := range RuleIDs() {
				c.Rules[id] = false
			}
		},
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Mode = ModeEnforce
			mut(&cfg)
			d, err := Assess(soundPlan(), cfg)
			require.Error(t, err, "misconfig must be an explicit error, not a pass")
			assert.True(t, errors.Is(err, ErrInvalidConfig))
			assert.True(t, d.Blocked || cfg.Mode != ModeEnforce)
		})
	}
}

func TestU160004_EngineFailureBlocksEnforce(t *testing.T) {
	boom := errors.New("boom")
	g := &Gate{cfg: Config{Mode: ModeEnforce}, engine: func(Plan, Config) (Report, error) { return Report{}, boom }}
	d, err := g.Assess(soundPlan())
	require.ErrorIs(t, err, boom)
	assert.True(t, d.Blocked)
}

func TestU160004_GateAndEntrypointShareOneEngine(t *testing.T) {
	g, err := NewGate(DefaultConfig())
	require.NoError(t, err)
	assert.Equal(t, reflect.ValueOf(Run).Pointer(), reflect.ValueOf(g.engine).Pointer(), "gate must call Run")

	calls := 0
	g.engine = func(p Plan, c Config) (Report, error) { calls++; return Run(p, c) }
	_, err = g.Assess(soundPlan())
	require.NoError(t, err)
	assert.Equal(t, 1, calls)

	cfg := DefaultConfig()
	cfg.Mode = ModeEnforce
	viaFunc, err := Assess(unsoundPlan(), cfg)
	require.NoError(t, err)
	gate, err := NewGate(cfg)
	require.NoError(t, err)
	viaGate, err := gate.Assess(unsoundPlan())
	require.NoError(t, err)
	assert.Equal(t, viaFunc, viaGate)
}

func TestU160004_RuleErrorFindingBlocksEnforce(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = ModeEnforce
	u := soundUnit("1-T", "A")
	u.References = []string{"docs/a.md"} // no root: unverifiable, fail closed
	d, err := Assess(Plan{Units: []Unit{u}}, cfg)
	require.NoError(t, err)
	assert.True(t, d.Blocked)
}
