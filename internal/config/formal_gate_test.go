package config

import (
	stderrors "errors"
	"os"
	"testing"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TestResolveFormalGateKey_Absent verifies that an unset BACKLOGIT_GATE_EVIDENCE_KEY
// resolves to ErrGateKeyAbsent, never a silent empty key.
func TestResolveFormalGateKey_Absent(t *testing.T) {
	unsetEnv(t, "BACKLOGIT_GATE_EVIDENCE_KEY")

	_, err := ResolveFormalGateKey()
	if !stderrors.Is(err, bkerrors.ErrGateKeyAbsent) {
		t.Fatalf("ResolveFormalGateKey() error = %v, want ErrGateKeyAbsent", err)
	}
}

// TestResolveFormalGateKey_TooShort verifies that a key decoding to fewer than
// 32 bytes is rejected as invalid rather than silently accepted.
func TestResolveFormalGateKey_TooShort(t *testing.T) {
	tests := []struct {
		name string
		val  string
	}{
		{"short hex (16 bytes)", "deadbeefdeadbeefdeadbeefdeadbeef"},
		{"short base64 (16 bytes)", "AAAAAAAAAAAAAAAAAAAAAA=="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", tt.val)
			_, err := ResolveFormalGateKey()
			if !stderrors.Is(err, bkerrors.ErrGateKeyInvalid) {
				t.Fatalf("ResolveFormalGateKey() error = %v, want ErrGateKeyInvalid", err)
			}
		})
	}
}

// TestResolveFormalGateKey_InvalidEncoding verifies that a value which is
// neither valid hex nor valid base64 is rejected as invalid.
func TestResolveFormalGateKey_InvalidEncoding(t *testing.T) {
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "not-a-valid-key-!!!@@@###")
	_, err := ResolveFormalGateKey()
	if !stderrors.Is(err, bkerrors.ErrGateKeyInvalid) {
		t.Fatalf("ResolveFormalGateKey() error = %v, want ErrGateKeyInvalid", err)
	}
}

// TestResolveFormalGateKey_ValidHex verifies a 32-byte (64 hex-char) key decodes
// successfully and unambiguously as hex even though it is also syntactically
// valid unpadded base64 (hex is a subset of the base64 alphabet). Hex is
// preferred whenever the value is hex-shaped (even length, hex-only charset)
// to avoid silently misinterpreting an operator-supplied hex key as base64.
func TestResolveFormalGateKey_ValidHex(t *testing.T) {
	hexKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", hexKey)
	key, err := ResolveFormalGateKey()
	if err != nil {
		t.Fatalf("ResolveFormalGateKey() unexpected error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("ResolveFormalGateKey() len = %d, want 32 (hex decode, not base64)", len(key))
	}
	if key[0] != 0x01 || key[1] != 0x23 {
		t.Fatalf("ResolveFormalGateKey() = %x, want hex-decoded bytes starting 0123", key)
	}
}

// TestResolveFormalGateKey_ValidBase64 verifies a padded base64-encoded 32-byte
// key decodes successfully via the base64 path.
func TestResolveFormalGateKey_ValidBase64(t *testing.T) {
	// 32 bytes of 0x01, standard base64 encoding (padded).
	b64Key := "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE="
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", b64Key)
	key, err := ResolveFormalGateKey()
	if err != nil {
		t.Fatalf("ResolveFormalGateKey() unexpected error: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("ResolveFormalGateKey() len = %d, want 32", len(key))
	}
}

// TestFormalGateEnforced_EnvironmentAnchorAuthoritative verifies that
// BACKLOGIT_FORMAL_GATE_REQUIRED is the sole authority for RAISING enforcement,
// and that workspace config cannot lower it once the environment requires it.
func TestFormalGateEnforced_EnvironmentAnchorAuthoritative(t *testing.T) {
	tests := []struct {
		name        string
		envRequired string
		cfgEnabled  bool
		want        bool
	}{
		{"env absent, config disabled -> not enforced", "", false, false},
		{"env absent, config enabled -> enforced (workspace opt-in)", "", true, true},
		{"env requires, config disabled -> STILL enforced (config cannot lower)", "true", false, true},
		{"env requires, config enabled -> enforced", "true", true, true},
		{"env explicitly false, config disabled -> not enforced", "false", false, false},
		{"env explicitly 0, config disabled -> not enforced", "0", false, false},
		// A plausible deployment typo (e.g. a truncated "true") or an
		// unrecognized-but-present value (e.g. "yes") must NEVER silently
		// downgrade this fail-closed, tamper-resistant anchor to "not
		// enforced" -- only an EXPLICIT falsy value opts out. Round 8
		// review finding.
		{"env is a typo of true, config disabled -> STILL enforced (fail closed on typo)", "tru", false, true},
		{"env is an unrecognized non-empty value, config disabled -> STILL enforced (fail closed on unknown)", "yes", false, true},
		{"env has leading/trailing whitespace -> still enforced (trimmed)", " true ", false, true},
		{"env has trailing newline (shell export artifact) -> still enforced", "true\n", false, true},
		{"env is mixed-case -> still enforced (case-insensitive)", "tRue", false, true},
		{"env is bare \"1\" with whitespace -> still enforced", " 1 ", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envRequired == "" {
				unsetEnv(t, "BACKLOGIT_FORMAL_GATE_REQUIRED")
			} else {
				t.Setenv("BACKLOGIT_FORMAL_GATE_REQUIRED", tt.envRequired)
			}
			cfg := FormalGateConfig{Enabled: tt.cfgEnabled}
			got := FormalGateEnforced(cfg)
			if got != tt.want {
				t.Fatalf("FormalGateEnforced() = %v, want %v", got, tt.want)
			}
		})
	}
}

// unsetEnv removes an environment variable for the duration of the test,
// restoring any prior value afterward via t.Cleanup.
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	prev, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, prev)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}
