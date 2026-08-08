package gateevidence_test

import (
	"testing"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
	"github.com/softwaresalt/backlogit/internal/gateproof"
)

func testKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef")
}

// signedPassEvent builds a real, correctly-signed EventGatePassed event so
// tests exercise genuine MAC verification rather than fixture deltas.
func signedPassEvent(t *testing.T, itemID, workspaceID string, counter int64, ran bool) events.Event {
	t.Helper()
	env := gateproof.Envelope{
		Magic:        gateproof.Magic,
		Purpose:      gateproof.PurposeTask,
		Schema:       gateproof.Schema,
		Alg:          gateproof.AlgHMACSHA256,
		KeyID:        "k1",
		WorkspaceID:  workspaceID,
		ItemID:       itemID,
		EventType:    gateevidence.EventGatePassed,
		Ran:          ran,
		Actor:        "backlogit",
		TimestampUTC: "2026-08-08T00:00:00Z",
		HeadSHA:      "deadbeef",
		ReportDigest: "digest123",
		Counter:      counter,
	}
	proof, err := gateproof.Sign(env, testKey())
	if err != nil {
		t.Fatalf("Sign() unexpected error: %v", err)
	}
	return events.Event{
		EventType: gateevidence.EventGatePassed,
		Delta: map[string]any{
			"ran":           ran,
			"proof":         proof,
			"key_id":        env.KeyID,
			"proof_schema":  env.Schema,
			"counter":       env.Counter,
			"head_sha":      env.HeadSHA,
			"report_digest": env.ReportDigest,
			"timestamp_utc": env.TimestampUTC,
		},
	}
}

func signedForcedEvent(t *testing.T, itemID, workspaceID string, counter int64) events.Event {
	t.Helper()
	env := gateproof.Envelope{
		Magic:        gateproof.Magic,
		Purpose:      gateproof.PurposeTask,
		Schema:       gateproof.Schema,
		Alg:          gateproof.AlgHMACSHA256,
		KeyID:        "k1",
		WorkspaceID:  workspaceID,
		ItemID:       itemID,
		EventType:    gateevidence.EventGateForced,
		Ran:          true,
		Actor:        "backlogit",
		TimestampUTC: "2026-08-08T00:00:00Z",
		ReportDigest: "digest123",
		Counter:      counter,
	}
	proof, err := gateproof.Sign(env, testKey())
	if err != nil {
		t.Fatalf("Sign() unexpected error: %v", err)
	}
	return events.Event{
		EventType: gateevidence.EventGateForced,
		Delta: map[string]any{
			"ran":           true,
			"proof":         proof,
			"key_id":        env.KeyID,
			"proof_schema":  env.Schema,
			"counter":       env.Counter,
			"report_digest": env.ReportDigest,
		},
	}
}

func baseCtx() gateevidence.FormalContext {
	return gateevidence.FormalContext{
		WorkspaceID: "ws-1",
		ItemID:      "106.099-T",
		Key:         testKey(),
	}
}

func TestFormalAdmit_HappyPathAdmitted(t *testing.T) {
	evs := []events.Event{signedPassEvent(t, "106.099-T", "ws-1", 1, true)}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if !res.Admitted {
		t.Fatalf("FormalAdmit() = not admitted, reason=%q, want admitted", res.Reason)
	}
}

func TestFormalAdmit_ForcedEventNeverAdmissible(t *testing.T) {
	evs := []events.Event{signedForcedEvent(t, "106.099-T", "ws-1", 1)}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted a forced event; EventGateForced must never be admissible")
	}
}

func TestFormalAdmit_LaterBlockAfterPassRefused(t *testing.T) {
	evs := []events.Event{
		signedPassEvent(t, "106.099-T", "ws-1", 1, true),
		{EventType: gateevidence.EventGateBlocked, Delta: map[string]any{"ran": false}},
	}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted a pass superseded by a later block")
	}
}

func TestFormalAdmit_LaterRequeueAfterPassRefused(t *testing.T) {
	evs := []events.Event{
		signedPassEvent(t, "106.099-T", "ws-1", 1, true),
		{EventType: gateevidence.EventGateRequeued, Delta: map[string]any{"ran": false}},
	}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted a pass superseded by a later requeue")
	}
}

func TestFormalAdmit_ReplayedCounterRefused(t *testing.T) {
	// A legitimate higher-numbered event exists earlier in the log (counter 2);
	// the "latest positioned" pass event carries a lower counter (1) — an
	// intact-log-detectable replay.
	evs := []events.Event{
		signedPassEvent(t, "106.099-T", "ws-1", 2, true),
		signedPassEvent(t, "106.099-T", "ws-1", 1, true),
	}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted a replayed (non-maximal) counter")
	}
}

func TestFormalAdmit_HighWaterLedgerRejectsNonIncreasingCounter(t *testing.T) {
	evs := []events.Event{signedPassEvent(t, "106.099-T", "ws-1", 1, true)}
	ctx := baseCtx()
	hw := int64(1)
	ctx.HighWaterCounter = &hw
	res := gateevidence.FormalAdmit(evs, ctx)
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted a counter not strictly greater than the high-water ledger value")
	}
}

func TestFormalAdmit_HighWaterLedgerAdmitsStrictlyGreaterCounter(t *testing.T) {
	evs := []events.Event{signedPassEvent(t, "106.099-T", "ws-1", 2, true)}
	ctx := baseCtx()
	hw := int64(1)
	ctx.HighWaterCounter = &hw
	res := gateevidence.FormalAdmit(evs, ctx)
	if !res.Admitted {
		t.Fatalf("FormalAdmit() = not admitted, reason=%q, want admitted (counter 2 > ledger 1)", res.Reason)
	}
}

func TestFormalAdmit_RanFalseRefused(t *testing.T) {
	evs := []events.Event{signedPassEvent(t, "106.099-T", "ws-1", 1, false)}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted an event with ran=false")
	}
}

func TestFormalAdmit_WrongItemContextRefused(t *testing.T) {
	// Proof was signed for a DIFFERENT item; verifier expects "106.099-T" but
	// the event is really for "106.100-T" — must not be admitted even though
	// the MAC itself is internally consistent for its own (wrong) item_id.
	evs := []events.Event{signedPassEvent(t, "106.100-T", "ws-1", 1, true)}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted a proof whose item_id does not match the verifier context")
	}
}

func TestFormalAdmit_WrongWorkspaceContextRefused(t *testing.T) {
	evs := []events.Event{signedPassEvent(t, "106.099-T", "ws-OTHER", 1, true)}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted a proof whose workspace_id does not match the verifier context")
	}
}

func TestFormalAdmit_WrongKeyRefused(t *testing.T) {
	evs := []events.Event{signedPassEvent(t, "106.099-T", "ws-1", 1, true)}
	ctx := baseCtx()
	ctx.Key = []byte("ffffffffffffffffffffffffffffffff")
	res := gateevidence.FormalAdmit(evs, ctx)
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted a proof verified against the wrong key")
	}
}

func TestFormalAdmit_NoEventsRefused(t *testing.T) {
	res := gateevidence.FormalAdmit(nil, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted with no events at all")
	}
}

func TestFormalAdmit_MissingProofFieldsRefused(t *testing.T) {
	evs := []events.Event{{EventType: gateevidence.EventGatePassed, Delta: map[string]any{"ran": true}}}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted an event with no proof fields at all")
	}
}

// TestFormalAdmit_TamperedOtherEventCounterRefused verifies a DISTINCT
// anti-replay attack beyond the plan's already-accepted deletion/truncation
// tradeoff: an actor edits an EXISTING, RETAINED event's counter field
// in-place (down from its genuinely-signed value) WITHOUT re-signing it,
// deflating the max-other-counter comparison so a stale, previously-used,
// replayed candidate counter appears to be the new maximum. Before this
// fix, maxOtherCounter trusted any "other" event's counter field blindly
// (never verifying that event's own proof); this let an in-place-tampered
// log defeat the intact-log premise the whole counter guarantee depends on.
// The fix authenticates every OTHER event that claims a counter before
// trusting it — a claim that fails its own verification is a same-log-
// tampering signal, so the WHOLE admission must refuse (fail closed)
// rather than silently ignoring the bad data point (106-F F1 review
// finding F6).
func TestFormalAdmit_TamperedOtherEventCounterRefused(t *testing.T) {
	genuine := signedPassEvent(t, "106.099-T", "ws-1", 3, true)
	genuine.Delta["counter"] = int64(1) // in-place tamper: counter edited, MAC not updated

	stale := signedPassEvent(t, "106.099-T", "ws-1", 2, true) // a validly-signed but stale/replayed candidate

	evs := []events.Event{genuine, stale}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if res.Admitted {
		t.Fatal("FormalAdmit() admitted despite an other event's counter claim failing its own proof verification (log integrity compromised)")
	}
}

// TestFormalAdmit_LegacyOtherEventWithoutProofNotTreatedAsTampering verifies
// that backward compatibility is preserved: an "other" event that predates
// formal enforcement (no counter/proof fields at all — a normal, expected,
// non-malicious history shape) is silently skipped, never treated as a
// tampering signal, and never blocks admission of a genuinely later signed
// pass.
func TestFormalAdmit_LegacyOtherEventWithoutProofNotTreatedAsTampering(t *testing.T) {
	legacy := events.Event{EventType: gateevidence.EventGatePassed, Delta: map[string]any{"ran": true}}
	current := signedPassEvent(t, "106.099-T", "ws-1", 1, true)
	evs := []events.Event{legacy, current}
	res := gateevidence.FormalAdmit(evs, baseCtx())
	if !res.Admitted {
		t.Fatalf("FormalAdmit() = not admitted, reason=%q, want admitted (legacy pre-enforcement history must not count as tampering)", res.Reason)
	}
}
