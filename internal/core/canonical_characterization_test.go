package core

import "testing"

// TestGateReportHashCharacterization pins the CURRENT behavior of
// gateReportHash without changing it. gateReportHash hashes the raw broker
// report bytes with SHA-256 and returns lowercase hex; an empty report hashes
// to "".
//
// This is a green-baseline characterization test (not red-before-green): it
// documents today's raw-broker-bytes hash. F2 does NOT change gateReportHash.
// F1 (106-F) deliberately did NOT re-route it either — that migration was
// never one of F1's reviewed units; F1's own NEW hashing (proof envelope,
// formal report digest, shipment manifest digest) routes through
// internal/canonical, but this pre-existing best-effort hash is left as-is
// until a future unit explicitly re-plumbs it (see
// internal/canonical/guard_test.go's allowlist for the paired guard).
func TestGateReportHashCharacterization(t *testing.T) {
	if got := gateReportHash(nil); got != "" {
		t.Errorf("gateReportHash(nil) = %q, want empty string", got)
	}
	if got := gateReportHash([]byte{}); got != "" {
		t.Errorf("gateReportHash([]byte{}) = %q, want empty string", got)
	}

	// Fixed input pinned to the literal lowercase-hex SHA-256 observed today.
	const wantHex = "72cc92f555d2238f8e8864f57a0d70e81142d1fe290cc1da0cdd8a7958607405"
	if got := gateReportHash([]byte("gate-report-body")); got != wantHex {
		t.Errorf("gateReportHash(\"gate-report-body\") = %q, want %q", got, wantHex)
	}
}
