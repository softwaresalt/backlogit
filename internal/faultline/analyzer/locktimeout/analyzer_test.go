package locktimeout_test

import (
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/locktimeout"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer invokes the analysistest contract for FL005 directly.
func TestAnalyzer(t *testing.T) {
	require.NotNil(t, locktimeout.Analyzer)
	require.Equal(t, "FL005locktimeout", locktimeout.Analyzer.Name)
	require.NotNil(t, locktimeout.Analyzer.Run)

	analysistest.Run(
		t,
		analysistest.TestData(),
		locktimeout.Analyzer,
		"fl005bad",
		"fl005good",
	)
}
