package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type lf175095Group struct {
	path     string
	message  string
	residual int
}

// TestU175_095_LintFix fails while errcheck findings remain in the groups owned
// by task 175.095-T (line-stable group multiset; line and column are ignored).
// The shared core findings include a GOOS=linux scan for the owned Linux-only
// files; the harness itself runs on any host OS.
func TestU175_095_LintFix(t *testing.T) {
	groups := []lf175095Group{
		{"internal/core/shipment_reconcile_append_unix.go", "Error return value of `dirFile.Close` is not checked", 0},
		{"internal/core/shipment_reconcile_append_unix.go", "Error return value of `file.Close` is not checked", 0},
		{"internal/core/shipment_reconcile_evidence_unix.go", "Error return value of `file.Close` is not checked", 0},
	}
	counts := lf175095Findings(t, "./internal/core")
	for _, g := range groups {
		key := g.path + "|" + g.message
		require.Equalf(t, g.residual, counts[key], "owned errcheck group %q in %s: remaining findings must equal expected residual", g.message, g.path)
	}
}

func lf175095Findings(t *testing.T, pkg string) map[string]int {
	t.Helper()
	return lf175062Findings(t, pkg)
}
