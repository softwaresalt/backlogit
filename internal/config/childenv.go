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
//
// The name comparison is case-INSENSITIVE (strings.EqualFold), matching
// os.LookupEnv/ResolveFormalGateKey's own resolution semantics: environment
// variable NAMES are case-insensitive at the OS level on Windows, and an
// operator, secrets manager, or .env loader may set the variable with
// non-canonical case (e.g. PowerShell `$env:backlogit_gate_evidence_key`). A
// case-SENSITIVE comparison here would let key resolution keep succeeding
// while scrubbing silently failed to strip the entry — leaking the secret
// into every spawned child process. A case-insensitive match is strictly
// conservative: an operator who happens to define an unrelated variable that
// only differs from the reserved name by case is exceedingly unlikely, and
// scrubbing it too is harmless.
func ScrubGateEvidenceKeyEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, gateEvidenceKeyEnvVar) {
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
