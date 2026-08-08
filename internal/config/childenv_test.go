package config

import (
	"strings"
	"testing"
)

// TestChildProcessEnv_StripsGateEvidenceKey verifies that ChildProcessEnv never
// includes BACKLOGIT_GATE_EVIDENCE_KEY, while preserving other variables.
func TestChildProcessEnv_StripsGateEvidenceKey(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "super-secret-key-material")
	t.Setenv("BACKLOGIT_CHILDENV_CANARY", "still-present")

	env := ChildProcessEnv()

	for _, entry := range env {
		if strings.HasPrefix(entry, "BACKLOGIT_GATE_EVIDENCE_KEY=") {
			t.Fatalf("ChildProcessEnv() leaked the gate evidence key: %q", entry)
		}
		if strings.Contains(entry, "super-secret-key-material") {
			t.Fatalf("ChildProcessEnv() leaked the key value in an unexpected entry: %q", entry)
		}
	}

	found := false
	for _, entry := range env {
		if entry == "BACKLOGIT_CHILDENV_CANARY=still-present" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("ChildProcessEnv() unexpectedly stripped an unrelated variable")
	}
}

// TestChildProcessEnv_AbsentKeyIsNoop verifies ChildProcessEnv behaves normally
// when the key was never set.
func TestChildProcessEnv_AbsentKeyIsNoop(t *testing.T) {
	unsetEnv(t, "BACKLOGIT_GATE_EVIDENCE_KEY")
	env := ChildProcessEnv()
	for _, entry := range env {
		if strings.HasPrefix(entry, "BACKLOGIT_GATE_EVIDENCE_KEY=") {
			t.Fatalf("ChildProcessEnv() should not include the key entry at all: %q", entry)
		}
	}
}

// TestScrubGateEvidenceKeyEnv_PureFunction verifies the underlying pure
// scrubbing function directly, independent of the live process environment.
func TestScrubGateEvidenceKeyEnv_PureFunction(t *testing.T) {
	in := []string{
		"PATH=/usr/bin",
		"BACKLOGIT_GATE_EVIDENCE_KEY=leaked-if-present",
		"HOME=/home/test",
	}
	out := ScrubGateEvidenceKeyEnv(in)
	if len(out) != 2 {
		t.Fatalf("ScrubGateEvidenceKeyEnv() len = %d, want 2: %v", len(out), out)
	}
	for _, entry := range out {
		if strings.HasPrefix(entry, "BACKLOGIT_GATE_EVIDENCE_KEY=") {
			t.Fatalf("ScrubGateEvidenceKeyEnv() leaked the key: %v", out)
		}
	}
}

