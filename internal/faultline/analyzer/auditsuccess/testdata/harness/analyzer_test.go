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

	analysistest.Run(
		t,
		filepath.Join(".."),
		auditsuccess.Analyzer,
		"fl004bad",
		"fl004good",
	)
}
