package core

import (
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUT2_CaptureSitesRestoreStdlibLogState runs the internal/core tests that
// swap slog.Default() for a buffer-backed handler. Installing a non-default
// slog handler also redirects the stdlib log package (log.SetOutput and
// log.SetFlags(0)), and restoring the previous slog default does not undo that.
// Each capture site must leave log.Writer(), log.Flags(), and log.Prefix() as
// they were before it ran.
//
// The sites are invoked directly rather than through a shared helper, so this
// harness compiles against the current tree and fails on the leaked state.
func TestUT2_CaptureSitesRestoreStdlibLogState(t *testing.T) {
	wantWriter, wantFlags, wantPrefix := log.Writer(), log.Flags(), log.Prefix()
	t.Cleanup(func() {
		log.SetOutput(wantWriter)
		log.SetFlags(wantFlags)
		log.SetPrefix(wantPrefix)
	})

	sites := []struct {
		name string
		run  func(*testing.T)
	}{
		{name: "gate_baseref_shadowed", run: TestGateBaseOverrideShadowed_WarnsAdvisory},
		{name: "gate_baseref_no_shadow", run: TestGateBaseNoShadow_NoWarn},
		{name: "shipped_event_append_failure", run: TestShipShipment_ShippedEventAppendFailureLogsFixedShape},
	}
	for _, site := range sites {
		t.Run(site.name, func(t *testing.T) {
			writer, flags, prefix := log.Writer(), log.Flags(), log.Prefix()
			t.Run("capture", site.run)
			assert.Same(t, writer, log.Writer(), "log.Writer() still redirects to the capture buffer after the site")
			assert.Equal(t, flags, log.Flags(), "log.Flags() was not restored after the site")
			assert.Equal(t, prefix, log.Prefix(), "log.Prefix() was not restored after the site")
		})
	}
}
