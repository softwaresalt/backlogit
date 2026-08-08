package core

import (
	"strings"
	"testing"
)

// TestGitCommandEnv_StripsGateEvidenceKey verifies that gitCommandEnv (used
// for every git child process this package spawns via the archive path) never
// forwards BACKLOGIT_GATE_EVIDENCE_KEY, closing the same leak path already
// covered for the gate broker's allowlisted gate.MinimalEnv (106-F F1/U2).
func TestGitCommandEnv_StripsGateEvidenceKey(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "leak-canary-value")

	env := gitCommandEnv()

	for _, entry := range env {
		if strings.HasPrefix(entry, "BACKLOGIT_GATE_EVIDENCE_KEY=") {
			t.Fatalf("gitCommandEnv() leaked the gate evidence key: %q", entry)
		}
		if strings.Contains(entry, "leak-canary-value") {
			t.Fatalf("gitCommandEnv() leaked the key value: %q", entry)
		}
	}
}

