package config

import (
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// absBinary returns a platform-appropriate absolute path for negative tests.
func absBinary() string {
	if runtime.GOOS == "windows" {
		return `C:\Tools\autoharness.exe`
	}
	return "/usr/local/bin/autoharness"
}

func TestPreTaskCompletionGate_Defaults(t *testing.T) {
	g := PreTaskCompletionGateConfig{}
	g.Normalize()
	if g.Enabled != "auto" {
		t.Fatalf("enabled default = %q, want auto", g.Enabled)
	}
	if len(g.TerminalStatuses) != 1 || g.TerminalStatuses[0] != "done" {
		t.Fatalf("terminal_statuses default = %v, want [done]", g.TerminalStatuses)
	}
	if g.AutoharnessBinary != "autoharness" {
		t.Fatalf("autoharness_binary default = %q", g.AutoharnessBinary)
	}
	if g.BaseRef != "auto" {
		t.Fatalf("base_ref default = %q", g.BaseRef)
	}
	if g.TimeoutSeconds != 600 {
		t.Fatalf("timeout_seconds default = %d", g.TimeoutSeconds)
	}
	if !g.ForceCLIOnlyValue() {
		t.Fatal("force_cli_only default should be true")
	}
	if !g.EvidenceRequiredValue() {
		t.Fatal("evidence_required default should be true")
	}
	if err := g.Validate(); err != nil {
		t.Fatalf("default config invalid: %v", err)
	}
}

func TestPreTaskCompletionGate_YAMLRoundTrip(t *testing.T) {
	src := `enabled: "true"
terminal_statuses: [done, shipped]
autoharness_binary: autoharness
base_ref: origin/main
timeout_seconds: 120
force_cli_only: true
evidence_required: false
`
	var g PreTaskCompletionGateConfig
	if err := yaml.Unmarshal([]byte(src), &g); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	g.Normalize()
	if err := g.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if g.Enabled != "true" || g.TimeoutSeconds != 120 {
		t.Fatalf("round-trip mismatch: %+v", g)
	}
	if g.EvidenceRequiredValue() {
		t.Fatal("evidence_required should honor explicit false")
	}
}

func TestPreTaskCompletionGate_Validation(t *testing.T) {
	falsePtr := func() *bool { b := false; return &b }

	tests := []struct {
		name    string
		mutate  func(*PreTaskCompletionGateConfig)
		wantErr string
	}{
		{"invalid enabled", func(g *PreTaskCompletionGateConfig) { g.Enabled = "maybe" }, "enabled must be one of"},
		{"empty terminal statuses", func(g *PreTaskCompletionGateConfig) { g.TerminalStatuses = []string{} }, "terminal_statuses must not be empty"},
		{"unknown terminal status", func(g *PreTaskCompletionGateConfig) { g.TerminalStatuses = []string{"finished"} }, "unknown status"},
		{"zero timeout", func(g *PreTaskCompletionGateConfig) { g.TimeoutSeconds = 0 }, "timeout_seconds must be within"},
		{"negative timeout", func(g *PreTaskCompletionGateConfig) { g.TimeoutSeconds = -5 }, "timeout_seconds must be within"},
		{"huge timeout", func(g *PreTaskCompletionGateConfig) { g.TimeoutSeconds = 100000 }, "timeout_seconds must be within"},
		{"force_cli_only false rejected", func(g *PreTaskCompletionGateConfig) { g.ForceCLIOnly = falsePtr() }, "force_cli_only must be true"},
		{"absolute binary rejected", func(g *PreTaskCompletionGateConfig) { g.AutoharnessBinary = absBinary() }, "must not be an absolute path"},
		{"traversal binary rejected", func(g *PreTaskCompletionGateConfig) { g.AutoharnessBinary = "../evil/autoharness" }, "must not contain '..'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := PreTaskCompletionGateConfig{}
			g.Normalize()
			// Re-apply timeout default in case mutate sets it before Normalize; here
			// mutate runs AFTER Normalize so zero/neg values are genuinely invalid.
			tt.mutate(&g)
			err := g.Validate()
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}
