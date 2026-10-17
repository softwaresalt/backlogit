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
	consumer.Dependencies = []string{"1-T"}
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

func TestCopilotR2_ConfinedArtifactIn_RejectsSymlinkEscape(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}
	require.NoError(t, os.MkdirAll(filepath.Join(root, "ok", "sub"), 0o750))

	_, err := confinedArtifactIn(root, "link/new.go")
	require.Error(t, err)
	_, err = confinedArtifactIn(root, "ok/sub/missing/new.go")
	require.NoError(t, err)
	_, err = confinedArtifactIn("", "link/new.go")
	require.NoError(t, err)
}

func TestCopilotR2_MissingArtifact_SymlinkEscapingProducerFlagged(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink creation unsupported: %v", err)
	}
	cfg := DefaultConfig()
	cfg.Root = root
	producer, consumer := soundUnit("1-T", "Make"), soundUnit("2-T", "Use")
	producer.Deliverables = []string{"link/new.go"}
	consumer.Requires = []string{"link/new.go"}
	consumer.Dependencies = []string{"1-T"}
	got := rulesOf(runPlan(t, Plan{Units: []Unit{producer, consumer}}, cfg))["missing-artifact"]
	assert.NotEmpty(t, got)
}
