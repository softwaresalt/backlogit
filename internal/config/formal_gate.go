package config

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"regexp"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// minFormalGateKeyBytes is the minimum decoded key length required for the
// formal-gate-evidence HMAC key (106-F F1). 32 bytes matches the HMAC-SHA256
// block size and gives an adequate security margin against brute force.
const minFormalGateKeyBytes = 32

// hexOnlyPattern matches a string consisting entirely of hex digits with an
// even length, used to decide encoding precedence in ResolveFormalGateKey.
var hexOnlyPattern = regexp.MustCompile(`^[0-9a-fA-F]+$`)

// ResolveFormalGateKey resolves and validates the HMAC evidence key for the
// formal gate (106-F F1) EXCLUSIVELY from the BACKLOGIT_GATE_EVIDENCE_KEY
// environment variable. The key is never read from workspace config or a CLI
// flag — FormalGateConfig intentionally has no key field.
//
// The value must decode as strict base64 (standard encoding) or hex to at
// least minFormalGateKeyBytes decoded bytes; any other outcome is rejected.
//
// Encoding precedence (deliberate, to avoid ambiguity): hex is a strict subset
// of the base64 alphabet, so an unpadded, hex-shaped string can sometimes
// decode successfully under both encodings, producing DIFFERENT byte results.
// To resolve this deterministically, a value matching the hex charset with an
// even length is ALWAYS decoded as hex; base64 decoding is attempted only when
// the value is not hex-shaped.
func ResolveFormalGateKey() ([]byte, error) {
	raw, ok := os.LookupEnv("BACKLOGIT_GATE_EVIDENCE_KEY")
	if !ok || raw == "" {
		return nil, bkerrors.ErrGateKeyAbsent
	}

	if len(raw)%2 == 0 && hexOnlyPattern.MatchString(raw) {
		decoded, err := hex.DecodeString(raw)
		if err != nil {
			return nil, bkerrors.ErrGateKeyInvalid
		}
		if len(decoded) < minFormalGateKeyBytes {
			return nil, bkerrors.ErrGateKeyInvalid
		}
		return decoded, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, bkerrors.ErrGateKeyInvalid
	}
	if len(decoded) < minFormalGateKeyBytes {
		return nil, bkerrors.ErrGateKeyInvalid
	}
	return decoded, nil
}

// formalGateRequiredTruthy reports whether a BACKLOGIT_FORMAL_GATE_REQUIRED
// value represents an explicit "yes, enforce" decision.
func formalGateRequiredTruthy(v string) bool {
	switch v {
	case "1", "true", "TRUE", "True":
		return true
	default:
		return false
	}
}

// FormalGateEnforced reports whether formal-admission enforcement for gate
// evidence (106-F F1) is currently required. Enforcement is anchored OUTSIDE
// the workspace via BACKLOGIT_FORMAL_GATE_REQUIRED, which is authoritative:
// workspace config (FormalGateConfig.Enabled) may only RAISE strictness, never
// lower it. When the environment anchor requires enforcement, an explicit
// formal_gate.enabled: false in config is ignored and enforcement still
// applies.
func FormalGateEnforced(cfg FormalGateConfig) bool {
	if v, ok := os.LookupEnv("BACKLOGIT_FORMAL_GATE_REQUIRED"); ok {
		if formalGateRequiredTruthy(v) {
			return true
		}
		// An explicit non-truthy value (e.g. "false") does not itself disable
		// enforcement that the workspace opted into; it only means the
		// environment does not additionally REQUIRE it. Config may still enable.
	}
	return cfg.Enabled
}
