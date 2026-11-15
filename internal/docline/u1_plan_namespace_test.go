package docline_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/docline"
)

func TestU1_ClassifyPlanNamespaces(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		relPath string
		want    docline.DocType
	}{
		{name: "canonical plan posix", relPath: "docs/plans/x.md", want: docline.DocTypePlan},
		{name: "legacy plan posix", relPath: "docs/exec-plans/x.md", want: docline.DocTypePlan},
		{name: "near-prefix guide posix", relPath: "docs/plans-extra/x.md", want: docline.DocTypeGuide},
		{name: "canonical plan backslash", relPath: `docs\plans\x.md`, want: docline.DocTypePlan},
		{name: "legacy plan backslash", relPath: `docs\exec-plans\x.md`, want: docline.DocTypePlan},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, docline.Classify(tc.relPath))
		})
	}
}

func TestU1_ValidateClassifyPathContainment(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	invalid := []struct {
		name string
		path string
	}{
		{name: "parent traversal", path: "../x.md"},
		{name: "absolute posix", path: "/abs/x.md"},
		{name: "leading dot segment", path: "./docs/plans/x.md"},
		{name: "volume path", path: `C:\docs\plans\x.md`},
		{name: "unc path", path: `\\host\share\docs\plans\x.md`},
		{name: "mid dotdot traversal", path: "docs/plans/../guides/x.md"},
	}

	for _, tc := range invalid {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, docline.ValidateClassifyPath(root, tc.path))
		})
	}

	valid := []string{
		"docs/plans/x.md",
		"docs/exec-plans/x.md",
		`docs\plans\x.md`,
	}
	for _, p := range valid {
		p := p
		t.Run("valid_"+p, func(t *testing.T) {
			t.Parallel()
			require.NoError(t, docline.ValidateClassifyPath(root, p))
		})
	}
}

func TestU1_ScopeIncludesBothPlanPrefixes(t *testing.T) {
	t.Parallel()
	scope := docline.Scope()
	assert.Equal(t, string(docline.DocTypePlan), scope.PathMap["docs/plans/"])
	assert.Equal(t, string(docline.DocTypePlan), scope.PathMap["docs/exec-plans/"])
}

func TestU1_PlanMigrationCanonicalAndLegacyFixtures(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 11, 15, 0, 0, 0, 0, time.UTC)

	fixtures := []struct {
		relPath string
		title   string
		body    string
	}{
		{
			relPath: "docs/plans/x.md",
			title:   "Canonical Plan",
			body:    "# Canonical Plan\n\nPreserved canonical body.\n",
		},
		{
			relPath: "docs/exec-plans/x.md",
			title:   "Legacy Plan",
			body:    "# Legacy Plan\n\nPreserved legacy body.\n",
		},
	}

	beforeBytes := make(map[string]string, len(fixtures))
	for _, fx := range fixtures {
		raw := "---\ntitle: " + fx.title + "\n---\n" + fx.body
		abs := filepath.Join(root, filepath.FromSlash(fx.relPath))
		require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
		require.NoError(t, os.WriteFile(abs, []byte(raw), 0o644))
		beforeBytes[fx.relPath] = raw
	}

	plan, err := docline.PlanMigration(docline.Options{Root: root, Now: now})
	require.NoError(t, err)
	require.Empty(t, plan.Findings)
	require.Len(t, plan.Changes, len(fixtures))

	byFile := make(map[string]docline.Change, len(plan.Changes))
	for _, c := range plan.Changes {
		byFile[c.File] = c
	}

	for _, fx := range fixtures {
		c, ok := byFile[fx.relPath]
		require.True(t, ok, "expected change entry for %s", fx.relPath)
		assert.False(t, c.BodyBytesChanged, "body bytes must remain unchanged for %s", fx.relPath)

		md, err := docline.Decode([]byte(c.After))
		require.NoError(t, err)
		assert.Equal(t, string(docline.DocTypePlan), md.Frontmatter["doc_type"], "doc_type for %s", fx.relPath)
		assert.Equal(t, fx.relPath, md.Frontmatter["source"], "source for %s", fx.relPath)
		assert.Equal(t, fx.body, string(md.Body), "decoded body for %s", fx.relPath)

		onDisk, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(fx.relPath)))
		require.NoError(t, err)
		assert.Equal(t, beforeBytes[fx.relPath], string(onDisk), "PlanMigration must not write %s", fx.relPath)
	}
}
