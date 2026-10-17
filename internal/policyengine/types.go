package policyengine

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

// SchemaVersion is the registry schema version. Any other version is refused.
const SchemaVersion = 1

// ValidatorIdentity is the validator recorded on every policy node.
const ValidatorIdentity = "backlogit/policyengine@v1"

// Resource bounds. Every input and output of this package is bounded.
const (
	// MaxRegistryEdges caps the edges in a registry. The canonical graph needs
	// five; the cap leaves room for redundant ordering edges only.
	MaxRegistryEdges = 32
	// MaxErrorBytes caps any single human-readable reason in a result.
	MaxErrorBytes = 256
	// maxErrorText caps a wrapped error message.
	maxErrorText = 1024
)

// ErrInvalidRegistry classifies every registry declaration defect.
var ErrInvalidRegistry = errors.New("policyengine: invalid registry")

// NodeType is one of the closed set of policy gate node types. A registry
// node's ID is its type; each type is declared exactly once.
type NodeType string

// The policy gate node types, in canonical prerequisite order.
const (
	// NodeHarnessScaffold is the failing-test harness scaffold (P-002).
	NodeHarnessScaffold NodeType = "harness-scaffold"
	// NodeRedPhaseVerify is the verified honest red phase (P-004, S9).
	NodeRedPhaseVerify NodeType = "red-phase-verify"
	// NodeGreenPhaseVerify is the verified green phase.
	NodeGreenPhaseVerify NodeType = "green-phase-verify"
	// NodeReviewComplete is the derived review completion (S7).
	NodeReviewComplete NodeType = "review-complete"
	// NodeCIPass is the derived CI success (S7).
	NodeCIPass NodeType = "ci-pass"
	// NodeMergeReady is the terminal merge-readiness attestation.
	NodeMergeReady NodeType = "merge-ready"
)

// NodeTypes returns the six policy node types in canonical order.
func NodeTypes() []NodeType {
	return []NodeType{
		NodeHarnessScaffold, NodeRedPhaseVerify, NodeGreenPhaseVerify,
		NodeReviewComplete, NodeCIPass, NodeMergeReady,
	}
}

// Mode is the engine enforcement mode, shared with the S10 gate.
type Mode = evidencedag.Mode

// Enforcement modes.
const (
	// ModeReportOnly reports every violation but allows the transition.
	ModeReportOnly = evidencedag.ModeReportOnly
	// ModeEnforce blocks any violation.
	ModeEnforce = evidencedag.ModeEnforce
)

// GateDecl is one gate in the registry.
type GateDecl struct {
	ID NodeType
	// Enabled is the per-gate rollout flag. A disabled gate is reported as
	// not applicable and neither blocks nor satisfies its dependents.
	Enabled bool
	// Overridable states whether a signed operator override may waive the gate
	// (P-002.1 harness-exempt applies to the harness and red-phase gates).
	Overridable bool
}

// clip bounds s to n bytes on a UTF-8 boundary and replaces control and
// format characters so text is safe to render.
func clip(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(s, "?"))
	if len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
