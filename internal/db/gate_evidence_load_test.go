package db_test

import (
	"context"
	"testing"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
)

// TestLoadPassingGateEvidence_StatusTokensMatchConstants is the drift guard for
// the literal status tokens embedded in the LoadPassingGateEvidence SQL
// (gate_status IN ('passed','forced','forced_no_run')). If the gateevidence
// Status* constants ever change, this test fails so the SQL is updated in step.
func TestLoadPassingGateEvidence_StatusTokensMatchConstants(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{gateevidence.StatusPassed, "passed"},
		{gateevidence.StatusForced, "forced"},
		{gateevidence.StatusForcedNoRun, "forced_no_run"},
		{gateevidence.StatusMissing, "missing"},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("gate status token drift: got %q, want %q (update LoadPassingGateEvidence SQL)", tc.got, tc.want)
		}
	}
}

// TestLoadPassingGateEvidence_ReturnsOnlyPositiveRows verifies the positive-index
// contract: passed/forced/forced_no_run rows are returned; missing (and any other
// non-positive) rows are excluded so the caller re-verifies them via the
// authoritative log-scan.
func TestLoadPassingGateEvidence_ReturnsOnlyPositiveRows(t *testing.T) {
	ctx := context.Background()
	database := setupTestDB(t)

	rows := map[string]string{
		"001.001-T": gateevidence.StatusPassed,
		"001.002-T": gateevidence.StatusForced,
		"001.003-T": gateevidence.StatusForcedNoRun,
		"001.004-T": gateevidence.StatusMissing,
	}
	for id, status := range rows {
		if _, err := database.ExecContext(ctx,
			`INSERT OR REPLACE INTO gate_evidence (item_id, gate_status, evidence_sha, head_sha) VALUES (?, ?, ?, ?)`,
			id, status, "", ""); err != nil {
			t.Fatalf("seed row %s: %v", id, err)
		}
	}

	got, err := db.LoadPassingGateEvidence(ctx, database)
	if err != nil {
		t.Fatalf("LoadPassingGateEvidence: %v", err)
	}

	want := map[string]string{
		"001.001-T": gateevidence.StatusPassed,
		"001.002-T": gateevidence.StatusForced,
		"001.003-T": gateevidence.StatusForcedNoRun,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d: %v", len(got), len(want), got)
	}
	for id, status := range want {
		if got[id] != status {
			t.Errorf("item %s: got %q, want %q", id, got[id], status)
		}
	}
	if _, present := got["001.004-T"]; present {
		t.Errorf("missing-status row must be excluded from the positive index")
	}
}
