package evidencedag

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/faultline"
	"github.com/softwaresalt/backlogit/internal/redhonesty"
)

func TestU162_002_AuthenticityBindingRejections(t *testing.T) {
	d := decl([]Node{pnode("t", true)})
	good := goodRec(t, "t")
	require.Equal(t, StatusVerified, statusOf(t, evalOne(t, d, map[string]EvidenceRecord{"t": good}), "t"))

	otherShip := ship()
	otherShip.ID = "145-S"
	otherShipRec, err := NewEvidenceRecord(otherShip, "t", parityArtifact(testTaskA, testHead, faultline.StatusPass))
	require.NoError(t, err)
	otherHeadShip := ship()
	otherHeadShip.HeadCommit = otherHead
	otherHeadRec, err := NewEvidenceRecord(otherHeadShip, "t", parityArtifact(testTaskA, otherHead, faultline.StatusPass))
	require.NoError(t, err)

	tamperedRec := good
	tamperedRec.Artifact = json.RawMessage(strings.Replace(string(good.Artifact), `"scenario_id":"t"`, `"scenario_id":"s2"`, 1))
	require.NotEqual(t, string(good.Artifact), string(tamperedRec.Artifact), "the artifact really was altered")
	badVersion := good
	badVersion.SchemaVersion = 9
	noDigest := good
	noDigest.Digest = ""
	otherNodeRec := good
	otherNodeRec.NodeID = "other"

	wrongTask := rec(t, "t", parityArtifact(testTaskB, testHead, faultline.StatusPass))
	wrongCommit := rec(t, "t", parityArtifact(testTaskA, otherHead, faultline.StatusPass))
	wrongValidatorArt := parityArtifact(testTaskA, testHead, faultline.StatusPass)
	wrongValidatorArt.ValidatorIdentity = "evil/forger@v1"
	wrongValidator := rec(t, "t", wrongValidatorArt)

	cases := []struct {
		name string
		rec  EvidenceRecord
		want string
	}{
		{"other node", otherNodeRec, "different shipment, node or head commit"},
		{"other shipment", otherShipRec, "different shipment, node or head commit"},
		{"other head commit", otherHeadRec, "different shipment, node or head commit"},
		{"tampered artifact", tamperedRec, "digest does not match"},
		{"bad record version", badVersion, "unsupported evidence record version"},
		{"missing digest", noDigest, "digest does not match"},
		{"wrong producing task", wrongTask, "producing task does not match"},
		{"wrong producing commit", wrongCommit, "producing commit does not match"},
		{"wrong validator", wrongValidator, "validator identity does not match"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep := evalOne(t, d, map[string]EvidenceRecord{"t": c.rec})
			n, _ := rep.Node("t")
			assert.Equal(t, StatusError, n.Status)
			assert.Contains(t, n.Reason, c.want)
			assert.NotEqual(t, RollupReady, rep.Rollup().State)
		})
	}
}

func TestU162_002_LoadEvidenceFile(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.MkdirAll(outside, 0o755))
	good := goodRec(t, "t")
	writeRecord(t, filepath.Join(root, "evidence", "144-S", "t.json"), good)
	// Real targets exist for every traversal form, so only validation can refuse them.
	writeRecord(t, filepath.Join(base, "x.json"), good)
	writeRecord(t, filepath.Join(outside, "x.json"), good)
	writeRecord(t, filepath.Join(root, ".hidden", "t.json"), good)
	writeRecord(t, filepath.Join(root, "_under", "t.json"), good)
	writeRecord(t, filepath.Join(root, "evidence", ".t.json"), good)

	hugeJSON := `{"schema_version":1,"shipment_id":"` + strings.Repeat("a", maxRecordBytes+1000) + `"}`
	require.True(t, json.Valid([]byte(hugeJSON)), "the oversize payload is valid JSON, so only the size cap can reject it")
	smallJSON := `{"schema_version":1,"shipment_id":"` + strings.Repeat("a", 100) + `"}`
	goodJSON, err := json.Marshal(good)
	require.NoError(t, err)
	files := map[string]string{
		"big.json":   hugeJSON,
		"small.json": smallJSON,
		"junk.json":  `{"nope":1}`,
		"dup.json":   `{"schema_version":1,"schema_version":1}`,
		"fold.json":  `{"schema_version":1,"Schema_Version":1}`,
		"trail.json": string(goodJSON) + ` {}`,
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	require.NoError(t, os.Mkdir(filepath.Join(root, "adir"), 0o755))

	got, err := LoadEvidenceFile(root, "evidence/144-S/t.json")
	require.NoError(t, err)
	assert.Equal(t, good.Digest, got.Digest)
	got, err = LoadEvidenceFile(root, `evidence\144-S\t.json`)
	require.NoError(t, err)
	assert.Equal(t, good.Digest, got.Digest)
	small, err := LoadEvidenceFile(root, "small.json")
	require.NoError(t, err, "a small valid payload loads, so the oversize refusal is the size cap")
	assert.Len(t, small.ShipmentID, 100)

	driveWant := "must be relative"
	if runtime.GOOS != "windows" {
		driveWant = "component is not allowed"
	}
	cases := []struct{ name, rel, want string }{
		{"dotdot", "../x.json", "must not contain . or .."},
		{"embedded dotdot", "evidence/../../x.json", "must not contain . or .."},
		{"absolute", filepath.Join(outside, "x.json"), "must be relative"},
		{"rooted slash", "/x.json", "must be relative"},
		{"backslash root", `\x.json`, "must be relative"},
		{"drive", `C:x.json`, driveWant},
		{"hidden dir", ".hidden/t.json", "hidden or underscore-prefixed"},
		{"underscore dir", "_under/t.json", "hidden or underscore-prefixed"},
		{"hidden file", "evidence/.t.json", "hidden or underscore-prefixed"},
		{"empty", "", "empty, too long or contains NUL"},
		{"nul", "evidence/t\x00.json", "empty, too long or contains NUL"},
		{"stream", "evidence/144-S/t.json:stream", "component is not allowed"},
		{"control character", "evidence/t\x01.json", "component is not allowed"},
		{"reserved CON with extension", "evidence/CON.json", "reserved device name"},
		{"reserved aux", "evidence/aux", "reserved device name"},
		{"reserved com1", "evidence/com1.txt", "reserved device name"},
		{"reserved lpt9", "evidence/LPT9", "reserved device name"},
		{"reserved nul with spaces", "evidence/nul .json", "reserved device name"},
		{"trailing dot", "evidence/144-S/t.json.", "end with a dot or space"},
		{"trailing space", "evidence/144-S/t.json ", "end with a dot or space"},
		{"short name", "EVIDEN~1/144-S/t.json", "8.3 short name"},
		{"mixed separators", `evidence/\144-S/t.json`, "empty components"},
		{"mixed separators reversed", `evidence\/144-S/t.json`, "empty components"},
		{"double slash", "evidence//144-S/t.json", "empty components"},
		{"trailing separator", "evidence/144-S/", "empty components"},
		{"directory", "adir", "not a regular file"},
		{"oversize", "big.json", "exceeds"},
		{"unknown field", "junk.json", "unknown field"},
		{"duplicate key", "dup.json", "duplicate key"},
		{"case-folded duplicate key", "fold.json", "duplicate key"},
		{"trailing data", "trail.json", "trailing data"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := LoadEvidenceFile(root, c.rel)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
	t.Run("missing", func(t *testing.T) {
		_, err := LoadEvidenceFile(root, "nothing.json")
		require.Error(t, err)
		assert.True(t, errors.Is(err, fs.ErrNotExist))
	})
	_, err = LoadEvidenceFile("", "evidence/144-S/t.json")
	assert.ErrorContains(t, err, "evidence root is required")
	_, err = LoadEvidenceFile(filepath.Join(root, "missing-root"), "x.json")
	assert.ErrorContains(t, err, "resolve evidence root")
}

func TestU162_002_LoadEvidenceFileRefusesDirectoryJunctions(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("SKIPPED: directory junctions (mklink /J) exist only on Windows; symlink coverage lives in the symlink test")
	}
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(root, 0o755))
	writeRecord(t, filepath.Join(outside, "real.json"), goodRec(t, "t"))
	out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "junc"), outside).CombinedOutput()
	if err != nil {
		t.Skipf("SKIPPED: could not create a junction: %v: %s", err, out)
	}
	_, err = LoadEvidenceFile(root, "junc/real.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink or reparse point")
}

func rgNodePinned(id string) Node {
	n := rgNode(id)
	n.ProducingCommit = otherHead
	return n
}

func TestRemediation_RedHonestyRejectsPinnedProducingCommit(t *testing.T) {
	d := decl([]Node{rgNodePinned("red")})
	chk := &fakeChecker{rep: rgReport(t, true)}
	x := &RedHonestyExecutor{Gate: chk, BaseRef: "origin/main"}
	res, err := RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{"red": x})
	require.NoError(t, err)
	assert.Empty(t, res.Evidence)
	assert.Contains(t, res.Failures["red"], "pinned producing_commit")
	assert.Equal(t, redhonesty.Request{}, chk.req, "the gate is never consulted for a pinned commit")
}

type panickyIdentity struct{}

func (panickyIdentity) Identity() string { panic("identity exploded") }
func (panickyIdentity) Execute(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
	return faultline.EvidenceArtifact{}, nil
}

func TestRemediation_ExecutorRunnerHardening(t *testing.T) {
	d := decl([]Node{pnode("a", true), pnode("b", true), pnode("c", true)})
	var typedNil *RedHonestyExecutor
	res, err := RunExecutors(context.Background(), d, ship(), map[string]NodeExecutor{
		"a": typedNil,
		"b": panickyIdentity{},
		"c": nil,
	})
	require.NoError(t, err)
	assert.Contains(t, res.Failures["a"], "nil")
	assert.Contains(t, res.Failures["b"], "panicked")
	assert.Contains(t, res.Failures["c"], "nil")
	assert.Empty(t, res.Evidence)

	var nilCtx context.Context
	_, err = RunExecutors(nilCtx, d, ship(), nil)
	assert.ErrorIs(t, err, ErrInvalidInput)

	// A vacuous run produces nothing, and the evaluator never reads that as satisfied.
	res, err = RunExecutors(context.Background(), d, ship(), nil)
	require.NoError(t, err)
	assert.Empty(t, res.Evidence)
	assert.Empty(t, res.Failures)
	rep := evalOne(t, d, res.Evidence)
	assert.Equal(t, RollupBlocked, rep.Rollup().State)
}

func TestRemediation_ExecutorTimeoutAndSanitizedErrors(t *testing.T) {
	d := decl([]Node{pnode("slow", true), pnode("ansi", true)})
	release := make(chan struct{})
	defer close(release)
	slow := fnExec(t, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
		<-release // ignores its context, like a misbehaving detector
		return faultline.EvidenceArtifact{}, nil
	})
	ansi := fnExec(t, func(context.Context, ExecRequest) (faultline.EvidenceArtifact, error) {
		return faultline.EvidenceArtifact{}, errors.New("\x1b[31mfake: all green\x1b[0m\n\u202eevil")
	})
	start := time.Now()
	res, err := RunExecutorsWith(context.Background(), d, ship(), map[string]NodeExecutor{"slow": slow, "ansi": ansi}, ExecOptions{NodeTimeout: 100 * time.Millisecond})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Contains(t, res.Failures["slow"], "timed out")
	reason := res.Failures["ansi"]
	assert.NotEmpty(t, reason)
	for _, r := range reason {
		assert.False(t, r < 0x20 || r == 0x7f || r == '\u202e', "control and bidi characters are stripped: %q", reason)
	}
}

func TestRemediation_DOTTruncationStillClosesTheGraph(t *testing.T) {
	d := &Declaration{SchemaVersion: SchemaVersion}
	long := strings.Repeat("n", 90)
	for i := 0; i < MaxNodes; i++ {
		d.Nodes = append(d.Nodes, Node{ID: long + string(rune('a'+i%26)) + strings.Repeat("x", i%7) + itoa(i), Terminal: i == 0})
	}
	for i := 0; i < MaxEdges; i++ {
		d.Edges = append(d.Edges, Edge{Node: d.Nodes[i%MaxNodes].ID, DependsOn: d.Nodes[(i*7+1)%MaxNodes].ID + itoa(i)})
	}
	out := d.DOT(nil)
	assert.LessOrEqual(t, len(out), MaxReportBytes)
	assert.True(t, strings.HasSuffix(out, "}\n"), "the graph is always closed")
	assert.Contains(t, out, "truncated")
	small := decl([]Node{pnode("t", true)}).DOT(nil)
	assert.True(t, strings.HasSuffix(small, "}\n"))
	assert.NotContains(t, small, "truncated")
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}

func TestRemediation_RenderedTextStripsControlCharacters(t *testing.T) {
	rep := Report{ShipmentID: "144-S\x1b[2J", HeadCommit: testHead, Nodes: []NodeResult{{ID: "t", Family: FamilyParity, Terminal: true, Status: StatusError, Reason: "\x1b[31mred\x07\u202e"}}}
	txt := rep.Text()
	for _, r := range txt {
		if r == '\n' || r == '\t' {
			continue
		}
		assert.False(t, r < 0x20 || r == 0x7f || r == '\u202e', "unexpected control character in %q", txt)
	}
}

func TestRemediation_ParserDifferentials(t *testing.T) {
	t.Run("case-insensitive duplicate JSON keys", func(t *testing.T) {
		_, err := Load([]byte(`{"schema_version":1,"Schema_Version":2,"nodes":[],"edges":[]}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate key")
	})
	t.Run("YAML anchors and aliases", func(t *testing.T) {
		doc := strings.Replace(validYAML, "applicability: {always: true}\n    validator: backlogit/parity@v1\n    producing_task: 162.001-T\n    producing_commit: head\n    on_unmet: block\n    terminal: false", "applicability: &a {always: true}\n    validator: backlogit/parity@v1\n    producing_task: 162.001-T\n    producing_commit: head\n    on_unmet: block\n    terminal: false", 1)
		require.NotEqual(t, validYAML, doc)
		_, err := Load([]byte(doc))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "anchors")
		alias := strings.Replace(validYAML, "applicability: {always: true}\n    validator: backlogit/parity@v1\n    producing_task: 162.001-T\n    producing_commit: head\n    on_unmet: block\n    terminal: true", "applicability: *a\n    validator: backlogit/parity@v1\n    producing_task: 162.001-T\n    producing_commit: head\n    on_unmet: block\n    terminal: true", 1)
		_, err = Load([]byte(alias))
		require.Error(t, err)
	})
	t.Run("YAML merge keys", func(t *testing.T) {
		_, err := Load([]byte("schema_version: 1\n<<: {nodes: []}\nnodes: []\nedges: []\n"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "merge")
	})
	t.Run("YAML case-differing duplicate keys", func(t *testing.T) {
		_, err := Load([]byte("schema_version: 1\nSchema_Version: 1\nnodes: []\nedges: []\n"))
		require.Error(t, err)
	})
	t.Run("plain YAML still loads", func(t *testing.T) {
		_, err := Load([]byte(validYAML))
		require.NoError(t, err)
	})
}
