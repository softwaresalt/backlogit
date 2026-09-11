package harness_test

import (
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/errwrap"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestContractAnalyzer(t *testing.T) {
	require.NotNil(t, errwrap.Analyzer)
	require.Equal(t, "FL002errwrap", errwrap.Analyzer.Name)
	require.NotNil(t, errwrap.Analyzer.Run)

	testdata, err := filepath.Abs("..")
	require.NoError(t, err)

	analysistest.Run(
		t,
		testdata,
		errwrap.Analyzer,
		"fl002bad",
		"fl002good",
	)
}
