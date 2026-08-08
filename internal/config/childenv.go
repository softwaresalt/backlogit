package config

import (
	"os"
	"strings"
)

// gateEvidenceKeyEnvVar is the environment variable name carrying the formal
// gate evidence HMAC key (106-F F1). It must never appear in a child-process
// environment, error string, log line, or persisted artifact.
const gateEvidenceKeyEnvVar = "BACKLOGIT_GATE_EVIDENCE_KEY"

// ScrubGateEvidenceKeyEnv returns a copy of env with any entry for
// BACKLOGIT_GATE_EVIDENCE_KEY removed. It is a pure function so every call
// site that builds a child-process environment can be tested directly against
// it without spawning a real subprocess.
func ScrubGateEvidenceKeyEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if key == gateEvidenceKeyEnvVar {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// ChildProcessEnv returns the current process environment (os.Environ()) with
// the formal-gate-evidence key stripped. Every production exec.Command /
// exec.CommandContext call site in this module that would otherwise inherit
// the full ambient environment (cmd.Env left nil) MUST route through this
// function — or an equivalent allowlist such as gate.MinimalEnv, which
// excludes the key by construction — so the key can never leak into a spawned
// process (106-F F1/U2).
func ChildProcessEnv() []string {
	return ScrubGateEvidenceKeyEnv(os.Environ())
}
