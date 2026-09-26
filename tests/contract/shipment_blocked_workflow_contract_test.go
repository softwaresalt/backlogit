package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlockedShipmentInstalledWorkflowContract(t *testing.T) {
	registryPath := findRegistry(t)
	root := filepath.Dir(filepath.Dir(registryPath))

	instructions := normalizeContractText(readContractFile(t, filepath.Join(
		root, ".github", "instructions", "backlogit.instructions.md",
	)))
	assert.Contains(t, instructions,
		"`blocked` is a canonical governed resumable nonterminal shipment status")
	assert.NotContains(t, instructions,
		"there is no separate shipment `blocked` status")
	assert.NotContains(t, instructions,
		"only ever holds `queued`, `active`, `shipped`, or `abandoned`")

	skill := normalizeContractText(readContractFile(t, filepath.Join(
		root, ".github", "skills", "shipment-reconcile", "SKILL.md",
	)))
	assert.Contains(t, skill, "`record-blocked-resumable`")
	assert.Contains(t, skill, "unexpected for the requested reconciliation phase")
	assert.Contains(t, skill, "canonical blocked envelope")
	assert.NotContains(t, skill,
		"**`blocked` itself is a non-standard/legacy value**")
	assert.NotContains(t, skill,
		"backlogit 1.8.0 has NO `blocked` shipment status")
}

func readContractFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func normalizeContractText(value string) string {
	return strings.Join(strings.Fields(value), " ")
}
