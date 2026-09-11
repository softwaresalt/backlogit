package errwrap_test

import (
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/errwrap"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer invokes the analysistest contract for FL002 directly.
func TestAnalyzer(t *testing.T) {
	require.NotNil(t, errwrap.Analyzer)
	require.Equal(t, "FL002errwrap", errwrap.Analyzer.Name)
	require.NotNil(t, errwrap.Analyzer.Run)

	analysistest.Run(
		t,
		analysistest.TestData(),
		errwrap.Analyzer,
		"fl002bad",
		"fl002good",
	)
}
