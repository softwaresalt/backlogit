package harness_test

import (
	"path/filepath"
	"testing"

	"github.com/softwaresalt/backlogit/internal/faultline/analyzer/failopen"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestContractAnalyzer(t *testing.T) {
	require.NotNil(t, failopen.Analyzer)
	require.Equal(t, "FL003failopen", failopen.Analyzer.Name)
	require.NotNil(t, failopen.Analyzer.Run)

	analysistest.Run(
		t,
		filepath.Join(".."),
		failopen.Analyzer,
		"fl003bad",
		"fl003good",
	)
}
