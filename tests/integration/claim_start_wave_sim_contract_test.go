package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUCS3_WaveSimClaimStart(t *testing.T) {
	type scenario struct {
		ID     string `json:"id"`
		Expect struct {
			Outcome                  string   `json:"outcome"`
			Waves                    int      `json:"waves"`
			Scheduled                int      `json:"scheduled"`
			HaltDetail               string   `json:"halt_detail"`
			HaltWave                 *int     `json:"halt_wave"`
			ActiveIDs                []string `json:"active_ids"`
			ClaimAssignedAtAdmission *int     `json:"claim_assigned_at_admission"`
		} `json:"expect"`
	}
	type simulationFixture struct {
		Scenarios []scenario `json:"scenarios"`
	}

	repoRoot := testRepoRoot(t)
	var fixture simulationFixture

	t.Run("Preserved", func(t *testing.T) {
		fixturePath := filepath.Join(repoRoot, filepath.FromSlash("tests/simulation/wave-scheduler-contract.json"))
		contents, err := os.ReadFile(fixturePath)
		require.NoError(t, err, "read wave scheduler simulation fixture")
		require.NoError(t, json.Unmarshal(contents, &fixture), "decode wave scheduler simulation fixture")

		present := make(map[string]struct{}, len(fixture.Scenarios))
		for _, item := range fixture.Scenarios {
			present[item.ID] = struct{}{}
		}
		existingScenarioIDs := []string{
			"baseline",
			"persistent_red_mapping",
			"blocked_injection",
			"blocked_mid_run",
			"active_residual",
			"unsupported_status_review",
			"unsupported_status_abandoned",
			"unsupported_status_off_catalog",
			"status_catalog_unavailable",
			"status_catalog_disagrees",
			"cycle_injection",
			"sibling_red_wave4",
			"non_frozen_m_control",
			"missing_green_maker",
			"ambiguous_green_maker",
			"missing_red_selector",
			"wrong_green_maker_close_wave",
			"green_maker_descoped",
			"open_red_early_green_carried_in",
			"open_red_closed_entry_not_reconfirmed",
			"green_maker_lands_but_selector_stays_red",
		}
		for _, id := range existingScenarioIDs {
			_, ok := present[id]
			require.True(t, ok, "existing scenario %q must remain present", id)
		}
	})

	t.Run("Fixture", func(t *testing.T) {
		findByID := func(t *testing.T, id string) *scenario {
			t.Helper()

			var found *scenario
			for i := range fixture.Scenarios {
				if fixture.Scenarios[i].ID == id {
					found = &fixture.Scenarios[i]
					break
				}
			}
			require.NotNil(t, found, "scenario %q must exist", id)
			return found
		}

		baseline := findByID(t, "baseline")
		claimAssigned := findByID(t, "claim_assigned_all")
		residuals := findByID(t, "claim_state_residuals")
		indeterminate := findByID(t, "claim_state_indeterminate")

		assert.Equal(t, "COMPLETE", claimAssigned.Expect.Outcome)
		assert.Equal(t, baseline.Expect.Waves, claimAssigned.Expect.Waves,
			"claim-assigned scenario must preserve baseline waves")
		assert.Equal(t, baseline.Expect.Scheduled, claimAssigned.Expect.Scheduled,
			"claim-assigned scenario must preserve baseline scheduled count")
		if assert.NotNil(t, claimAssigned.Expect.ClaimAssignedAtAdmission,
			"claim_assigned_all must expect claim_assigned_at_admission") {
			assert.Equal(t, claimAssigned.Expect.Scheduled, *claimAssigned.Expect.ClaimAssignedAtAdmission,
				"claim_assigned_at_admission must equal scheduled")
		}

		assert.Equal(t, "WAVE_NO_PROGRESS", residuals.Expect.Outcome)
		assert.Equal(t, "active residual", residuals.Expect.HaltDetail)
		assert.Len(t, residuals.Expect.ActiveIDs, 3,
			"claim_state_residuals must list exactly the residual active IDs")

		assert.Equal(t, "WAVE_CLAIM_STATE_INDETERMINATE", indeterminate.Expect.Outcome)
		if assert.NotNil(t, indeterminate.Expect.HaltWave,
			"claim_state_indeterminate must expect a halt wave") {
			assert.Equal(t, 1, *indeterminate.Expect.HaltWave)
		}
	})

	t.Run("Replay", func(t *testing.T) {
		pwshPath, err := exec.LookPath("pwsh")
		if err != nil {
			t.Fatalf("locate pwsh for wave scheduler simulation: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, pwshPath, "-NoProfile", "-File", "scripts/wave-scheduler-sim.ps1")
		cmd.Dir = repoRoot
		cmd.Env = append(os.Environ(), "NO_COLOR=1")
		rawOutput, runErr := cmd.CombinedOutput()

		ansiEscapePattern := regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07]*(?:\x07|\x1b\\)|[@-_])`)
		output := ansiEscapePattern.ReplaceAllString(string(rawOutput), "")
		output = strings.ReplaceAll(output, "\r\n", "\n")

		if ctx.Err() != nil {
			assert.Fail(t, "wave scheduler simulation timed out: %v\noutput:\n%s", ctx.Err(), output)
			return
		}

		exitCode := 0
		if runErr != nil {
			var exitErr *exec.ExitError
			if !errors.As(runErr, &exitErr) {
				assert.Fail(t, "run wave scheduler simulation: %v\noutput:\n%s", runErr, output)
				return
			}
			exitCode = exitErr.ExitCode()
		}
		assert.Equal(t, 0, exitCode, "wave scheduler simulation must exit successfully\noutput:\n%s", output)

		assert.NotRegexp(t, regexp.MustCompile(`(?m)^.*WAVE_SIM_FAIL.*$`), output,
			"simulation output must not contain a WAVE_SIM_FAIL line")
		trimmedOutput := strings.TrimSpace(output)
		assert.NotEmpty(t, trimmedOutput, "simulation output must not be empty")
		if trimmedOutput != "" {
			outputLines := strings.Split(trimmedOutput, "\n")
			assert.Regexp(t, regexp.MustCompile(`^WAVE_SIM_OK:`), strings.TrimSpace(outputLines[len(outputLines)-1]),
				"the final simulation output line must begin with WAVE_SIM_OK:")
		}

		outcomeLinePattern := regexp.MustCompile(`(?m)^[ \t]*\S+[ \t]+outcome=`)
		outcomeLines := outcomeLinePattern.FindAllString(output, -1)
		assert.GreaterOrEqual(t, len(fixture.Scenarios), 24,
			"fixture must contain at least the 21 preserved and 3 claim-start scenarios")
		assert.GreaterOrEqual(t, len(outcomeLines), 24,
			"simulation must report at least the 21 preserved and 3 claim-start scenarios")
		assert.Len(t, outcomeLines, len(fixture.Scenarios),
			"simulation outcome-line count must match the fixture scenario count")

		expectedOutcomes := []struct {
			id      string
			outcome string
		}{
			{id: "claim_assigned_all", outcome: "COMPLETE"},
			{id: "claim_state_residuals", outcome: "WAVE_NO_PROGRESS"},
			{id: "claim_state_indeterminate", outcome: "WAVE_CLAIM_STATE_INDETERMINATE"},
		}
		for _, expected := range expectedOutcomes {
			linePattern := regexp.MustCompile(
				`(?m)^[ \t]*` + regexp.QuoteMeta(expected.id) + `[ \t]+outcome=` +
					regexp.QuoteMeta(expected.outcome) + `([ \t]|$)`,
			)
			assert.Len(t, linePattern.FindAllString(output, -1), 1,
				"simulation must emit exactly one %s outcome=%s line", expected.id, expected.outcome)
		}
	})
}
