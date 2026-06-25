package docline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyDocType(t *testing.T) {
	cases := map[string]DocType{
		"docs/cli-reference/backlogit_add.md": DocTypeReference,
		"docs/decisions/x.md":                 DocTypeDecision,
		"docs/exec-plans/x.md":                DocTypePlan,
		"docs/closure/x.md":                   DocTypeClosure,
		"docs/research/x.md":                  DocTypeResearch,
		"docs/reviews/x.md":                   DocTypeReview,
		"docs/compound/x.md":                  DocTypeLearning,
		"docs/design-docs/x.md":               DocTypeDesign,
		"docs/product-specs/x.md":             DocTypeSpec,
		"docs/spikes/x.md":                    DocTypeSpike,
		"docs/ARCHITECTURE.md":                DocTypeReference,
		"docs/installation.md":                DocTypeGuide,
		"README.md":                           DocTypeGuide,
		"AGENTS.md":                           DocTypeGuide,
	}
	for p, want := range cases {
		assert.Equal(t, want, classifyDocType(p), "path %s", p)
	}
}

func TestInScope(t *testing.T) {
	in := []string{
		"docs/closure/x.md",
		"docs/installation.md",
		"docs/ARCHITECTURE.md",
		"README.md",
		"AGENTS.md",
	}
	out := []string{
		"docs/memory/x.md",
		"docs/archive/x.md",
		".github/prompts/x.prompt.md",
		"prompt.md",
		"internal/x.go",
		"schemas/docline/base-frontmatter-v1.schema.json",
	}
	for _, p := range in {
		assert.True(t, inScope(p), "expected in scope: %s", p)
	}
	for _, p := range out {
		assert.False(t, inScope(p), "expected out of scope: %s", p)
	}
}

func TestIsContractField(t *testing.T) {
	for _, f := range []string{"title", "source", "ingested_at", "doc_type", "description", "content_sha256", "source_path", "chunk_strategy", "schema_version", "docline"} {
		assert.True(t, isContractField(f), "contract field: %s", f)
	}
	for _, f := range []string{"tags", "severity", "date", "gate_decision", "ms.date"} {
		assert.False(t, isContractField(f), "non-contract field: %s", f)
	}
}
