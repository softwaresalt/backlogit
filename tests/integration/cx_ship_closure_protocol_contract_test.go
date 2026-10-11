package integration_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUCXS1_ShipClosureProtocolContract(t *testing.T) {
	repoRoot := testRepoRoot(t)
	readSource := func(relativePath string) string {
		t.Helper()

		content, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relativePath)))
		require.NoError(t, err, "read contract source %s", relativePath)
		return string(content)
	}
	normalizeWhitespace := func(text string) string {
		return strings.Join(strings.Fields(text), " ")
	}
	sliceBetweenUniqueAnchors := func(text, startAnchor, endAnchor string) (string, bool) {
		if strings.Count(text, startAnchor) != 1 || strings.Count(text, endAnchor) != 1 {
			return "", false
		}
		start := strings.Index(text, startAnchor)
		end := strings.Index(text, endAnchor)
		if end <= start {
			return "", false
		}
		return text[start:end], true
	}

	ship := normalizeWhitespace(readSource(".github/agents/_ship.agent.md"))
	step6Item1, ok := sliceBetweenUniqueAnchors(ship,
		"1. **Close the shipment**", "2. **Runtime validation and releasability evidence**")
	require.True(t, ok, "Ship Step 6 item 1 must have unique start and end anchors")

	t.Run("AllowlistedStaging", func(t *testing.T) {
		assert.NotContains(t, step6Item1, "git add .backlogit/",
			"Step 6 item 1 must not stage the whole backlog directory")
		for _, literal := range []string{"safe-close report", "allowlist", "git diff --cached --name-only"} {
			assert.Contains(t, step6Item1, literal, "Step 6 item 1 allowlisted staging must name %q", literal)
		}
	})

	t.Run("SafeCloseOrdering", func(t *testing.T) {
		pre := strings.Index(step6Item1, "mode: pre")
		safeClose := strings.Index(step6Item1, "mode: safe-close")
		post := strings.Index(step6Item1, "mode: post")
		require.GreaterOrEqual(t, pre, 0, "Step 6 item 1 must invoke shipment-reconcile with mode: pre")
		require.Greater(t, safeClose, pre, "mode: safe-close must follow mode: pre")
		require.Greater(t, post, safeClose, "mode: post must follow mode: safe-close")

		const shipmentCall = "Call `backlogit_ship_shipment`"
		outside := 0
		for _, loc := range regexp.MustCompile(regexp.QuoteMeta(shipmentCall)).FindAllStringIndex(step6Item1, -1) {
			if loc[0] < safeClose || loc[0] >= post {
				outside++
			}
		}
		assert.Zero(t, outside, "direct backlogit_ship_shipment calls must sit inside the mode: safe-close step")
	})

	t.Run("CapabilityPredicate", func(t *testing.T) {
		// The heading clause ends where sub-item a0 begins.
		heading := step6Item1
		if i := strings.Index(step6Item1, " a0. "); i >= 0 {
			heading = step6Item1[:i]
		}
		assert.Contains(t, heading, "features.shipments: true",
			"Step 6 item 1 heading must gate on the features.shipments: true registry key")
		assert.NotContains(t, step6Item1, "shipments are enabled in this workspace",
			"Step 6 item 1 must not assume shipments are enabled")
	})
}
