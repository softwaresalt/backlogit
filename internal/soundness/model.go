package soundness

// Unit kinds accepted by the engine.
const (
	KindPlan    = "plan"
	KindFeature = "feature"
	KindTask    = "task"
	KindSubtask = "subtask"
)

// Severity classifies a finding. Only SeverityError findings block in
// enforce mode.
type Severity string

// Finding severities.
const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
)

// Stance values for a Directive.
const (
	StanceRequire = "require"
	StanceForbid  = "forbid"
)

// Assertion operators for a TestAssertion.
const (
	OpEqual    = "eq"
	OpNotEqual = "ne"
)

// Unit is one harvested plan, feature, task or subtask presented to the linter.
type Unit struct {
	ID               string      `json:"id"`
	Kind             string      `json:"kind"`
	Title            string      `json:"title"`
	ParentID         string      `json:"parent_id,omitempty"`
	Files            []string    `json:"files,omitempty"`
	Functions        []string    `json:"functions,omitempty"`
	Scenarios        []string    `json:"scenarios,omitempty"`
	Responsibilities []string    `json:"responsibilities,omitempty"`
	References       []string    `json:"references,omitempty"`
	Verification     []string    `json:"verification,omitempty"`
	Acceptance       []string    `json:"acceptance,omitempty"`
	Dependencies     []string    `json:"dependencies,omitempty"`
	Related          []string    `json:"related,omitempty"`
	Requires         []string    `json:"requires,omitempty"`
	Deliverables     []string    `json:"deliverables,omitempty"`
	Procedure        string      `json:"procedure,omitempty"`
	Directives       []Directive `json:"directives,omitempty"`
	Tests            []TestSpec  `json:"tests,omitempty"`
	VerifiedWith     []string    `json:"verified_with,omitempty"`
}

// Directive is a stance a unit or decision takes about a subject.
type Directive struct {
	Subject string `json:"subject"`
	Stance  string `json:"stance"`
}

// Decision is an originating decision that units must not contradict.
type Decision struct {
	ID         string      `json:"id"`
	Directives []Directive `json:"directives"`
}

// TestSpec declares the assertions of one planned test.
type TestSpec struct {
	Name       string          `json:"name"`
	Assertions []TestAssertion `json:"assertions"`
}

// TestAssertion asserts that Subject compares to Value by Op.
type TestAssertion struct {
	Subject string `json:"subject"`
	Op      string `json:"op"`
	Value   string `json:"value"`
}

// Plan is the set of units and decisions evaluated together.
type Plan struct {
	Units     []Unit     `json:"units"`
	Decisions []Decision `json:"decisions,omitempty"`
}

// Finding is one soundness defect reported by a rule.
type Finding struct {
	Rule     string   `json:"rule"`
	UnitID   string   `json:"unit_id,omitempty"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Report is the result of one engine run.
type Report struct {
	Findings     []Finding `json:"findings"`
	RulesRun     []string  `json:"rules_run"`
	RulesSkipped []string  `json:"rules_skipped,omitempty"`
}
