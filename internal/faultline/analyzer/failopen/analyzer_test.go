package failopen_test

import (
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/failopen"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer invokes the analysistest contract for FL003 directly.
func TestAnalyzer(t *testing.T) {
	require.NotNil(t, failopen.Analyzer)
	require.Equal(t, "FL003failopen", failopen.Analyzer.Name)
	require.NotNil(t, failopen.Analyzer.Run)

	analysistest.Run(
		t,
		analysistest.TestData(),
		failopen.Analyzer,
		"fl003bad",
		"fl003good",
	)
}
