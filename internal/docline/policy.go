package docline

import (
	"errors"
	"strings"
)

// Sentinel errors exported by the docline package. Boundaries wrap these with
// fmt.Errorf("docline.<fn>: %w", err) so callers can match with errors.Is.
var (
	// ErrUnknownDocType indicates a doc_type outside the closed vocabulary.
	ErrUnknownDocType = errors.New("docline: unknown doc_type")
	// ErrMissingRequiredField indicates a required contract field is absent/empty.
	ErrMissingRequiredField = errors.New("docline: missing required field")
	// ErrPathEscapesWorkspace indicates a target path escaped the workspace root.
	ErrPathEscapesWorkspace = errors.New("docline: path escapes workspace")
	// ErrBodyMutated indicates a migration changed body bytes (a hard invariant
	// violation; migration must only change the frontmatter block).
	ErrBodyMutated = errors.New("docline: body bytes mutated")
)

// DocType is a member of the closed controlled vocabulary of document types.
type DocType string

// The closed doc_type vocabulary (see the 065.001-T decision doc for rationale).
const (
	DocTypeReference DocType = "reference"
	DocTypeDecision  DocType = "decision"
	DocTypeSpike     DocType = "spike"
	DocTypePlan      DocType = "plan"
	DocTypeClosure   DocType = "closure"
	DocTypeResearch  DocType = "research"
	DocTypeReview    DocType = "review"
	DocTypeLearning  DocType = "learning"
	DocTypeSpec      DocType = "spec"
	DocTypeDesign    DocType = "design"
	DocTypeGuide     DocType = "guide"
)

// knownDocTypes is the closed membership set. doc_type values outside this set
// fail validation with ErrUnknownDocType.
var knownDocTypes = map[DocType]struct{}{
	DocTypeReference: {},
	DocTypeDecision:  {},
	DocTypeSpike:     {},
	DocTypePlan:      {},
	DocTypeClosure:   {},
	DocTypeResearch:  {},
	DocTypeReview:    {},
	DocTypeLearning:  {},
	DocTypeSpec:      {},
	DocTypeDesign:    {},
	DocTypeGuide:     {},
}

// IsKnownDocType reports whether dt is a member of the closed vocabulary.
func IsKnownDocType(dt string) bool {
	_, ok := knownDocTypes[DocType(dt)]
	return ok
}

// KnownDocTypes returns the closed vocabulary as a sorted-by-declaration slice.
func KnownDocTypes() []DocType {
	return []DocType{
		DocTypeReference, DocTypeDecision, DocTypeSpike, DocTypePlan,
		DocTypeClosure, DocTypeResearch, DocTypeReview, DocTypeLearning,
		DocTypeSpec, DocTypeDesign, DocTypeGuide,
	}
}

// Profile selects which validation rules apply: the repo-owned authoring
// surface, or the full graphtor-docs ingestion surface.
type Profile string

const (
	// ProfileAuthoring validates the fields the repo owns (title, source,
	// doc_type). Used by the CI gate and `docs lint` by default.
	ProfileAuthoring Profile = "authoring"
	// ProfileIngestion validates the full schema-required set (adds ingested_at).
	ProfileIngestion Profile = "ingestion"
)

// requiredFields returns the required top-level contract fields for a profile.
func requiredFields(p Profile) []string {
	switch p {
	case ProfileIngestion:
		return []string{"title", "source", "ingested_at", "doc_type"}
	default: // authoring (and any unknown profile) — the repo-owned subset
		return []string{"title", "source", "doc_type"}
	}
}

// contractFields is the closed set of top-level keys that belong on the docline
// contract surface. Any other key is folded under the docline namespace by the
// normalizer (move, never drop).
var contractFields = map[string]struct{}{
	"title":          {},
	"source":         {},
	"ingested_at":    {},
	"doc_type":       {},
	"description":    {},
	"content_sha256": {},
	"source_path":    {},
	"chunk_strategy": {},
	"schema_version": {},
	"docline":        {},
}

// isContractField reports whether key is a top-level docline contract field.
func isContractField(key string) bool {
	_, ok := contractFields[key]
	return ok
}

// pathRule maps a repo-relative POSIX directory prefix to a doc_type.
type pathRule struct {
	prefix  string
	docType DocType
}

// pathRules are evaluated longest-prefix-first by the classifier (065.005-T).
// Each prefix is a repo-relative POSIX directory path ending in "/".
var pathRules = []pathRule{
	{"docs/cli-reference/", DocTypeReference},
	{"docs/decisions/", DocTypeDecision},
	{"docs/exec-plans/", DocTypePlan},
	{"docs/closure/", DocTypeClosure},
	{"docs/research/", DocTypeResearch},
	{"docs/reviews/", DocTypeReview},
	{"docs/compound/", DocTypeLearning},
	{"docs/design-docs/", DocTypeDesign},
	{"docs/product-specs/", DocTypeSpec},
	{"docs/spikes/", DocTypeSpike},
}

// fileOverrides map exact repo-relative POSIX paths to a doc_type, taking
// precedence over the docs/*.md "guide" default but not over a directory rule.
var fileOverrides = map[string]DocType{
	"docs/ARCHITECTURE.md": DocTypeReference,
	"README.md":            DocTypeGuide,
	"AGENTS.md":            DocTypeGuide,
}

// Scope globs (repo-relative POSIX). The service (065.006-T) consults these to
// decide which files are in scope for lint/migrate.
var (
	// scopeExcludeDirs are directory prefixes never touched.
	scopeExcludeDirs = []string{
		"docs/memory/",
		"docs/archive/",
		".github/",
	}
	// scopeExcludeFiles are exact repo-relative paths never touched.
	scopeExcludeFiles = map[string]struct{}{
		"prompt.md": {},
	}
	// scopeIncludeFiles are exact repo-root files included despite living
	// outside docs/.
	scopeIncludeFiles = map[string]struct{}{
		"README.md": {},
		"AGENTS.md": {},
	}
)

// classifyDocType derives the doc_type for a repo-relative POSIX path using the
// policy tables: longest matching directory prefix wins, then an exact file
// override, then the docs/*.md direct-child default (guide). Paths outside the
// known structure default to guide; callers gate on inScope first.
func classifyDocType(relPath string) DocType {
	best := ""
	var bestType DocType
	for _, r := range pathRules {
		if strings.HasPrefix(relPath, r.prefix) && len(r.prefix) > len(best) {
			best, bestType = r.prefix, r.docType
		}
	}
	if best != "" {
		return bestType
	}
	if dt, ok := fileOverrides[relPath]; ok {
		return dt
	}
	return DocTypeGuide
}

// inScope reports whether a repo-relative POSIX path is within the docline
// standardization scope: excluded files/dirs are rejected, explicit root files
// and the docs/ tree are included.
func inScope(relPath string) bool {
	if _, ok := scopeExcludeFiles[relPath]; ok {
		return false
	}
	for _, d := range scopeExcludeDirs {
		if strings.HasPrefix(relPath, d) {
			return false
		}
	}
	if _, ok := scopeIncludeFiles[relPath]; ok {
		return true
	}
	return strings.HasPrefix(relPath, "docs/")
}
