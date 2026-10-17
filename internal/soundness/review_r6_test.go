package soundness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopilotR6_References_RejectRawDotDot(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "sub"), 0o750))
	// In-root decoy that the cleaned path would wrongly resolve to.
	require.NoError(t, os.WriteFile(filepath.Join(root, "artifact.md"), []byte("package x\n\nfunc Symbol() {}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.go"), []byte("package x\n\nfunc Symbol() {}\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "a"), 0o750))
	if err := os.Symlink(filepath.Join(outside, "sub"), filepath.Join(root, "link")); err != nil {
		t.Logf("symlink creation unsupported, using pure-string cases only: %v", err)
	}

	cfg := DefaultConfig()
	cfg.Root = root
	refs := []string{
		"link/../artifact.md",
		"link/../artifact.md:1",
		"link/../artifact.md::Symbol",
		"link\\..\\artifact.md",
		"a/../b.go",
		"a/../b.go:1",
		"a/../b.go::Symbol",
		"a/ .. /b.go",
	}
	for _, ref := range refs {
		t.Run(ref, func(t *testing.T) {
			u := soundUnit("1-T", "A")
			u.References = []string{ref}
			got := rulesOf(runPlan(t, Plan{Units: []Unit{u}}, cfg))["references"]
			require.Len(t, got, 1)
			assert.Contains(t, got[0].Message, "escapes")
		})
	}

	ok := soundUnit("2-T", "B")
	ok.References = []string{"b.go", "b.go:1", "b.go::Symbol"}
	assert.Empty(t, rulesOf(runPlan(t, Plan{Units: []Unit{ok}}, cfg))["references"])
}
