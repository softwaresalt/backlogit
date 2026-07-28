package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
	"github.com/softwaresalt/backlogit/internal/config"
)

// TestReplaceFileWithOptions_RoutesDurableFlag is the F5 regression: archive and
// restore content writes must route the workspace durable_writes preference into
// the low-level atomic primitive instead of always taking the durable-off path.
//
// Must not run with t.Parallel: this test swaps the package-global
// replaceFileWriteFn seam read on the production write path.
func TestReplaceFileWithOptions_RoutesDurableFlag(t *testing.T) {
	cases := []struct {
		name string
		ws   *Workspace
		want bool
	}{
		{name: "durable workspace routes durable=true", ws: &Workspace{Config: &config.WorkspaceConfig{DurableWrites: true}}, want: true},
		{name: "non-durable workspace routes durable=false", ws: &Workspace{Config: &config.WorkspaceConfig{DurableWrites: false}}, want: false},
		{name: "nil workspace routes durable=false", ws: nil, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var captured bool
			orig := replaceFileWriteFn
			replaceFileWriteFn = func(path string, data []byte, opts atomicfile.Options) error {
				captured = opts.DurableWrites
				return atomicfile.WriteFileAtomicWithOptions(path, data, opts)
			}
			t.Cleanup(func() { replaceFileWriteFn = orig })

			path := filepath.Join(t.TempDir(), "record.md")
			require.NoError(t, replaceFileWithOptions(tc.ws, path, []byte("content\n")))

			assert.Equal(t, tc.want, captured, "durable flag must be routed from WorkspaceDurableWrites(ws)")
			got, err := os.ReadFile(path)
			require.NoError(t, err)
			assert.Equal(t, "content\n", string(got))
		})
	}
}
