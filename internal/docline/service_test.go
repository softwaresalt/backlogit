package docline

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var svcNow = time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

// writeDoc writes content to root/rel, creating parent dirs.
func writeDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	require.NoError(t, os.WriteFile(abs, []byte(content), 0o644))
}

func readDoc(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	require.NoError(t, err)
	return string(b)
}

// newTree builds a small in-scope + out-of-scope doc tree in a temp dir.
func newTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	// In scope, already in canonical normalized form (generated via Normalize so
	// the fixture can never drift from the real output).
	goodRaw := "---\ntitle: Good Decision\ntype: decision\n---\nBody.\n"
	goodNorm, err := Normalize("docs/decisions/good.md", []byte(goodRaw), NormalizeOptions{Now: svcNow})
	require.NoError(t, err)
	writeDoc(t, root, "docs/decisions/good.md", string(goodNorm))
	// In scope, missing required fields (source, doc_type) → lint findings.
	writeDoc(t, root, "docs/reviews/bad.md", "---\ntitle: Bad Review\n---\nBody.\n")
	// In scope, no frontmatter at all.
	writeDoc(t, root, "docs/research/none.md", "# Research\n\nNo frontmatter.\n")
	// Out of scope (memory) — must be ignored.
	writeDoc(t, root, "docs/memory/skip.md", "---\ntitle: Skip\n---\nBody.\n")
	// Root knowledge file in scope.
	writeDoc(t, root, "README.md", "---\ntitle: Readme\n---\nProject.\n")
	return root
}

func TestLintTree_ReportsFindingsWithoutMutation(t *testing.T) {
	t.Parallel()
	root := newTree(t)
	before := readDoc(t, root, "docs/reviews/bad.md")

	findings, err := LintTree(Options{Root: root, Profile: ProfileAuthoring})
	require.NoError(t, err)

	// bad.md is missing source + doc_type.
	var badFields []string
	for _, f := range findings {
		if f.File == "docs/reviews/bad.md" {
			badFields = append(badFields, f.Field)
		}
	}
	assert.Contains(t, badFields, "source")
	assert.Contains(t, badFields, "doc_type")

	// Out-of-scope memory file produced no findings.
	for _, f := range findings {
		assert.NotEqual(t, "docs/memory/skip.md", f.File, "out-of-scope file must not be linted")
	}

	// No mutation occurred.
	assert.Equal(t, before, readDoc(t, root, "docs/reviews/bad.md"))
}

func TestLintTree_SkipsNonDocsTopLevelDirs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// In scope: a docs/ file missing required fields.
	writeDoc(t, root, "docs/decisions/x.md", "---\ntitle: X\n---\nBody.\n")
	// Out of scope: markdown nested under non-docs top-level dirs the walk now
	// skips wholesale (cmd/, internal/, schemas/). None may be linted.
	writeDoc(t, root, "internal/notes.md", "---\ntitle: Internal\n---\nBody.\n")
	writeDoc(t, root, "cmd/tool/readme.md", "---\ntitle: Tool\n---\nBody.\n")
	writeDoc(t, root, "schemas/spec.md", "---\ntitle: Spec\n---\nBody.\n")

	findings, err := LintTree(Options{Root: root, Profile: ProfileAuthoring})
	require.NoError(t, err)

	var files []string
	for _, f := range findings {
		files = append(files, f.File)
	}
	assert.Contains(t, files, "docs/decisions/x.md")
	for _, f := range files {
		assert.NotContains(t, f, "internal/", "non-docs top-level dir must not be linted")
		assert.NotContains(t, f, "cmd/", "non-docs top-level dir must not be linted")
		assert.NotContains(t, f, "schemas/", "non-docs top-level dir must not be linted")
	}
}

func TestPlanMigration_ComputesDiffWithoutWriting(t *testing.T) {
	t.Parallel()
	root := newTree(t)
	beforeBad := readDoc(t, root, "docs/reviews/bad.md")
	beforeNone := readDoc(t, root, "docs/research/none.md")

	plan, err := PlanMigration(Options{Root: root, Now: svcNow})
	require.NoError(t, err)
	require.NotEmpty(t, plan.Changes)

	byFile := map[string]Change{}
	for _, c := range plan.Changes {
		byFile[c.File] = c
		assert.Falsef(t, c.BodyBytesChanged, "%s: body bytes must not change", c.File)
	}

	// The already-normalized file is a noop.
	assert.Equal(t, ActionNoop, byFile["docs/decisions/good.md"].Action)
	// The file with no frontmatter is an insert.
	assert.Equal(t, ActionInsert, byFile["docs/research/none.md"].Action)
	// The under-specified file is an update.
	assert.Equal(t, ActionUpdate, byFile["docs/reviews/bad.md"].Action)

	// Nothing was written.
	assert.Equal(t, beforeBad, readDoc(t, root, "docs/reviews/bad.md"))
	assert.Equal(t, beforeNone, readDoc(t, root, "docs/research/none.md"))
}

func TestApplyMigration_WritesAtomicallyAndIsIdempotent(t *testing.T) {
	t.Parallel()
	root := newTree(t)

	plan, err := PlanMigration(Options{Root: root, Now: svcNow})
	require.NoError(t, err)
	res, err := ApplyMigration(plan, Options{Root: root, Now: svcNow})
	require.NoError(t, err)
	assert.Contains(t, res.Applied, "docs/reviews/bad.md")
	assert.Contains(t, res.Applied, "docs/research/none.md")
	assert.Contains(t, res.Skipped, "docs/decisions/good.md")

	// Re-planning after apply yields all-noop (idempotent).
	plan2, err := PlanMigration(Options{Root: root, Now: svcNow})
	require.NoError(t, err)
	for _, c := range plan2.Changes {
		assert.Equalf(t, ActionNoop, c.Action, "%s should be noop after migration", c.File)
	}

	// The migrated file now passes the authoring profile.
	findings, err := LintTree(Options{Root: root, Profile: ProfileAuthoring})
	require.NoError(t, err)
	for _, f := range findings {
		assert.NotEqualf(t, "source", f.Field, "migrated tree should not be missing source: %s", f.File)
	}
}

func TestApplyMigration_PreservesBodyBytes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	body := "Para one.\r\n\r\n---\r\n\r\nAfter HR.\r\n"
	writeDoc(t, root, "docs/cli-reference/x.md", "---\ntitle: Doc\n---\n"+body)

	plan, err := PlanMigration(Options{Root: root, Now: svcNow})
	require.NoError(t, err)
	_, err = ApplyMigration(plan, Options{Root: root, Now: svcNow})
	require.NoError(t, err)

	got := readDoc(t, root, "docs/cli-reference/x.md")
	md, err := Decode([]byte(got))
	require.NoError(t, err)
	assert.Equal(t, body, string(md.Body), "body bytes preserved verbatim through apply")
}

func TestService_RejectsPathEscape(t *testing.T) {
	t.Parallel()
	root := newTree(t)

	_, err := LintTree(Options{Root: root, Path: "../escape", Profile: ProfileAuthoring})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPathEscapesWorkspace)

	_, err = PlanMigration(Options{Root: root, Path: "../../etc", Now: svcNow})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPathEscapesWorkspace)
}

func TestApplyMigration_RejectsBodyMutation(t *testing.T) {
	t.Parallel()
	root := newTree(t)
	// A hand-crafted plan whose change claims a body mutation must be refused
	// with zero writes (defensive invariant).
	target := "docs/reviews/bad.md"
	before := readDoc(t, root, target)
	plan := MigrationPlan{Changes: []Change{{
		File:             target,
		Action:           ActionUpdate,
		Before:           before,
		After:            before + "tampered",
		BodyBytesChanged: true,
	}}}
	_, err := ApplyMigration(plan, Options{Root: root})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBodyMutated)
	assert.Equal(t, before, readDoc(t, root, target), "no write on body-mutation refusal")
}

func TestApplyMigration_PreflightAbortsBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	root := newTree(t)

	good := "docs/reviews/bad.md"
	bad := "docs/research/none.md"
	goodBefore := readDoc(t, root, good)
	badBefore := readDoc(t, root, bad)

	// A valid change ordered before an invalid (body-mutating) change. The
	// preflight pass must reject the whole plan with zero writes, so the valid
	// earlier file stays untouched (all-or-nothing apply).
	plan := MigrationPlan{Changes: []Change{
		{File: good, Action: ActionUpdate, Before: goodBefore, After: goodBefore + "\nappended.\n"},
		{File: bad, Action: ActionUpdate, Before: badBefore, After: badBefore + "tampered", BodyBytesChanged: true},
	}}

	res, err := ApplyMigration(plan, Options{Root: root})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrBodyMutated)
	assert.Empty(t, res.Applied, "no file may be applied when a later change is invalid")
	assert.Equal(t, goodBefore, readDoc(t, root, good), "earlier valid file must not be written")
	assert.Equal(t, badBefore, readDoc(t, root, bad), "invalid file must not be written")
}

func TestValidateApplyPath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cases := []struct {
		name    string
		path    string
		wantErr error
	}{
		{"empty is whole-tree", "", ErrWholeTreeApply},
		{"dot resolves to root", ".", ErrWholeTreeApply},
		{"dotdot back to root", "docs/..", ErrWholeTreeApply},
		{"escape outside root", "../escape", ErrPathEscapesWorkspace},
		{"scoped sub-path allowed", "docs/decisions", nil},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateApplyPath(root, tc.path)
			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestAtomicWrite_PreservesExistingFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not represented on Windows filesystems")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	require.NoError(t, os.WriteFile(path, []byte("orig"), 0o644))

	require.NoError(t, atomicWrite(path, []byte("updated")))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "in-place rewrite preserves original mode, not the 0600 temp default")
	assert.Equal(t, "updated", readDoc(t, dir, "doc.md"))
}

func TestAtomicWrite_NewFileDefaultsTo0644(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not represented on Windows filesystems")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "new.md")

	require.NoError(t, atomicWrite(path, []byte("data")))

	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(), "new file gets the 0644 default, not 0600")
}
