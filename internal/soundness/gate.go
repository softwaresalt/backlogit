package soundness

import (
	"fmt"
)

// GateDecision is the outcome of the pre-assembly gate. In report-only mode
// Blocked is always false and Findings carry warn severity; in enforce mode
// Blocked is true when any error-severity finding exists.
type GateDecision struct {
	Mode     Mode      `json:"mode"`
	Blocked  bool      `json:"blocked"`
	Findings []Finding `json:"findings"`
	Report   Report    `json:"report"`
}

// Gate runs the soundness engine as a pre-shipment-assembly gate. It holds no
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
// blocked. Only the explicit report-only modes may proceed.
func failsClosed(mode Mode) bool { return mode != "" && mode != ModeReportOnly }

// NewGate builds a gate over cfg. A misconfigured cfg is an explicit error.
func NewGate(cfg Config) (*Gate, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("new soundness gate: %w", err)
	}
	return &Gate{cfg: cfg, engine: Run}, nil
}

// Assess evaluates plan. An engine failure in enforce mode returns a blocked
// decision together with the error, so enforcement never fails open.
func (g *Gate) Assess(plan Plan) (GateDecision, error) {
	mode, err := parseMode(string(g.cfg.Mode))
	if err != nil {
		return GateDecision{Blocked: true}, fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	report, err := g.engine(plan, g.cfg)
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

// Assess is the library entrypoint shared by the pre-assembly gate and the
// S10 plan/task soundness DAG node: both run the same Run engine.
func Assess(plan Plan, cfg Config) (GateDecision, error) {
	g, err := NewGate(cfg)
	if err != nil {
		return GateDecision{Blocked: failsClosed(cfg.Mode)}, err
	}
	return g.Assess(plan)
}
