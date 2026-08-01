package core

import (
	"sort"

	"github.com/softwaresalt/backlogit/internal/models"
)

// F3 (106.002-T): the authoritative artifact-status taxonomy.
//
// Completion is not a single boolean. Different subsystems ask DIFFERENT questions
// about a status, so the taxonomy exposes named, context-specific predicates rather
// than one shared "is terminal" flag. Two truth tables diverge DELIBERATELY and MUST
// NOT be unified:
//
//   - The 6-status CASCADE set {done, accepted, archived, shipped, abandoned,
//     rejected} governs the downward blocking cascade (IsCascadeTerminalStatus) and
//     queue no-longer-blocking dependency resolution (IsNoLongerBlockingStatus). Here
//     shipped and abandoned DO stop blocking dependents.
//   - The 4-status RELEASABLE set {done, accepted, rejected, archived}
//     (IsReleasableStatus) governs release relocation/lifecycle transitions and
//     OMITS shipped and abandoned.
//
// The DESCOPE-eligibility predicate (isDescopeEligibleStatus) and the recognized
// -status allowlist (isRecognizedReleaseStatus) live with the shipment
// release-progression logic in shipment_lifecycle.go; both are part of this
// taxonomy and are pinned by the F3 characterization tests.
//
// archived vs archived_status: every predicate here treats the status "archived"
// LITERALLY and IGNORES the archived_status helper. The archived_status composition
// (restoring the pre-archive provenance) is scoped to the RESTORE path only. Because
// the SQLite index omits archived_status and loadArtifact is index-first, any future
// predicate that genuinely needs the pre-archive provenance must read it from the
// Markdown source and FAIL CLOSED on an empty/unrecognized value via
// isRecognizedReleaseStatus. The cascade, releasable, and gate-target predicates are
// pure-status and therefore need no archived_status read.

// terminalCascadeStatuses is the UNEXPORTED, immutable backing set for the 6-status
// downward blocking cascade. It replaces the previously exported mutable
// TerminalStatuses slice: being unexported it cannot be mutated from another
// package, and it is never mutated after initialization within this package. Read it
// only through IsCascadeTerminalStatus, IsNoLongerBlockingStatus, or the copying
// accessor CascadeTerminalStatuses.
var terminalCascadeStatuses = map[models.ArtifactStatus]struct{}{
	models.StatusDone:      {},
	models.StatusAccepted:  {},
	models.StatusArchived:  {},
	models.StatusShipped:   {},
	models.StatusAbandoned: {},
	models.StatusRejected:  {},
}

// releasableStatuses is the UNEXPORTED, immutable backing set for the 4-status
// release-progression taxonomy. It deliberately OMITS shipped and abandoned, which
// keeps it distinct from terminalCascadeStatuses. Read it only through
// IsReleasableStatus.
var releasableStatuses = map[models.ArtifactStatus]struct{}{
	models.StatusDone:     {},
	models.StatusAccepted: {},
	models.StatusRejected: {},
	models.StatusArchived: {},
}

// IsCascadeTerminalStatus reports whether status satisfies the children-terminal /
// parent-completion predicate: a parent may move to a terminal status only when
// every child is in one of the 6-status cascade set {done, accepted, archived,
// shipped, abandoned, rejected}. Callers pass the raw DB/request status string;
// unknown or empty values fail closed (false).
func IsCascadeTerminalStatus(status string) bool {
	_, ok := terminalCascadeStatuses[models.ArtifactStatus(status)]
	return ok
}

// IsNoLongerBlockingStatus reports whether a dependency in status has stopped
// blocking its dependents for queue resolution. It is a DISTINCT named predicate
// from IsCascadeTerminalStatus but ALIASES the same 6-status cascade set today
// (shipped and abandoned dependencies no longer block). It is named separately so
// the two questions can diverge later without touching call sites. Unknown or empty
// values fail closed (still blocking).
func IsNoLongerBlockingStatus(status string) bool {
	return IsCascadeTerminalStatus(status)
}

// CascadeTerminalStatuses returns a sorted COPY of the 6-status cascade set. It
// returns a fresh slice on every call so callers cannot mutate the backing
// taxonomy. Prefer IsCascadeTerminalStatus / IsNoLongerBlockingStatus for membership
// tests; use this accessor only when the full set is needed (e.g. diagnostics).
func CascadeTerminalStatuses() []string {
	out := make([]string, 0, len(terminalCascadeStatuses))
	for status := range terminalCascadeStatuses {
		out = append(out, string(status))
	}
	sort.Strings(out)
	return out
}

// IsReleasableStatus reports whether status is in the 4-status release-progression
// set {done, accepted, rejected, archived}. It is the single source of truth for
// release relocation/lifecycle terminality and is behaviorally identical to
// isTerminalReleaseStatus, which delegates to it. Unlike the cascade set this OMITS
// shipped and abandoned; that divergence is intentional and pinned by the F3
// characterization tests.
func IsReleasableStatus(status models.ArtifactStatus) bool {
	_, ok := releasableStatuses[status]
	return ok
}

// IsGateTargetStatus reports whether status is one of the workspace-configured gate
// terminal statuses. It is the ONLY taxonomy predicate parameterized by the
// configured set (every other predicate is pure-status): the gate's completion
// target is operator-configurable rather than a static "done". An empty or nil
// configuredTerminalStatuses falls back to the ["done"] default.
func IsGateTargetStatus(status string, configuredTerminalStatuses []string) bool {
	terms := configuredTerminalStatuses
	if len(terms) == 0 {
		terms = []string{string(models.StatusDone)}
	}
	for _, t := range terms {
		if t == status {
			return true
		}
	}
	return false
}
