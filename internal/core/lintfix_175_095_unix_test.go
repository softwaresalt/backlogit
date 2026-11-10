package core

import (
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type lf175095Group struct {
	path     string
	message  string
	residual int
}

var (
	lf175LinuxOnce   sync.Once
	lf175LinuxCounts map[string]int
)

// TestU175_095_LintFix fails while errcheck findings remain in the groups owned
// by task 175.095-T (line-stable group multiset; line and column are ignored).
// A separate cached GOOS=linux scan covers the owned Linux-only files when the
// harness runs on another host OS.
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
	if runtime.GOOS == "linux" {
		return lf175062Findings(t, pkg)
	}
	lint := lf175062Linter(t)
	if pkg == "./internal/core" {
		lf175LinuxOnce.Do(func() {
			lf175LinuxCounts = lf175062Scan(t, lint, pkg, "linux")
		})
		require.NotNil(t, lf175LinuxCounts, "shared Linux core lint scan must have completed")
		return lf175LinuxCounts
	}
	return lf175062Scan(t, lint, pkg, "linux")
}
