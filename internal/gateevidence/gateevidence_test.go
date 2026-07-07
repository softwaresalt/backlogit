package gateevidence_test

import (
	"testing"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
)

// TestIsGateEvent pins the gate-family classifier used by the Q3 projection to
// index only items that went through the gate.
func TestIsGateEvent(t *testing.T) {
	gate := []string{
		gateevidence.EventGatePassed, gateevidence.EventGateBlocked,
		gateevidence.EventGateForced, gateevidence.EventGateRequeued,
		gateevidence.EventGateEscalated, gateevidence.EventGateBaseOverride,
		gateevidence.EventGateError,
	}
	for _, et := range gate {
		if !gateevidence.IsGateEvent(et) {
			t.Errorf("IsGateEvent(%q) = false, want true", et)
		}
	}
	for _, et := range []string{"status_changed", "comment", "", "gate", "pre_task_completion_gate_"} {
		if gateevidence.IsGateEvent(et) {
			t.Errorf("IsGateEvent(%q) = true, want false", et)
		}
	}
}

func ev(t string, ran bool, hash, head string) events.Event {
	d := map[string]any{"ran": ran}
	if hash != "" {
		d["gate_report_hash"] = hash
	}
	if head != "" {
		d["head_sha"] = head
	}
	return events.Event{EventType: t, Delta: d}
}

// TestLatest_ComposedPredicate pins the finalized F4 composed predicate as a
// single shared leaf helper (Q3.0 / 083.005.001-ST): the latest event that is
// EventGateForced (unconditional audited break-glass, any ran) OR EventGatePassed
// with ran==true is the valid evidence; a fail-open EventGatePassed no-run is
// skipped. The forced_no_run token distinguishes a forced event with ran==false
// from a forced real run for audit visibility (Q3.2).
func TestLatest_ComposedPredicate(t *testing.T) {
	cases := []struct {
		name        string
		evs         []events.Event
		wantStatus  string
		wantEvent   bool // Event non-nil
		wantSHA     string
		wantHeadSHA string
	}{
		{
			name:       "empty stream is missing",
			evs:        nil,
			wantStatus: gateevidence.StatusMissing,
			wantEvent:  false,
		},
		{
			name:       "block-only stream is missing",
			evs:        []events.Event{ev(gateevidence.EventGateBlocked, false, "", ""), ev(gateevidence.EventGateRequeued, false, "", "")},
			wantStatus: gateevidence.StatusMissing,
			wantEvent:  false,
		},
		{
			name:        "passed ran=true is passed",
			evs:         []events.Event{ev(gateevidence.EventGatePassed, true, "sha:aa", "head:11")},
			wantStatus:  gateevidence.StatusPassed,
			wantEvent:   true,
			wantSHA:     "sha:aa",
			wantHeadSHA: "head:11",
		},
		{
			name:       "passed ran=false (fail-open) is skipped -> missing",
			evs:        []events.Event{ev(gateevidence.EventGatePassed, false, "sha:aa", "head:11")},
			wantStatus: gateevidence.StatusMissing,
			wantEvent:  false,
		},
		{
			name:        "forced ran=true is forced",
			evs:         []events.Event{ev(gateevidence.EventGateForced, true, "sha:bb", "head:22")},
			wantStatus:  gateevidence.StatusForced,
			wantEvent:   true,
			wantSHA:     "sha:bb",
			wantHeadSHA: "head:22",
		},
		{
			name:        "forced ran=false is forced_no_run but still valid evidence",
			evs:         []events.Event{ev(gateevidence.EventGateForced, false, "sha:cc", "head:33")},
			wantStatus:  gateevidence.StatusForcedNoRun,
			wantEvent:   true,
			wantSHA:     "sha:cc",
			wantHeadSHA: "head:33",
		},
		{
			name:       "interleaved: forced ran=true then passed ran=false -> latest qualifying is forced",
			evs:        []events.Event{ev(gateevidence.EventGateForced, true, "sha:bb", "head:22"), ev(gateevidence.EventGatePassed, false, "sha:zz", "head:99")},
			wantStatus: gateevidence.StatusForced,
			wantEvent:  true,
			wantSHA:    "sha:bb",
		},
		{
			name:       "interleaved: passed ran=true then forced ran=false -> latest is forced_no_run",
			evs:        []events.Event{ev(gateevidence.EventGatePassed, true, "sha:aa", "head:11"), ev(gateevidence.EventGateForced, false, "sha:cc", "head:33")},
			wantStatus: gateevidence.StatusForcedNoRun,
			wantEvent:  true,
			wantSHA:    "sha:cc",
		},
		{
			name:       "interleaved: forced ran=false then passed ran=true -> latest is passed",
			evs:        []events.Event{ev(gateevidence.EventGateForced, false, "sha:cc", "head:33"), ev(gateevidence.EventGatePassed, true, "sha:aa", "head:11")},
			wantStatus: gateevidence.StatusPassed,
			wantEvent:  true,
			wantSHA:    "sha:aa",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gateevidence.Latest(tc.evs)
			if got.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", got.Status, tc.wantStatus)
			}
			if (got.Event != nil) != tc.wantEvent {
				t.Errorf("Event non-nil = %v, want %v", got.Event != nil, tc.wantEvent)
			}
			if tc.wantSHA != "" && got.EvidenceSHA != tc.wantSHA {
				t.Errorf("EvidenceSHA = %q, want %q", got.EvidenceSHA, tc.wantSHA)
			}
			if tc.wantHeadSHA != "" && got.HeadSHA != tc.wantHeadSHA {
				t.Errorf("HeadSHA = %q, want %q", got.HeadSHA, tc.wantHeadSHA)
			}
		})
	}
}
