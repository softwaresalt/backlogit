package core

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core/gate"
)

// TestGateBaseOverrideShadowed_WarnsAdvisory verifies the F1 (083.001-T) core
// surface: when a pinned non-auto config base_ref shadows an operator --gate-base,
// the completion path emits an advisory warning while preserving config-first
// precedence (the completion still succeeds and the resolved base is the config
// value). The warning is advisory only and never changes the outcome.
func TestGateBaseOverrideShadowed_WarnsAdvisory(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	// Broker with a pinned non-auto config base_ref; fakeGitAllOK verifies it.
	ws.GateBroker = &gate.Broker{
		Runner:         runner,
		Git:            fakeGitAllOK{},
		Version:        fakeVersion{v: okVersion},
		Enabled:        gate.EnabledTrue,
		TimeoutSeconds: 30,
		ConfigBaseRef:  "release/1.x",
	}
	cfg := config.PreTaskCompletionGateConfig{Enabled: string(gate.EnabledTrue)}
	cfg.Normalize()
	ws.gateConfig = cfg

	// Capture default-logger output for the duration of this (non-parallel) test.
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	id := newActiveTask(t, ws)
	// Complete through the gate WITH an operator --gate-base override that is
	// shadowed by the pinned config base_ref.
	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id,
		map[string]any{"status": "done"}, TransitionOptions{GateBase: "feature-base"})
	require.NoError(t, err, "config-first precedence: the completion still succeeds")
	require.Equal(t, "done", statusOf(t, ws, id))
	assert.Contains(t, buf.String(), "shadowed",
		"a shadowed --gate-base must emit an advisory warning")
}

// TestGateBaseNoShadow_NoWarn confirms the advisory does NOT fire when only the
// operator --gate-base is supplied (config base_ref is auto/unset).
func TestGateBaseNoShadow_NoWarn(t *testing.T) {
	ws := newGateTestWorkspace(t)
	runner := &fakeGateRunner{res: gate.GateResult{ExitCode: 0, Stdout: []byte(`{}`)}}
	// No config base_ref pinned; the operator --gate-base is honored, not shadowed.
	injectBroker(ws, gate.EnabledTrue, runner, fakeVersion{v: okVersion})

	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	id := newActiveTask(t, ws)
	_, _, err := UpdateArtifactWithGate(context.Background(), ws, id,
		map[string]any{"status": "done"}, TransitionOptions{GateBase: "feature-base"})
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "shadowed",
		"a gate_base-only override must not be flagged as shadowed")
}
