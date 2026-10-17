package evidencedag

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

// SchemaVersion is the current DAG declaration schema version. Any other
// version is refused with ErrUnsupportedVersion.
const SchemaVersion = 1

// Resource bounds. Every input and output of this package is bounded so a
// hostile declaration, evidence file or ledger cannot exhaust memory or CPU.
const (
	// MaxDeclarationBytes caps the size of a DAG declaration document.
	MaxDeclarationBytes = 256 << 10
	// MaxNodes caps the number of nodes in a declaration.
	MaxNodes = 512
	// MaxEdges caps the number of edges in a declaration.
	MaxEdges = 4096
	// MaxIDLength caps the length of node and shipment identifiers.
	MaxIDLength = 96
	// MaxRequiresKeys caps the applicability predicate size per node.
	MaxRequiresKeys = 32
	// MaxRequiresValues caps the allowed values per applicability key.
	MaxRequiresValues = 64
	// MaxAttributes caps the shipment attribute count.
	MaxAttributes = 256
	// MaxReasonBytes caps any human-readable reason carried in results.
	MaxReasonBytes = 256
	// MaxReportBytes caps rendered status output.
	MaxReportBytes = 256 << 10
)

// Typed error categories. Callers classify results with errors.Is.
var (
	// ErrInvalidDeclaration indicates a malformed or invalid DAG declaration.
	ErrInvalidDeclaration = errors.New("evidencedag: invalid declaration")
	// ErrUnsupportedVersion indicates an unknown schema version.
	ErrUnsupportedVersion = errors.New("evidencedag: unsupported schema version")
	// ErrInvalidInput indicates an invalid evaluation input.
	ErrInvalidInput = errors.New("evidencedag: invalid input")
)

// Family is a fault-line DAG family.
type Family string

// The initial fault-line DAG families.
const (
	// FamilyParity covers cross-surface parity evidence (S4).
	FamilyParity Family = "parity"
	// FamilyMutationState covers mutation/state-transition evidence.
	FamilyMutationState Family = "mutation-state"
	// FamilyCompatError covers compatibility/error-contract evidence.
	FamilyCompatError Family = "compat-error"
	// FamilyDocsAudit covers docs/audit evidence.
	FamilyDocsAudit Family = "docs-audit"
	// FamilyPlanTaskSoundness covers plan/task soundness evidence (S8).
	FamilyPlanTaskSoundness Family = "plan-task-soundness"
	// FamilyRedTestHonesty covers red-test honesty evidence (S9).
	FamilyRedTestHonesty Family = "red-test-honesty"
)

// Families returns the closed, sorted set of known families.
func Families() []Family {
	return []Family{
		FamilyCompatError, FamilyDocsAudit, FamilyMutationState,
		FamilyParity, FamilyPlanTaskSoundness, FamilyRedTestHonesty,
	}
}

func knownFamily(f Family) bool {
	for _, k := range Families() {
		if k == f {
			return true
		}
	}
	return false
}

// Status is a node evaluation status.
type Status string

// Node statuses. Anything that cannot be positively established is StatusError
// or StatusUnmet; nothing fails open.
const (
	// StatusVerified means conformant, authentic, passing evidence exists.
	StatusVerified Status = "verified"
	// StatusUnmet means required evidence is missing, failing or an upstream is unmet.
	StatusUnmet Status = "unmet"
	// StatusWaived means an authorized, authenticated waiver covers the node.
	StatusWaived Status = "waived"
	// StatusNotApplicable means the declared applicability predicate excludes the node.
	StatusNotApplicable Status = "not-applicable"
	// StatusError means the node could not be verified (unverifiable).
	StatusError Status = "error"
)

// OnUnmet declares the blocking semantics of a node.
type OnUnmet string

// Blocking semantics.
const (
	// OnUnmetBlock means an unmet applicable terminal node blocks review-ready.
	OnUnmetBlock OnUnmet = "block"
	// OnUnmetReport means an unmet terminal node is reported but does not block.
	OnUnmetReport OnUnmet = "report"
)

// CommitHead is the ProducingCommit value that binds evidence to the
// shipment head commit.
const CommitHead = "head"

// Applicability is a declarative predicate resolved against shipment
// attributes. Exactly one of Always or a non-empty Requires must be set.
// Requires maps an attribute key to the set of values that make the node
// applicable; all keys must match (logical AND).
type Applicability struct {
	Always   bool                `json:"always" yaml:"always"`
	Requires map[string][]string `json:"requires,omitempty" yaml:"requires,omitempty"`
}

// Node is one required check in a shipment DAG.
type Node struct {
	ID               string         `json:"id" yaml:"id"`
	Family           Family         `json:"family" yaml:"family"`
	Applicability    *Applicability `json:"applicability" yaml:"applicability"`
	Validator        string         `json:"validator" yaml:"validator"`
	RequiredEvidence string         `json:"required_evidence,omitempty" yaml:"required_evidence,omitempty"`
	ProducingTask    string         `json:"producing_task" yaml:"producing_task"`
	ProducingCommit  string         `json:"producing_commit" yaml:"producing_commit"`
	WaiverFamily     Family         `json:"waiver_family,omitempty" yaml:"waiver_family,omitempty"`
	OnUnmet          OnUnmet        `json:"on_unmet" yaml:"on_unmet"`
	Terminal         bool           `json:"terminal" yaml:"terminal"`
}

// WaiverAuthorityFamily returns the family used for waiver authorization.
func (n Node) WaiverAuthorityFamily() Family {
	if n.WaiverFamily != "" {
		return n.WaiverFamily
	}
	return n.Family
}

// Edge is an evidence dependency: Node needs verified evidence from DependsOn.
type Edge struct {
	Node      string `json:"node" yaml:"node"`
	DependsOn string `json:"depends_on" yaml:"depends_on"`
}

// Declaration is a shipment evidence DAG declaration.
type Declaration struct {
	SchemaVersion int    `json:"schema_version" yaml:"schema_version"`
	Nodes         []Node `json:"nodes" yaml:"nodes"`
	Edges         []Edge `json:"edges" yaml:"edges"`
}

// Shipment identifies the shipment under evaluation and carries the declared
// attributes used to resolve node applicability.
type Shipment struct {
	ID         string
	HeadCommit string
	Attributes map[string]string
}

var (
	idPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
	taskPattern = regexp.MustCompile(`^\d{3,}(\.\d{3,})*-[A-Z]{1,2}$`)
)

func validID(s string) bool {
	return len(s) > 0 && len(s) <= MaxIDLength && idPattern.MatchString(s)
}

// truncate bounds s to MaxReasonBytes on a UTF-8 boundary and strips control
// characters so reasons are safe to render.
func truncate(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(s, "?"))
	if len(s) <= MaxReasonBytes {
		return s
	}
	cut := MaxReasonBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
