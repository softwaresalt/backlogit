package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUCXS3_OrchestratorGateAwareEligibility(t *testing.T) {
	repoRoot := testRepoRoot(t)
	content, err := os.ReadFile(filepath.Join(repoRoot, ".github", "agents", "_orchestrator.agent.md"))
	require.NoError(t, err, "read Orchestrator agent definition")
	orchestrator := strings.Join(strings.Fields(string(content)), " ")

	// Step 0 is followed by Step 1 (Step 0.0b precedes Step 0 in the file).
	start, end := "### Step 0: State Assessment", "### Step 1: Route to Stage"
	require.Equal(t, 1, strings.Count(orchestrator, start), "Step 0 start anchor must be unique")
	require.Equal(t, 1, strings.Count(orchestrator, end), "Step 1 end anchor must be unique")
	step0 := orchestrator[strings.Index(orchestrator, start):strings.Index(orchestrator, end)]

	assert.Contains(t, step0, "autoharness gate pipeline-topology --mode agent --shipment {id} --phase pre_claim --json",
		"Orchestrator Step 0 must run the pre_claim pipeline-topology gate for each DAG-ready candidate")
	assert.Contains(t, step0, "gate verdict",
		"Orchestrator Step 0 eligibility reporting must show the gate verdict")
	assert.Contains(t, step0, "DAG readiness",
		"Orchestrator Step 0 eligibility reporting must show DAG readiness beside the gate verdict")
}
