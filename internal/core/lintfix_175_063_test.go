package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type lf175063Group struct {
	path     string
	message  string
	residual int
}

// TestU175_063_LintFix fails while errcheck findings remain in the groups owned
// by task 175.063-T (line-stable group multiset; line and column are ignored).
func TestU175_063_LintFix(t *testing.T) {
	groups := []lf175063Group{
		{"internal/core/hierarchy.go", "Error return value of `rows.Close` is not checked", 0},
		{"internal/core/stash.go", "Error return value of `f.Close` is not checked", 0},
		{"internal/core/stash.go", "Error return value of `file.Close` is not checked", 0},
	}
	counts := lf175063Findings(t, "./internal/core")
	for _, g := range groups {
		key := g.path + "|" + g.message
		require.Equalf(t, g.residual, counts[key], "owned errcheck group %q in %s: remaining findings must equal expected residual", g.message, g.path)
	}
}

func lf175063Findings(t *testing.T, pkg string) map[string]int {
	t.Helper()
	return lf175062Findings(t, pkg)
}
