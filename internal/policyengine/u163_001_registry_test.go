package policyengine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

const canonicalRegistryYAML = `schema_version: 1
mode: report-only
nodes:
  - {id: harness-scaffold, enabled: true, overridable: true}
  - {id: red-phase-verify, enabled: true, overridable: true}
  - {id: green-phase-verify, enabled: true, overridable: false}
  - {id: review-complete, enabled: true, overridable: false}
  - {id: ci-pass, enabled: true, overridable: false}
  - {id: merge-ready, enabled: true, overridable: false}
edges:
  - {node: red-phase-verify, depends_on: harness-scaffold}
  - {node: green-phase-verify, depends_on: red-phase-verify}
  - {node: review-complete, depends_on: green-phase-verify}
  - {node: ci-pass, depends_on: review-complete}
  - {node: merge-ready, depends_on: ci-pass}
`

const canonicalRegistryJSON = `{"schema_version":1,"mode":"enforce","nodes":[
{"id":"harness-scaffold","enabled":true,"overridable":true},
{"id":"red-phase-verify","enabled":true,"overridable":true},
{"id":"green-phase-verify","enabled":true,"overridable":false},
{"id":"review-complete","enabled":true,"overridable":false},
{"id":"ci-pass","enabled":true,"overridable":false},
{"id":"merge-ready","enabled":true,"overridable":false}],
"edges":[
{"node":"red-phase-verify","depends_on":"harness-scaffold"},
{"node":"green-phase-verify","depends_on":"red-phase-verify"},
{"node":"review-complete","depends_on":"green-phase-verify"},
{"node":"ci-pass","depends_on":"review-complete"},
{"node":"merge-ready","depends_on":"ci-pass"}]}`

func TestU163_001_NodeTypesAreTheSixPolicyGatesInOrder(t *testing.T) {
	got := NodeTypes()
	want := []NodeType{
		NodeHarnessScaffold, NodeRedPhaseVerify, NodeGreenPhaseVerify,
		NodeReviewComplete, NodeCIPass, NodeMergeReady,
	}
	assert.Equal(t, want, got)
	got[0] = "mutated"
	assert.Equal(t, NodeHarnessScaffold, NodeTypes()[0], "NodeTypes must return a copy")
}

func TestU163_001_LoadsCanonicalRegistryFromYAMLAndJSON(t *testing.T) {
	y, err := LoadRegistry([]byte(canonicalRegistryYAML))
	require.NoError(t, err)
	assert.Equal(t, evidencedag.ModeReportOnly, y.Mode())
	require.Len(t, y.Gates(), 6)
	assert.Equal(t, NodeHarnessScaffold, y.Gates()[0].ID)
	g, ok := y.Gate(NodeRedPhaseVerify)
	require.True(t, ok)
	assert.True(t, g.Enabled)
	assert.True(t, g.Overridable)
	g, ok = y.Gate(NodeCIPass)
	require.True(t, ok)
	assert.False(t, g.Overridable)
	_, ok = y.Gate("nope")
	assert.False(t, ok)

	j, err := LoadRegistry([]byte(canonicalRegistryJSON))
	require.NoError(t, err)
	assert.Equal(t, evidencedag.ModeEnforce, j.Mode())
}

func TestU163_001_GatesReturnsACopy(t *testing.T) {
	r, err := LoadRegistry([]byte(canonicalRegistryYAML))
	require.NoError(t, err)
	gs := r.Gates()
	gs[0].Enabled = false
	assert.True(t, r.Gates()[0].Enabled)
}

func TestU163_001_GraphLoadsThroughTheSharedCore(t *testing.T) {
	r, err := LoadRegistry([]byte(canonicalRegistryYAML))
	require.NoError(t, err)
	d, err := r.Declaration("163.001-T")
	require.NoError(t, err)
	require.NoError(t, d.Validate())
	_, err = evidencedag.NewEvaluator(d)
	require.NoError(t, err)

	require.Len(t, d.Nodes, 6)
	terminals := 0
	for _, n := range d.Nodes {
		assert.Equal(t, evidencedag.FamilyWorkflowPolicy, n.Family)
		assert.Equal(t, evidencedag.OnUnmetBlock, n.OnUnmet)
		assert.Equal(t, "163.001-T", n.ProducingTask)
		if n.Terminal {
			terminals++
			assert.Equal(t, string(NodeMergeReady), n.ID)
		}
	}
	assert.Equal(t, 1, terminals)
	assert.Len(t, d.Edges, 5)
}

func TestU163_001_DeclarationRejectsBadTaskIDs(t *testing.T) {
	r, err := LoadRegistry([]byte(canonicalRegistryYAML))
	require.NoError(t, err)
	for _, id := range []string{"", "x", "163.001", "../163.001-T", strings.Repeat("1", 200) + "-T"} {
		_, err := r.Declaration(id)
		assert.Error(t, err, id)
	}
}

func TestU163_001_DisabledGateBecomesNotApplicableThroughApplicability(t *testing.T) {
	doc := strings.Replace(canonicalRegistryYAML, "{id: green-phase-verify, enabled: true", "{id: green-phase-verify, enabled: false", 1)
	r, err := LoadRegistry([]byte(doc))
	require.NoError(t, err)
	g, _ := r.Gate(NodeGreenPhaseVerify)
	assert.False(t, g.Enabled)
	d, err := r.Declaration("163.001-T")
	require.NoError(t, err)
	for _, n := range d.Nodes {
		require.NotNil(t, n.Applicability)
		assert.False(t, n.Applicability.Always, "applicability must key off the per-gate attribute")
		assert.Contains(t, n.Applicability.Requires, GateAttribute(NodeType(n.ID)))
	}
}

func TestU163_001_RejectsAmbiguousDeclarations(t *testing.T) {
	rep := func(old, new string) string { return strings.Replace(canonicalRegistryYAML, old, new, 1) }
	cases := map[string]string{
		"empty":                "",
		"whitespace":           "   \n",
		"empty graph":          "schema_version: 1\nmode: enforce\nnodes: []\nedges: []\n",
		"nil graph":            "schema_version: 1\nmode: enforce\n",
		"unsupported version":  rep("schema_version: 1", "schema_version: 2"),
		"missing version":      rep("schema_version: 1\n", ""),
		"missing mode":         rep("mode: report-only\n", ""),
		"unknown mode":         rep("mode: report-only", "mode: advisory"),
		"duplicate node id":    rep("  - {id: ci-pass, enabled: true, overridable: false}\n", "  - {id: ci-pass, enabled: true, overridable: false}\n  - {id: ci-pass, enabled: true, overridable: false}\n"),
		"unknown node type":    rep("{id: ci-pass,", "{id: deploy-prod,"),
		"uppercase node type":  rep("{id: ci-pass,", "{id: CI-Pass,"),
		"missing node":         rep("  - {id: ci-pass, enabled: true, overridable: false}\n", ""),
		"missing enabled":      rep("{id: ci-pass, enabled: true, overridable: false}", "{id: ci-pass, overridable: false}"),
		"missing overridable":  rep("{id: ci-pass, enabled: true, overridable: false}", "{id: ci-pass, enabled: true}"),
		"merge-ready disabled": rep("{id: merge-ready, enabled: true", "{id: merge-ready, enabled: false"),
		"dangling edge":        rep("depends_on: ci-pass}", "depends_on: ghost}"),
		"self edge":            rep("{node: ci-pass, depends_on: review-complete}", "{node: ci-pass, depends_on: ci-pass}"),
		"cyclic edge":          canonicalRegistryYAML + "  - {node: harness-scaffold, depends_on: merge-ready}\n",
		"duplicate edge":       canonicalRegistryYAML + "  - {node: ci-pass, depends_on: review-complete}\n",
		"missing ordering":     rep("  - {node: red-phase-verify, depends_on: harness-scaffold}\n", ""),
		"reversed ordering":    rep("{node: red-phase-verify, depends_on: harness-scaffold}", "{node: harness-scaffold, depends_on: red-phase-verify}"),
		"unknown field":        rep("mode: report-only", "mode: report-only\nextra: 1"),
		"unknown node field":   rep("{id: ci-pass, enabled: true,", "{id: ci-pass, owner: x, enabled: true,"),
		"yaml anchor":          rep("nodes:\n", "nodes: &n\n"),
		"two documents":        canonicalRegistryYAML + "---\nschema_version: 1\n",
		"json duplicate key":   `{"schema_version":1,"schema_version":1,"mode":"enforce","nodes":[],"edges":[]}`,
		"json fold duplicate":  `{"schema_version":1,"mode":"enforce","MODE":"enforce","nodes":[],"edges":[]}`,
		"json trailing data":   canonicalRegistryJSON + `{}`,
		"oversized":            "schema_version: 1\n# " + strings.Repeat("a", evidencedag.MaxDeclarationBytes) + "\n",
		"edge flood":           canonicalRegistryYAML + strings.Repeat("  - {node: ci-pass, depends_on: review-complete}\n", 100),
		"node flood":           rep("nodes:\n", "nodes:\n"+strings.Repeat("  - {id: ci-pass, enabled: true, overridable: false}\n", 100)),
		"null enabled":         rep("{id: ci-pass, enabled: true,", "{id: ci-pass, enabled: null,"),
		"nodes wrong type":     "schema_version: 1\nmode: enforce\nnodes: nope\nedges: []\n",
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			r, err := LoadRegistry([]byte(doc))
			require.Error(t, err)
			assert.Nil(t, r)
			assert.ErrorIs(t, err, ErrInvalidRegistry)
			assert.LessOrEqual(t, len(err.Error()), 4096, "error text must be bounded")
		})
	}
}

func TestU163_001_ErrorTextIsBoundedForHostileIDs(t *testing.T) {
	doc := strings.Replace(canonicalRegistryYAML, "{id: ci-pass,", "{id: \""+strings.Repeat("x", 100000)+"\",", 1)
	_, err := LoadRegistry([]byte(doc))
	require.Error(t, err)
	assert.LessOrEqual(t, len(err.Error()), 4096)
}

func TestU163_001_LoadRegistryNeverPanicsOnHostileInput(t *testing.T) {
	for _, doc := range []string{"\x00", "{", "[", "nodes:\n  - [", "{\"nodes\":[null]}", "nodes:\n  - null\n", "\xff\xfe", "&a [*a]"} {
		assert.NotPanics(t, func() {
			r, err := LoadRegistry([]byte(doc))
			assert.Nil(t, r)
			assert.Error(t, err)
		}, doc)
	}
}

func TestU163_001_CheckedInRegistryLoads(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "policies", "workflow-policy-registry.yaml"))
	require.NoError(t, err)
	r, err := LoadRegistry(data)
	require.NoError(t, err)
	assert.Equal(t, evidencedag.ModeReportOnly, r.Mode(), "the shipped registry rolls out report-only first")
	for _, g := range r.Gates() {
		assert.True(t, g.Enabled, g.ID)
	}
}

// TestU163_001_NoDuplicateGraphTraversal guards the reuse boundary: the policy
// engine must consume the shared evidencedag core and never grow its own
// traversal, cycle, reachability or roll-up code.
func TestU163_001_NoDuplicateGraphTraversal(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)topo|cycle|reach|traverse|dfs|bfs|transitive|ancestors`)
	fset := token.NewFileSet()
	matches, err := filepath.Glob("*.go")
	require.NoError(t, err)
	importsCore := false
	for _, f := range matches {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		full, err := parser.ParseFile(fset, f, nil, parser.SkipObjectResolution)
		require.NoError(t, err)
		for _, imp := range full.Imports {
			if imp.Path.Value == `"github.com/softwaresalt/backlogit/internal/evidencedag"` {
				importsCore = true
			}
		}
		for _, d := range full.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && forbidden.MatchString(fn.Name.Name) {
				t.Errorf("%s declares %s: graph traversal belongs to internal/evidencedag", f, fn.Name.Name)
			}
		}
	}
	assert.True(t, importsCore, "policyengine must import internal/evidencedag")
}
