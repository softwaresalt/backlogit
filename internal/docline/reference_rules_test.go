package docline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleGo = "package pkg\n\n// Widget builds a widget\n// from parts.\nfunc Widget() {}\n\ntype Gadget struct{}\n"

func TestU159006_ParseAnchor(t *testing.T) {
	tests := []struct {
		in      string
		want    Anchor
		wantErr bool
	}{
		{"internal/x/a.go:12", Anchor{Path: "internal/x/a.go", Line: 12}, false},
		{"internal/x/a.go:12-20", Anchor{Path: "internal/x/a.go", Line: 12, EndLine: 20}, false},
		{"internal/x/a.go::Widget", Anchor{Path: "internal/x/a.go", Symbol: "Widget"}, false},
		{"internal/x/a.go", Anchor{}, true},
		{"internal/x/a.go:0", Anchor{}, true},
		{"internal/x/a.go:9-3", Anchor{}, true},
		{"internal/x/a.go::", Anchor{}, true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseAnchor(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				assert.ErrorIs(t, err, ErrAnchorSyntax)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestU159006_ResolveAnchor(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "internal/pkg/a.go", sampleGo)

	r, err := ResolveAnchorString(root, "internal/pkg/a.go:5")
	require.NoError(t, err)
	assert.Equal(t, "internal/pkg/a.go", r.Rel)
	assert.Equal(t, filepath.Join(root, "internal", "pkg", "a.go"), r.Abs)

	_, err = ResolveAnchorString(root, "internal/pkg/a.go:5-7")
	require.NoError(t, err)
	_, err = ResolveAnchorString(root, "internal/pkg/a.go::Gadget")
	require.NoError(t, err)

	_, err = ResolveAnchorString(root, "internal/pkg/a.go:99")
	assert.ErrorIs(t, err, ErrAnchorLineOutOfRange)
	_, err = ResolveAnchorString(root, "internal/pkg/a.go::Nope")
	assert.ErrorIs(t, err, ErrAnchorSymbolNotFound)
	_, err = ResolveAnchorString(root, "internal/pkg/missing.go:1")
	assert.ErrorIs(t, err, ErrAnchorNotFound)
}

func TestU159006_ResolveAnchor_RejectsOutsideRoot(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "ws")
	require.NoError(t, os.MkdirAll(root, 0o755))
	writeDoc(t, parent, "outside/secret.go", sampleGo)
	abs := filepath.Join(parent, "outside", "secret.go")

	for _, in := range []string{
		"../outside/secret.go:1",
		"internal/../../outside/secret.go:1",
		abs + ":1",
		"/etc/passwd.go:1",
		`..\outside\secret.go:1`,
	} {
		t.Run(in, func(t *testing.T) {
			_, err := ResolveAnchorString(root, in)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrAnchorOutsideRoot)
		})
	}
}

func TestU159006_ResolveAnchor_RejectsSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "ws")
	require.NoError(t, os.MkdirAll(root, 0o755))
	writeDoc(t, parent, "outside/secret.go", sampleGo)
	if err := os.Symlink(filepath.Join(parent, "outside"), filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(parent, "outside", "secret.go"), filepath.Join(root, "file.go")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	for _, in := range []string{"link/secret.go:1", "file.go:1"} {
		_, err := ResolveAnchorString(root, in)
		require.Error(t, err, in)
		assert.ErrorIs(t, err, ErrAnchorOutsideRoot, in)
	}
}

func TestU159006_Anchors(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "internal/pkg/a.go", sampleGo)

	clean := "See `internal/pkg/a.go:5` and `internal/pkg/a.go::Widget`. Also `localhost:8080` and `x.go`.\n"
	assert.Empty(t, checkAnchors(root, "docs/x.md", clean))

	fenced := "```\n`internal/pkg/missing.go:1`\n```\n"
	assert.Empty(t, checkAnchors(root, "docs/x.md", fenced))

	got := checkAnchors(root, "docs/x.md", "Gone `internal/pkg/missing.go:1`, bad line `internal/pkg/a.go:99`, no sym `internal/pkg/a.go::Nope`.\n")
	assert.Equal(t, []string{RuleAnchorUnresolved, RuleAnchorUnresolved, RuleAnchorUnresolved}, rulesOf(got))

	got = checkAnchors(root, "docs/x.md", "Out `../x/y.go:3`, abs `/etc/passwd.go:1`.\n")
	assert.Equal(t, []string{RuleAnchorOutsideRoot, RuleAnchorOutsideRoot}, rulesOf(got))
	for _, f := range got {
		assert.Equal(t, SeverityWarn, f.Severity)
	}
}

func TestU159006_TaskIDs(t *testing.T) {
	exists := func(id string) bool { return id == "159.006-T" || id == "159-F" }

	assert.Empty(t, checkTaskIDs("docs/x.md", "Covers 159.006-T under 159-F.\n", exists))
	assert.Empty(t, checkTaskIDs("docs/x.md", "```\n999.999-T\n```\n", exists))

	got := checkTaskIDs("docs/x.md", "Covers 159.006-T and 777.001-T.\n", exists)
	require.Len(t, got, 1)
	assert.Equal(t, RuleTaskIDMissing, got[0].Rule)
	assert.Contains(t, got[0].Fix, "777.001-T")
}

func TestU159006_WorkspaceTaskIDs(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, ".backlogit/queue/159.006-T.md", "---\nid: 159.006-T\n---\nbody\n")
	writeDoc(t, root, ".backlogit/archive/old/001-F.md", "---\nid: 001-F\n---\n")
	writeDoc(t, root, ".backlogit/queue/.159.007-T.md.lock", "x")

	ids, present, err := workspaceTaskIDs(root)
	require.NoError(t, err)
	assert.True(t, ids["159.006-T"])
	assert.True(t, ids["001-F"])
	assert.False(t, ids["159.007-T"])

	empty, present2, err := workspaceTaskIDs(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, empty)
	assert.True(t, present)
	assert.False(t, present2)
}

func TestU159006_StaleTerms(t *testing.T) {
	terms := map[string]string{"Backlog Harvester": "Stage agent"}
	assert.Empty(t, checkStaleTerms("docs/x.md", "Use the Stage agent. Backlog Harvesters.\n", terms))
	assert.Empty(t, checkStaleTerms("docs/x.md", "```\nBacklog Harvester\n```\n", terms))
	assert.Empty(t, checkStaleTerms("docs/x.md", "Backlog Harvester\n", nil))

	got := checkStaleTerms("docs/x.md", "Ask the Backlog Harvester now.\n", terms)
	require.Len(t, got, 1)
	assert.Equal(t, RuleStaleTerm, got[0].Rule)
	assert.Contains(t, got[0].Fix, "Stage agent")
}

func TestU159006_CommentDrift(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "internal/pkg/a.go", sampleGo)

	clean := "<!-- docline:comment-ref path=\"internal/pkg/a.go\" symbol=\"Widget\" text=\"builds a widget from parts\" -->\n"
	assert.Empty(t, checkCommentDrift(root, "docs/x.md", clean))

	drift := "<!-- docline:comment-ref path=\"internal/pkg/a.go\" symbol=\"Widget\" text=\"destroys widgets\" -->\n"
	got := checkCommentDrift(root, "docs/x.md", drift)
	require.Len(t, got, 1)
	assert.Equal(t, RuleCommentDrift, got[0].Rule)

	missing := "<!-- docline:comment-ref path=\"internal/pkg/a.go\" symbol=\"Gone\" text=\"x\" -->\n"
	assert.Equal(t, []string{RuleCommentDrift}, rulesOf(checkCommentDrift(root, "docs/x.md", missing)))

	outside := "<!-- docline:comment-ref path=\"../a.go\" symbol=\"Widget\" text=\"x\" -->\n"
	assert.Equal(t, []string{RuleAnchorOutsideRoot}, rulesOf(checkCommentDrift(root, "docs/x.md", outside)))
}

func TestU159006_LintTreeOptIn(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, ".backlogit/queue/159-F.md", "---\nid: 159-F\n---\n")
	writeDoc(t, root, "docs/decisions/x.md", "---\ntitle: X\nsource: y\ndoc_type: decision\n---\n# A\n\nSee `missing.go:1`, 777.001-T, old Gizmo.\n")

	base, err := LintTree(Options{Root: root, Rules: RuleSet{Structural: true}})
	require.NoError(t, err)
	assert.Empty(t, base, "reference rules are opt-in")

	got, err := LintTree(Options{Root: root, Rules: RuleSet{
		References: true,
		StaleTerms: map[string]string{"Gizmo": "Widget"},
	}})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{RuleAnchorUnresolved, RuleTaskIDMissing, RuleStaleTerm}, rulesOf(got))

	rs, err := ParseRules([]string{"structural", "references"})
	require.NoError(t, err)
	assert.True(t, rs.Structural && rs.References)
}

func TestWorkspaceTaskIDs_FrontmatterIDsSlugsPrefixAndLarge(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, ".backlogit/queue/159.006-T-some-title.md", "---\nid: 159.006-T\ntitle: x\n---\n")
	writeDoc(t, root, ".backlogit/queue/T001.md", "---\nid: T001\n---\n")
	writeDoc(t, root, ".backlog/queue/1000-F-big.md", "---\nid: \"1000-F\"\n---\n")

	ids, present, err := workspaceTaskIDs(root)
	require.NoError(t, err)
	assert.True(t, present)
	for _, id := range []string{"159.006-T", "T001", "1000-F"} {
		assert.True(t, ids[id], id)
	}
}

func TestWorkspaceTaskIDs_MalformedFrontmatterReported(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, ".backlogit/queue/001-F.md", "---\ntitle: x\nno closing delimiter\n")
	_, _, err := workspaceTaskIDs(root)
	require.Error(t, err)
}

func TestCheckTaskIDs_SupportedFormats(t *testing.T) {
	exists := func(id string) bool { return id == "1000-F" || id == "T001" }
	assert.Empty(t, checkTaskIDs("d.md", "See 1000-F and T001.\n", exists))
	got := checkTaskIDs("d.md", "See 1001-F and T002.\n", func(id string) bool { return false })
	assert.Len(t, got, 2)
}

func TestLintTree_SluggedAndPrefixIDsResolve(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, ".backlogit/queue/001-F-slug.md", "---\nid: 001-F\n---\n")
	writeDoc(t, root, ".backlogit/queue/T001.md", "---\nid: T001\n---\n")
	writeDoc(t, root, "docs/decisions/x.md", "---\ntitle: X\nsource: y\ndoc_type: decision\n---\n# A\n\nSee 001-F, T001, T999 and SHA256.\n")
	got, err := LintTree(Options{Root: root, Rules: RuleSet{References: true}})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Fix, "T999")
}
