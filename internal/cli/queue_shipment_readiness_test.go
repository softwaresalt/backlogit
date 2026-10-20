package cli_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeQueueReadinessFixture(t *testing.T, root, dir, id, extra string) {
	t.Helper()
	target := filepath.Join(root, ".backlogit", dir)
	require.NoError(t, os.MkdirAll(target, 0o755))
	body := "---\nid: " + id + "\nartifact_type: shipment\ntitle: Fixture " + id + "\n" + extra + "---\n\nfixture body\n"
	require.NoError(t, os.WriteFile(filepath.Join(target, id+".md"), []byte(body), 0o644))
}

// TestQueueShipmentReadiness_CLIViewSplitsOnArchivedProvenance verifies the
// queue view includes a shipment behind an archived shipped predecessor and
// excludes one behind an archived abandoned predecessor (A-U9 / 188.005-T).
func TestQueueShipmentReadiness_CLIViewSplitsOnArchivedProvenance(t *testing.T) {
	root := setupCLIWorkspace(t)
	writeQueueReadinessFixture(t, root, "archive", "950-S", "status: archived\narchived_status: shipped\n")
	writeQueueReadinessFixture(t, root, "archive", "951-S", "status: archived\narchived_status: abandoned\n")
	writeQueueReadinessFixture(t, root, "queue", "960-S", "status: queued\ndependencies:\n    - id: 950-S\n      type: blocks\n")
	writeQueueReadinessFixture(t, root, "queue", "961-S", "status: queued\ndependencies:\n    - id: 951-S\n      type: blocks\n")

	syncOutput, err := runRootCommand(t, "--cwd", root, "sync")
	require.NoError(t, err, syncOutput)

	output, err := runRootCommand(t, "--cwd", root, "queue", "view", "--type", "shipment", "--status", "queued", "--format", "json")
	require.NoError(t, err, output)

	var view queueViewResult
	require.NoError(t, json.Unmarshal([]byte(output), &view), output)
	ids := make([]string, 0, len(view.Items))
	for _, item := range view.Items {
		ids = append(ids, item.ID)
	}
	assert.Contains(t, ids, "960-S", "shipment behind archived shipped predecessor must be shown")
	assert.NotContains(t, ids, "961-S", "shipment behind archived abandoned predecessor must be hidden")
}

// TestQueueShipmentReadiness_SingleQueryQueueReference scans every non-test Go
// file under internal/ and cmd/ and requires exactly one QueryQueue identifier
// reference, inside QueryQueueForWorkspace in internal/core.
func TestQueueShipmentReadiness_SingleQueryQueueReference(t *testing.T) {
	type ref struct {
		dir  string
		fn   string
		file string
	}
	var refs []ref
	fset := token.NewFileSet()
	scanned := 0

	for _, base := range []string{filepath.Join("..", "..", "internal"), filepath.Join("..", "..", "cmd")} {
		require.NoError(t, filepath.WalkDir(base, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			scanned++
			for _, decl := range file.Decls {
				fd, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				ast.Inspect(fd, func(n ast.Node) bool {
					id, ok := n.(*ast.Ident)
					if ok && id.Name == "QueryQueue" && id != fd.Name {
						refs = append(refs, ref{dir: filepath.ToSlash(filepath.Dir(path)), fn: fd.Name.Name, file: filepath.Base(path)})
					}
					return true
				})
			}
			return nil
		}))
	}

	require.Positive(t, scanned, "scan must cover non-test packages")
	require.Len(t, refs, 1, "adapters must call QueryQueueForWorkspace; QueryQueue references: %+v", refs)
	assert.Equal(t, "QueryQueueForWorkspace", refs[0].fn)
	assert.True(t, strings.HasSuffix(refs[0].dir, "internal/core"), "reference must live in internal/core, got %s", refs[0].dir)
}
