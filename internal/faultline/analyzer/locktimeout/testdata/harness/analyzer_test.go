package harness_test

import (
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/locktimeout"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestContractAnalyzer(t *testing.T) {
	require.NotNil(t, locktimeout.Analyzer)
	require.Equal(t, "FL005locktimeout", locktimeout.Analyzer.Name)
	require.NotNil(t, locktimeout.Analyzer.Run)

	testdata, err := filepath.Abs("..")
	require.NoError(t, err)

	analysistest.Run(
		t,
		testdata,
		locktimeout.Analyzer,
		"fl005bad",
		"fl005good",
	)
}
