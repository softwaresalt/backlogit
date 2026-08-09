package gate

import (
	"strings"
	"testing"
)

// TestResolveChildEnv_NilFallsBackToScrubbedAmbient verifies that
// resolveChildEnv(nil) never includes BACKLOGIT_GATE_EVIDENCE_KEY while
// preserving other ambient variables — a real, environment-inspecting test
// (not merely a functional smoke test) so a future regression is actually
// caught (106-F F1 review finding F8).
func TestResolveChildEnv_NilFallsBackToScrubbedAmbient(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("GATE_CHILDENV_TEST_CANARY", "still-present")

	env := resolveChildEnv(nil)

	for _, e := range env {
		if strings.HasPrefix(e, "BACKLOGIT_GATE_EVIDENCE_KEY=") {
			t.Fatalf("resolveChildEnv(nil) leaked the gate evidence key: %q", e)
		}
	}
	found := false
	for _, e := range env {
		if e == "GATE_CHILDENV_TEST_CANARY=still-present" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("resolveChildEnv(nil) unexpectedly stripped an unrelated variable")
	}
}

// TestResolveChildEnv_ExplicitEnvPreservedUnchanged verifies an explicitly
// supplied env slice is returned exactly as given, with no scrubbing applied
// (the caller is responsible for what it explicitly sets).
func TestResolveChildEnv_ExplicitEnvPreservedUnchanged(t *testing.T) {
	explicit := []string{"FOO=bar", "BACKLOGIT_GATE_EVIDENCE_KEY=explicit-caller-choice"}
	got := resolveChildEnv(explicit)
	if len(got) != len(explicit) {
		t.Fatalf("resolveChildEnv(explicit) = %v, want unchanged %v", got, explicit)
	}
	for i := range explicit {
		if got[i] != explicit[i] {
			t.Fatalf("resolveChildEnv(explicit)[%d] = %q, want %q", i, got[i], explicit[i])
		}
	}
}
