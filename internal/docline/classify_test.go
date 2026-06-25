package docline

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassify(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		relPath string
		want    DocType
	}{
		{"cli-reference", "docs/cli-reference/docs-lint.md", DocTypeReference},
		{"decisions", "docs/decisions/2026-06-22-taxonomy.md", DocTypeDecision},
		{"exec-plans", "docs/exec-plans/some-plan.md", DocTypePlan},
		{"closure", "docs/closure/065-closure.md", DocTypeClosure},
		{"research", "docs/research/topic.md", DocTypeResearch},
		{"reviews", "docs/reviews/065-review.md", DocTypeReview},
		{"compound", "docs/compound/learning.md", DocTypeLearning},
		{"design-docs", "docs/design-docs/api.md", DocTypeDesign},
		{"product-specs", "docs/product-specs/spec.md", DocTypeSpec},
		{"spikes", "docs/spikes/probe.md", DocTypeSpike},
		{"architecture override", "docs/ARCHITECTURE.md", DocTypeReference},
		{"readme override", "README.md", DocTypeGuide},
		{"agents override", "AGENTS.md", DocTypeGuide},
		{"docs direct child default", "docs/CONTRIBUTING.md", DocTypeGuide},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, Classify(tc.relPath))
		})
	}
}

func TestDeriveSource(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "docs/decisions/x.md", DeriveSource("docs/decisions/x.md"))
	// Windows-derived separators are normalized to POSIX.
	assert.Equal(t, "docs/decisions/x.md", DeriveSource(`docs\decisions\x.md`))
}
