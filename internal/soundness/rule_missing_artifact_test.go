package soundness

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestU160009_MissingArtifact_FlagsUnproducedRequirement(t *testing.T) {
	root := t.TempDir()
	cfg := DefaultConfig()
	cfg.Root = root
	u := soundUnit("1-T", "A")
	u.Requires = []string{"internal/nothing/here.go"}
	got := rulesOf(runPlan(t, Plan{Units: []Unit{u}}, cfg))["missing-artifact"]
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Message, "internal/nothing/here.go")
}

func TestU160009_MissingArtifact_ProducedOrExistingPasses(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "present.md"), []byte("x"), 0o600))
	cfg := DefaultConfig()
	cfg.Root = root
	producer, consumer := soundUnit("1-T", "Make"), soundUnit("2-T", "Use")
	producer.Deliverables = []string{"internal/x/new.go"}
	consumer.Requires = []string{"internal/x/new.go", "present.md"}
	assert.Empty(t, rulesOf(runPlan(t, Plan{Units: []Unit{producer, consumer}}, cfg))["missing-artifact"])
}

func TestU160009_MissingArtifact_NoRootFailsClosed(t *testing.T) {
	u := soundUnit("1-T", "A")
	u.Requires = []string{"present.md"}
	assert.Len(t, findingsFor(t, "missing-artifact", u), 1)
}

func TestU160009_MissingArtifact_OutOfRootFlagged(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Root = t.TempDir()
	u := soundUnit("1-T", "A")
	u.Requires = []string{"../outside.md"}
	assert.Len(t, rulesOf(runPlan(t, Plan{Units: []Unit{u}}, cfg))["missing-artifact"], 1)
}
