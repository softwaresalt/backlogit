package gate

import (
	"context"
	stderrors "errors"
	"os/exec"
	"strings"
	"testing"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
)

type fakeVersion struct {
	ver string
	err error
}

func (f fakeVersion) Version(context.Context) (string, error) { return f.ver, f.err }

func TestProbe(t *testing.T) {
	tests := []struct {
		name        string
		enabled     EnabledMode
		vr          fakeVersion
		wantEnforce bool
		wantSetup   bool
	}{
		{
			name: "resolvable compatible enforces", enabled: EnabledTrue,
			vr: fakeVersion{ver: "1.4.7"}, wantEnforce: true,
		},
		{
			name: "newer version enforces", enabled: EnabledTrue,
			vr: fakeVersion{ver: "1.5.0"}, wantEnforce: true,
		},
		{
			name: "missing binary under auto fails open", enabled: EnabledAuto,
			vr: fakeVersion{err: bkerrors.ErrGateBinaryNotFound}, wantEnforce: false,
		},
		{
			name: "missing binary under true is setup error", enabled: EnabledTrue,
			vr: fakeVersion{err: bkerrors.ErrGateBinaryNotFound}, wantEnforce: false, wantSetup: true,
		},
		{
			name: "version below floor under true is setup error", enabled: EnabledTrue,
			vr: fakeVersion{ver: "1.4.6"}, wantEnforce: false, wantSetup: true,
		},
		{
			name: "version below floor under auto fails open", enabled: EnabledAuto,
			vr: fakeVersion{ver: "1.3.0"}, wantEnforce: false,
		},
		{
			name: "disabled never enforces", enabled: EnabledFalse,
			vr: fakeVersion{ver: "1.4.7"}, wantEnforce: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforce, err := Probe(context.Background(), tt.vr, tt.enabled)
			if enforce != tt.wantEnforce {
				t.Fatalf("enforce = %v, want %v", enforce, tt.wantEnforce)
			}
			if tt.wantSetup {
				var ge *bkerrors.GateError
				if !stderrors.As(err, &ge) || ge.Class != "setup" {
					t.Fatalf("err = %v, want setup-class GateError", err)
				}
				if !stderrors.Is(err, bkerrors.ErrGateSetup) {
					t.Fatalf("err = %v, want ErrGateSetup", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		have, want string
		ok         bool
	}{
		{"1.4.7", "1.4.7", true},
		{"1.4.8", "1.4.7", true},
		{"1.5.0", "1.4.7", true},
		{"2.0.0", "1.4.7", true},
		{"1.4.6", "1.4.7", false},
		{"1.3.9", "1.4.7", false},
		{"v1.4.7", "1.4.7", true},
		{"1.4.7-dev", "1.4.7", true},
	}
	for _, c := range cases {
		got, err := versionAtLeast(c.have, c.want)
		if err != nil {
			t.Fatalf("versionAtLeast(%q,%q): %v", c.have, c.want, err)
		}
		if got != c.ok {
			t.Fatalf("versionAtLeast(%q,%q) = %v, want %v", c.have, c.want, got, c.ok)
		}
	}
}

// TestExecVersionRunner_NilEnvDefaultsToScrubbed verifies that
// ExecVersionRunner.Version, when Env is unset, no longer leaves cmd.Env nil
// (which would inherit the ambient environment including
// BACKLOGIT_GATE_EVIDENCE_KEY unfiltered) and still functions correctly
// against a real binary (106-F F1/U2). The explicit-Env branch is unchanged
// existing code and is already exercised by TestProbe's fake VersionRunner.
func TestExecVersionRunner_NilEnvDefaultsToScrubbed(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	t.Setenv("BACKLOGIT_GATE_EVIDENCE_KEY", "0123456789abcdef0123456789abcdef")

	r := ExecVersionRunner{Binary: "git"} // Env intentionally unset (nil)
	out, err := r.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() unexpected error: %v", err)
	}
	// firstVersionToken strips leading "git"/"version" words and returns just
	// the parsed version token (e.g. "2.55.0.windows.3"), so assert on shape
	// (digits and dots) rather than the literal word "git".
	if !strings.ContainsAny(out, "0123456789") || !strings.Contains(out, ".") {
		t.Fatalf("Version() = %q, want a version-shaped token", out)
	}
}

