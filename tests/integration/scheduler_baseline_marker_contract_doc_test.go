package integration_test

// U5 (173.005-T) content-probe harness for the scheduler-baseline marker
// contract. The deliverable is a hand-written operator document, so the
// harness asserts over its content rather than over any Go symbol: it
// compiles before the document exists and fails on an assertion until the
// document publishes every item the task's acceptance criteria require.
//
// Two acceptance criteria are checked against their sources of truth instead
// of against restated strings:
//   - the three-part predicate and residuals R1-R3 must match the decision
//     artifact verbatim (whitespace-normalized so line wrapping is free);
//   - the recipe JSON paths must be the same paths the U0b transport tests
//     assert.

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	markerContractDocPath      = "docs/design-docs/scheduler-baseline-marker-contract.md"
	markerDecisionArtifactPath = "docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md"
	markerU0bCLITestPath       = "internal/cli/claim_marker_read_surface_test.go"
	markerU0bMCPTestPath       = "internal/mcp/claim_marker_read_surface_test.go"
)

// normalizeMarkerContractText collapses every whitespace run to one space so
// verbatim comparisons ignore line wrapping and list indentation.
func normalizeMarkerContractText(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// sliceBetween returns the whitespace-normalized text from the first
// occurrence of start through the end of the first occurrence of end that
// follows it. All three inputs are normalized first, so markers may span the
// source's line wrapping.
func sliceBetween(t *testing.T, text, start, end, label string) string {
	t.Helper()
	text = normalizeMarkerContractText(text)
	start = normalizeMarkerContractText(start)
	end = normalizeMarkerContractText(end)
	startIndex := strings.Index(text, start)
	require.GreaterOrEqual(t, startIndex, 0, "%s: start marker %q not found in the decision artifact", label, start)
	endOffset := strings.Index(text[startIndex:], end)
	require.GreaterOrEqual(t, endOffset, 0, "%s: end marker %q not found in the decision artifact", label, end)
	return text[startIndex : startIndex+endOffset+len(end)]
}

func readMarkerContractDoc(t *testing.T) string {
	t.Helper()
	return readGovernedSurface(t, testRepoRoot(t), markerContractDocPath)
}

func TestU5_MarkerContractDocPublishesKeyVersionAndRecipe(t *testing.T) {
	doc := readMarkerContractDoc(t)
	normalized := normalizeMarkerContractText(doc)

	required := []string{
		"`scheduler_baseline_claim`",
		"reserved but not write-protected",
		"`scheduler-baseline-marker/v1`",
		"breaking change",
		"consumer notice",
		"backlogit shipment list --status active --format json",
		"`backlogit_list_shipments`",
		"{\"status\":\"active\"}",
		"`.[0].id`",
		"`.[0].custom_fields.items[]`",
		"no top-level `items`",
		"backlogit get <id> --format json",
		"`backlogit_get_item`",
		"`.status`",
		"`.custom_fields.scheduler_baseline_claim`",
	}
	for _, want := range required {
		assert.Contains(t, normalized, want, "marker contract doc must publish %q", want)
	}
}

func TestU5_MarkerContractDocRecipePathsMatchU0bAssertions(t *testing.T) {
	doc := normalizeMarkerContractText(readMarkerContractDoc(t))
	repoRoot := testRepoRoot(t)
	cliTest := readGovernedSurface(t, repoRoot, markerU0bCLITestPath)
	mcpTest := readGovernedSurface(t, repoRoot, markerU0bMCPTestPath)

	// Each published path is paired with the U0b assertion that walks it, so a
	// drift on either side fails this harness.
	pairs := []struct {
		docPath   string
		assertion string
		source    string
	}{
		{"`.[0].id`", `shipments[0]["id"]`, cliTest},
		{"`.[0].custom_fields.items[]`", `shipments[0]["custom_fields"]`, cliTest},
		{"`.[0].custom_fields.items[]`", `shipmentFields["items"]`, cliTest},
		{"`.custom_fields.scheduler_baseline_claim`", `itemFields["scheduler_baseline_claim"]`, cliTest},
		{"`.custom_fields.scheduler_baseline_claim`", `itemFields["scheduler_baseline_claim"]`, mcpTest},
		{"{\"status\":\"active\"}", `map[string]any{"status": "active"}`, mcpTest},
	}
	for _, pair := range pairs {
		assert.Contains(t, pair.source, pair.assertion, "U0b must still assert %s", pair.assertion)
		assert.Contains(t, doc, pair.docPath, "marker contract doc must publish the U0b path %s", pair.docPath)
	}
}

func TestU5_MarkerContractDocPredicateMatchesDecisionVerbatim(t *testing.T) {
	doc := normalizeMarkerContractText(readMarkerContractDoc(t))
	decision := readGovernedSurface(t, testRepoRoot(t), markerDecisionArtifactPath)

	predicate := sliceBetween(t, decision,
		"An item is **claim-activated** if and only if all three hold:",
		"the item's ID is in that shipment's current manifest (`items`).",
		"predicate")
	assert.Contains(t, doc, normalizeMarkerContractText(predicate),
		"the three-part predicate must match the decision artifact verbatim")

	organic := sliceBetween(t, decision,
		"Every other active item is **organic-active**.",
		"the backlogit claim gate stays authoritative.",
		"organic-active and advisory rule")
	assert.Contains(t, doc, normalizeMarkerContractText(organic),
		"the organic-active and advisory rule must match the decision artifact verbatim")

	cardinality := sliceBetween(t, decision,
		"if a consumer's active-shipment read returns more than one active shipment,",
		"never on the raw value alone.",
		"cardinality guard")
	assert.Contains(t, doc, normalizeMarkerContractText(cardinality),
		"the active-shipment cardinality guard must match the decision artifact verbatim")

	assert.Contains(t, doc, "withdrawn", "the doc must state that the non-empty rule is withdrawn")
	assert.Contains(t, doc, "non-empty", "the doc must name the withdrawn non-empty rule")
}

func TestU5_MarkerContractDocResidualsMatchDecisionVerbatim(t *testing.T) {
	doc := normalizeMarkerContractText(readMarkerContractDoc(t))
	decision := readGovernedSurface(t, testRepoRoot(t), markerDecisionArtifactPath)

	residuals := []struct {
		label string
		start string
		end   string
	}{
		{"R1", "* **R1: stale markers persist.**", "that ignore the predicate will be wrong."},
		{"R2", "* **R2: any activation route for an item that carries the active shipment's", "The operator can restore such an item to `queued` before claiming."},
		{"R3", "* **R3: CLI/MCP divergence after a partial compensation.**", "Do not substitute another lifecycle operation."},
		{"mixed-binary caveat", "The mixed-binary caveat (attempt 6 P2-9) applies to **any** binary", "can re-create the F1 wedge."},
	}
	for _, residual := range residuals {
		text := sliceBetween(t, decision, residual.start, residual.end, residual.label)
		assert.Contains(t, doc, normalizeMarkerContractText(text),
			"residual %s must match the decision artifact verbatim", residual.label)
	}
}

func TestU5_MarkerContractDocLifecycleTableAndConsumerCaveats(t *testing.T) {
	doc := normalizeMarkerContractText(readMarkerContractDoc(t))

	required := []string{
		"Verification status",
		"U0a",
		"U0c",
		"U3",
		"unaudited",
		"182.001-T",
		"block, unblock, return, ship, abandon, normalize",
		"by construction",
		"docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md",
		"rollout checkpoint",
		"`json_extract`",
		"autoharness follow-up",
		"U1b",
		"advisory",
		"authoritative",
	}
	for _, want := range required {
		assert.Contains(t, doc, want, "marker contract doc must publish %q", want)
	}
}
