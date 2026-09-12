package harness_test

import (
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/auditsuccess"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestContractAnalyzer(t *testing.T) {
	require.NotNil(t, auditsuccess.Analyzer)
	require.Equal(t, "FL004auditsuccess", auditsuccess.Analyzer.Name)
	require.NotNil(t, auditsuccess.Analyzer.Run)

	testdata, err := filepath.Abs("..")
	require.NoError(t, err)

	analysistest.Run(
		t,
		testdata,
		auditsuccess.Analyzer,
		"fl004bad",
		"fl004good",
	)
}
