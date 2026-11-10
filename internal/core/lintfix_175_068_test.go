package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type lf175068Group struct {
	path     string
	message  string
	residual int
}

// TestU175_068_LintFix fails while errcheck findings remain in the groups owned
// by task 175.068-T (line-stable group multiset; line and column are ignored).
func TestU175_068_LintFix(t *testing.T) {
	groups := []lf175068Group{
		{"internal/core/harness_status_test.go", "Error return value of `database.Close` is not checked", 0},
		{"internal/core/migrate_links.go", "Error return value of `rows.Close` is not checked", 0},
	}
	counts := lf175068Findings(t, "./internal/core")
	for _, g := range groups {
		key := g.path + "|" + g.message
		require.Equalf(t, g.residual, counts[key], "owned errcheck group %q in %s: remaining findings must equal expected residual", g.message, g.path)
	}
}

func lf175068Findings(t *testing.T, pkg string) map[string]int {
	t.Helper()
	return lf175062Findings(t, pkg)
}
