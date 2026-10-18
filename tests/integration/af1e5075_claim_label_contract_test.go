package integration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// af1e5075Sections holds the whitespace-normalized agent sections that the
// AF1E5075 pre-claim contract tests inspect.
type af1e5075Sections struct {
	orchestrator string
	ship         string
	shipFallback string
}

// loadAF1E5075Sections reads the Orchestrator and Ship agent files and slices
// the named sections used by the AF1E5075 contract tests.
func loadAF1E5075Sections(t *testing.T) af1e5075Sections {
	t.Helper()

	repoRoot := testRepoRoot(t)
	read := func(relativePath string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relativePath)))
		require.NoError(t, err)
		return strings.Join(strings.Fields(string(content)), " ")
	}
	slice := func(text, from, to string) string {
		t.Helper()
		start := strings.Index(text, from)
		require.GreaterOrEqual(t, start, 0, "section start %q not found", from)
		end := strings.Index(text[start:], to)
		require.GreaterOrEqual(t, end, 0, "section end %q not found after %q", to, from)
		return text[start : start+end]
	}

	orchestrator := read(".github/agents/_orchestrator.agent.md")
	ship := read(".github/agents/_ship.agent.md")
	shipSlice := slice(ship, "### Step 0.5: Shipment Intake", "### Validation Boundary")
	return af1e5075Sections{
		orchestrator: slice(orchestrator, "### Step 2: Route to Ship", "### Step 3: Iteration Decision"),
		ship:         shipSlice,
		shipFallback: slice(ship, "**Fallback path", "### Validation Boundary"),
	}
}

// runAF1E5075Subtests runs check once for the Orchestrator and once for Ship.
func runAF1E5075Subtests(t *testing.T, check func(t *testing.T, text string)) {
	t.Helper()

	sections := loadAF1E5075Sections(t)
	t.Run("Orchestrator", func(t *testing.T) { check(t, sections.orchestrator) })
	t.Run("Ship", func(t *testing.T) { check(t, sections.ship) })
}

func requireAF1E5075Tokens(t *testing.T, text string, tokens []string) {
	t.Helper()

	for _, token := range tokens {
		require.Contains(t, text, token)
	}
}

// TestAF1E5075ClaimLabelContract pins the hold-label refusal contract (U1).
//
// Transition-bound: the 154-S-specific assertions (the historical 154-S state
// and the 154-S archive evidence path) are tied to the 154-S convergence
// transition and may be retired once that transition is closed out.
func TestAF1E5075ClaimLabelContract(t *testing.T) {
	runAF1E5075Subtests(t, func(t *testing.T, text string) {
		t.Run("LabelStateTable", func(t *testing.T) {
			requireAF1E5075Tokens(t, text, []string{
				"hard claim refusal label",
				"`do-not-claim-until-convergence` and `bootstrap-bypass-unapproved`, waiver source any: Refuse",
				"`do-not-claim-until-convergence` and `bootstrap-bypass-approved-conditional`, waiver source any: Refuse",
				"`bootstrap-bypass-approved-conditional` only, waiver source recorded with every condition satisfied: Continue to condition (b)",
				"`bootstrap-bypass-approved-conditional` only, waiver source missing or a condition unsatisfied: Refuse",
				// Transition-bound (154-S): the second row is the historical state.
				"the historical `154-S` state",
			})
			require.Contains(t, text, "`do-not-claim-until-convergence`")
			require.Contains(t, text, "`bootstrap-bypass-unapproved`")
		})
		t.Run("Provenance", func(t *testing.T) {
			requireAF1E5075Tokens(t, text, []string{
				"`bootstrap-bypass-approved-conditional` is approval provenance, not a refusal label",
				"recorded waiver conditions",
				"fail closed when the waiver source is missing",
			})
		})
		t.Run("GoSignalAndBypass", func(t *testing.T) {
			requireAF1E5075Tokens(t, text, []string{
				"GO_SIGNAL: do-not-claim-until-convergence removed after a recorded C29EBEE5 PASS",
				"Gate: C29EBEE5 \u2014 SATISFIED",
				"never bypasses",
				"no DARK_MODE exemption",
			})
		})
	})
}
