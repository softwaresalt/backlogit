package config_test

// hooks_normalize_test.go: TDD tests for LoadHooks legacy-transition-map
// upgrade (124.004-T). Tests the upgradeLegacyTransitions discrimination:
//   - legacy generated-default map → upgraded (adds blocked->queued, active->queued)
//   - operator-customized map      → preserved byte-for-byte (no queued injected)
//   - absent transitions block     → resolves via DefaultTransitions() fallback

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// legacyDefaultHooksYAML is the hooks.yaml content that WriteDefaults would have
// generated BEFORE the 124.002-T transition-map widening (no queued in blocked/active).
// Slice values are in the original Go source order; map keys are in YAML-marshal
// alphabetical order to match what yaml.Marshal produces.
const legacyDefaultHooksYAML = `enabled: true
event_thresholds:
  blocked_stale_days: 7
agent_subscriptions:
  ship:
  - post_merge_closure
  - feature_review_ready
  stage:
  - feature_review_ready
  - blocked_stale
lifecycle:
  validate_transition: true
  emit_events: true
  transitions:
    active:
    - done
    - blocked
    - review
    - shipped
    - abandoned
    blocked:
    - active
    done:
    - archived
    queued:
    - active
    - blocked
    review:
    - done
    - accepted
    - rejected
  pre_task_completion_gate:
    enabled: auto
    terminal_statuses:
    - done
    autoharness_binary: autoharness
    base_ref: auto
    timeout_seconds: 600
    force_cli_only: true
    evidence_required: true
notifications:
  rate_limit_per_second: 10
`

// writeHooksYAML creates a directory with a hooks.yaml containing the given content.
func writeHooksYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hooks.yaml"), []byte(content), 0o644))
	return dir
}

// TestLoadHooks_LegacyMap_Upgraded asserts that a hooks.yaml carrying the
// pre-124.002-T generated-default transitions map is upgraded on load to
// include blocked->queued and active->queued.
// RED before the upgradeLegacyTransitions implementation; green after.
func TestLoadHooks_LegacyMap_Upgraded(t *testing.T) {
	dir := writeHooksYAML(t, legacyDefaultHooksYAML)

	cfg, err := config.LoadHooks(dir)
	require.NoError(t, err)

	tr := cfg.Lifecycle.Transitions

	// blocked must now include queued.
	assert.True(t, slices.Contains(tr["blocked"], "queued"),
		"LoadHooks must upgrade legacy map: blocked->queued missing; blocked targets: %v", tr["blocked"])

	// active must now include queued.
	assert.True(t, slices.Contains(tr["active"], "queued"),
		"LoadHooks must upgrade legacy map: active->queued missing; active targets: %v", tr["active"])
}

// TestLoadHooks_OperatorMap_Preserved asserts that a hooks.yaml whose
// transitions map differs from the prior generated default in any way is
// treated as operator-customized and left completely unchanged.
// This test must pass both before and after the implementation (preservation
// must never be broken).
func TestLoadHooks_OperatorMap_Preserved(t *testing.T) {
	// Operator-restricted map: active has only "done" (intentionally omits
	// blocked, review, shipped, abandoned, queued). This differs from the prior
	// generated default, so upgradeLegacyTransitions must leave it untouched.
	const operatorHooks = `lifecycle:
  validate_transition: true
  emit_events: true
  transitions:
    queued:
    - active
    active:
    - done
    blocked:
    - active
    done:
    - archived
`
	dir := writeHooksYAML(t, operatorHooks)

	cfg, err := config.LoadHooks(dir)
	require.NoError(t, err)

	tr := cfg.Lifecycle.Transitions

	// active must still be exactly [done] — no queued injected.
	assert.Equal(t, []string{"done"}, tr["active"],
		"operator-customized active must be preserved: %v", tr["active"])

	// blocked must still be exactly [active] — no queued injected.
	assert.Equal(t, []string{"active"}, tr["blocked"],
		"operator-customized blocked must be preserved: %v", tr["blocked"])

	// no unexpected keys should appear.
	assert.NotContains(t, tr["active"], "queued", "no queued in active for operator-customized map")
	assert.NotContains(t, tr["blocked"], "queued", "no queued in blocked for operator-customized map")
}

// TestLoadHooks_AbsentTransitions_FallsBackToDefault asserts that a hooks.yaml
// without a transitions block returns a nil Lifecycle.Transitions, which lets
// ValidateStatusTransition(nil) fall back to the updated DefaultTransitions()
// (which now includes blocked->queued and active->queued).
func TestLoadHooks_AbsentTransitions_FallsBackToDefault(t *testing.T) {
	const noTransitions = `lifecycle:
  validate_transition: true
  emit_events: true
`
	dir := writeHooksYAML(t, noTransitions)

	cfg, err := config.LoadHooks(dir)
	require.NoError(t, err)

	// Absent transitions block → nil/empty map → runtime falls back to DefaultTransitions().
	assert.Empty(t, cfg.Lifecycle.Transitions,
		"absent transitions block must yield nil/empty map; got: %v", cfg.Lifecycle.Transitions)
}
