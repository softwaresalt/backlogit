package soundness

import (
	"fmt"
)

// GateDecision is the outcome of the pre-assembly gate. In report-only mode
// Blocked is always false and every finding is downgraded to warn severity
// (the original error severity is intentionally not preserved in Findings;
// the unmodified findings remain available in Report). In enforce mode Blocked
// is true when any error-severity finding exists. A decision produced by a
// misconfigured or zero-value gate is Blocked.
type GateDecision struct {
	Mode     Mode      `json:"mode"`
	Blocked  bool      `json:"blocked"`
	Findings []Finding `json:"findings"`
	Report   Report    `json:"report"`
}

// Gate runs the soundness engine as a pre-assembly gate. Wiring it into
// shipment assembly is a follow-up; it holds no
// rule logic of its own: every decision comes from the shared Run engine.
type Gate struct {
	cfg    Config
	engine func(Plan, Config) (Report, error)
}

// RuleIDs returns the IDs of every flaggable rule in execution order.
func RuleIDs() []string {
	rules := allRules()
	ids := make([]string, len(rules))
	for i, r := range rules {
		ids[i] = r.id
	}
	return ids
}

// Validate reports whether cfg can be honored. Enforce mode additionally
// requires at least one enabled rule, so a fully disabled rule set can never
// silently pass every plan.
func (c Config) Validate() error {
	known := map[string]bool{}
	for _, id := range RuleIDs() {
		known[id] = true
	}
	if err := c.validate(known); err != nil {
		return err
	}
	if c.Mode == ModeEnforce {
		for id := range known {
			if c.enabled(id) {
				return nil
			}
		}
		return fmt.Errorf("%w: enforce mode requires at least one enabled rule", ErrInvalidConfig)
	}
	return nil
}

// failsClosed reports whether a misconfigured run in mode must be treated as
// blocked. Only the explicit report-only modes (including the unset default)
// may proceed; an unknown mode fails closed.
func failsClosed(mode Mode) bool { return mode != "" && mode != ModeReportOnly }

// NewGate builds a gate over cfg. A misconfigured cfg is an explicit error.
func NewGate(cfg Config) (*Gate, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("new soundness gate: %w", err)
	}
	return &Gate{cfg: cloneConfig(cfg), engine: Run}, nil
}

// cloneConfig copies the reference-typed fields so later caller mutation
// cannot change a constructed gate.
func cloneConfig(cfg Config) Config {
	rules := make(map[string]bool, len(cfg.Rules))
	for k, v := range cfg.Rules {
		rules[k] = v
	}
	cfg.Rules = rules
	cfg.CanonicalProcedures = append([]string(nil), cfg.CanonicalProcedures...)
	return cfg
}

// Assess evaluates plan. An engine failure in enforce mode returns a blocked
// decision together with the error, so enforcement never fails open.
func (g *Gate) Assess(plan Plan) (GateDecision, error) {
	if g == nil || g.engine == nil {
		return GateDecision{Blocked: true}, fmt.Errorf("%w: gate was not built with NewGate", ErrInvalidConfig)
	}
	mode, err := parseMode(string(g.cfg.Mode))
	if err != nil {
		return GateDecision{Blocked: true}, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	run := g.engine
	report, err := run(plan, g.cfg)
	if err != nil {
		return GateDecision{Mode: mode, Blocked: failsClosed(mode)}, fmt.Errorf("assess plan: %w", err)
	}
	d := GateDecision{Mode: mode, Report: report, Findings: []Finding{}}
	for _, f := range report.Findings {
		if mode == ModeReportOnly {
			f.Severity = SeverityWarn
		} else if f.Severity == SeverityError {
			d.Blocked = true
		}
		d.Findings = append(d.Findings, f)
	}
	return d, nil
}

// Assess is the library entrypoint for the soundness gate; it runs the same
// Run engine as every other caller.
func Assess(plan Plan, cfg Config) (GateDecision, error) {
	g, err := NewGate(cfg)
	if err != nil {
		return GateDecision{Blocked: failsClosed(cfg.Mode)}, err
	}
	return g.Assess(plan)
}
