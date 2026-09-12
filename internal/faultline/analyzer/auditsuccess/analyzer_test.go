package auditsuccess_test

import (
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/auditsuccess"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

// TestAnalyzer runs the FL004 analysistest contract directly so a skipped or
// unmatched nested test cannot produce a false green result.
func TestAnalyzer(t *testing.T) {
	require.NotNil(t, auditsuccess.Analyzer)
	require.Equal(t, "FL004auditsuccess", auditsuccess.Analyzer.Name)
	require.NotNil(t, auditsuccess.Analyzer.Run)

	testdata, err := filepath.Abs("testdata")
	require.NoError(t, err)

	results := analysistest.Run(
		t,
		testdata,
		auditsuccess.Analyzer,
		"fl004bad",
		"fl004good",
	)
	require.Len(t, results, 2)
}
