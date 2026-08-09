package gate

import "github.com/softwaresalt/backlogit/internal/config"

// resolveChildEnv returns env unchanged when explicitly set (non-nil),
// otherwise falls back to the ambient environment scrubbed of the
// formal-gate-evidence key (106-F F1/U2) via config.ChildProcessEnv. Every
// os/exec call site in this package that would otherwise leave cmd.Env nil —
// and therefore inherit the FULL ambient environment unfiltered, including
// BACKLOGIT_GATE_EVIDENCE_KEY when present — MUST resolve its environment
// through this helper instead of assigning cmd.Env directly. Centralizing the
// "explicit env, or safely-scrubbed default" resolution here (rather than
// repeating the nil-check-and-fallback inline at each call site, as
// ExecVersionRunner.Version originally did and ExecGitRunner.Verify
// previously omitted entirely) also makes the property independently
// testable without spawning a real subprocess (106-F F1 review finding F8).
func resolveChildEnv(env []string) []string {
	if env != nil {
		return env
	}
	return config.ChildProcessEnv()
}
