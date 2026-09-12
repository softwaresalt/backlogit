package scannerdiscipline

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer verifies FL001 Scanner.Buffer and Scanner.Err discipline,
// including its type boundary, escape exclusions, and suppression comment.
func TestAnalyzer(t *testing.T) {
	require.NotNil(t, Analyzer)
	require.Equal(t, "FL001scannerdiscipline", Analyzer.Name)
	require.NotNil(t, Analyzer.Run)

	analysistest.Run(
		t,
		analysistest.TestData(),
		Analyzer,
		"fl001bad",
		"fl001good",
	)
}
