package config_test

// transitions_sync_test.go: mandatory deep-equality sync-guard and
// production-map coverage for the status-transition map (124.002-T).
// This external _test package can import both config and hooks without a cycle
// (hooks imports only stdlib + internal/errors; config does not import hooks).

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/hooks"
)

// TestTransitionMapSync_DeepEqual is the mandatory sync guard. It fails if the
// two transition-map definition sites drift apart, even on non-queued entries.
// No weak containment check — the maps must be reflect.DeepEqual.
func TestTransitionMapSync_DeepEqual(t *testing.T) {
	hookMap := hooks.DefaultTransitions()
	cfgMap := config.DefaultHooksConfig().Lifecycle.Transitions
	if !reflect.DeepEqual(hookMap, cfgMap) {
		t.Errorf("hooks.DefaultTransitions() and config.DefaultHooksConfig().Lifecycle.Transitions have drifted:\nhooks:  %v\nconfig: %v", hookMap, cfgMap)
	}
}

// TestProductionMapTransitions_BlockedAndActiveToQueued verifies the
// production-wired config map (workspace.go:114-118 wires Lifecycle.Transitions
// from DefaultHooksConfig) accepts the two new transitions. The nil-fed tests in
// builtin_pre_test.go exercise the hooks fallback; this test proves the defaults.go
// edit independently.
func TestProductionMapTransitions_BlockedAndActiveToQueued(t *testing.T) {
	cfgMap := config.DefaultHooksConfig().Lifecycle.Transitions
	hook := hooks.ValidateStatusTransition(cfgMap)

	cases := []struct {
		from string
		to   string
	}{
		{"blocked", "queued"},
		{"active", "queued"},
	}
	for _, tc := range cases {
		hc := hooks.HookContext{
			OldValues: map[string]any{"status": tc.from},
			NewValues: map[string]any{"status": tc.to},
		}
		err := hook(context.Background(), hc)
		assert.NoError(t, err, "production config map must allow %s->%s", tc.from, tc.to)
	}
}

// TestProductionMapTransitions_StillForbidden ensures the widening did not
// over-open the map (e.g. blocked->done remains rejected).
func TestProductionMapTransitions_StillForbidden(t *testing.T) {
	cfgMap := config.DefaultHooksConfig().Lifecycle.Transitions
	hook := hooks.ValidateStatusTransition(cfgMap)

	hc := hooks.HookContext{
		OldValues: map[string]any{"status": "blocked"},
		NewValues: map[string]any{"status": "done"},
	}
	require.Error(t, hook(context.Background(), hc), "blocked->done must remain forbidden")
}
