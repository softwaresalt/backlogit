package gateevidence

import (
	"fmt"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateproof"
)

// FormalContext is the verifier-supplied context a formal-admission check
// must match against the envelope reconstructed from a candidate event. A
// proof's MAC only proves it was signed by someone holding the key — it does
// NOT by itself prove it belongs to the item/workspace being checked. This
// context closes that gap so a valid proof for one item can never be
// accepted as evidence for a different item, and a valid proof from one
// workspace can never be replayed into another.
type FormalContext struct {
	// WorkspaceID and ItemID are the expected values; a candidate whose
	// envelope carries different values is refused.
	WorkspaceID string
	ItemID      string
	// Key is the resolved formal-gate-evidence HMAC key.
	Key []byte
	// HighWaterCounter, when non-nil, is the counter floor read from an
	// external verifier-owned high-water ledger (config.FormalGateEnforced /
	// BACKLOGIT_GATE_HIGHWATER_LEDGER). backlogit only ever READS this value
	// for comparison; updating the ledger after a successful admission is the
	// external verifier's responsibility, never this codebase's.
	HighWaterCounter *int64
}

// FormalResult is the outcome of a formal-admission check.
type FormalResult struct {
	// Admitted is true only when every formal-admission requirement passed.
	Admitted bool
	// Reason is a short, human-readable refusal explanation. Empty when Admitted.
	Reason string
	// Event points at the admitted candidate event, or nil when not admitted.
	Event *events.Event
}

// refuse builds a non-admitted FormalResult with a reason, reducing
// call-site boilerplate across FormalAdmit's many refusal branches.
func refuse(reason string) FormalResult {
	return FormalResult{Admitted: false, Reason: reason}
}

// FormalAdmit is the dedicated formal-admission predicate (106-F F1/U6),
// distinct from Latest. Unlike Latest — which accepts EventGateForced
// regardless of ran, and keeps an earlier pass even after a later
// block/requeue — FormalAdmit is deliberately strict, admitting a candidate
// only when ALL of the following hold:
//
//   - the candidate is the chronologically LATEST EventGatePassed event
//     (EventGateForced is never admissible, regardless of its proof);
//   - no EventGateBlocked, EventGateRequeued, or EventGateEscalated event
//     appears after it (a later block/requeue invalidates a prior pass);
//   - its delta's "ran" field is true;
//   - its delta carries proof/key_id/proof_schema/counter/report_digest/
//     head_sha fields that reconstruct into an Envelope matching ctx's
//     WorkspaceID and ItemID, and gateproof.Verify succeeds against ctx.Key;
//   - its counter is strictly greater than every OTHER counter recorded
//     anywhere in the event slice (the intact-log rollback/duplicate-detection
//     guarantee), and, when ctx.HighWaterCounter is set, strictly greater than
//     that external ledger value too (the stronger guarantee).
func FormalAdmit(evs []events.Event, ctx FormalContext) FormalResult {
	if len(evs) == 0 {
		return refuse("no events recorded for this item")
	}

	candidateIdx := -1
	for i := range evs {
		if evs[i].EventType == EventGatePassed {
			candidateIdx = i
		}
	}
	if candidateIdx == -1 {
		return refuse("no EventGatePassed event present (EventGateForced is never formally admissible)")
	}

	for i := candidateIdx + 1; i < len(evs); i++ {
		switch evs[i].EventType {
		case EventGateBlocked, EventGateRequeued, EventGateEscalated:
			return refuse("a later block/requeue/escalate event invalidates the prior pass")
		}
	}

	candidate := evs[candidateIdx]
	ran, _ := candidate.Delta["ran"].(bool)
	if !ran {
		return refuse("candidate pass event has ran=false")
	}

	env, macHex, err := envelopeFromEvent(candidate, ctx)
	if err != nil {
		return refuse(err.Error())
	}

	if verifyErr := gateproof.Verify(env, macHex, ctx.Key); verifyErr != nil {
		return refuse("proof verification failed: " + verifyErr.Error())
	}

	maxOther, hasOther, otherErr := maxOtherCounter(evs, candidateIdx, ctx)
	if otherErr != nil {
		return refuse("log integrity check failed: " + otherErr.Error())
	}
	if hasOther && env.Counter <= maxOther {
		return refuse("counter is not strictly greater than another counter recorded in this log (possible replay)")
	}
	if ctx.HighWaterCounter != nil && env.Counter <= *ctx.HighWaterCounter {
		return refuse("counter is not strictly greater than the external high-water ledger value")
	}

	return FormalResult{Admitted: true, Event: &evs[candidateIdx]}
}

// envelopeFromEvent reconstructs a gateproof.Envelope and its MAC from an
// event's OWN EventType and delta fields, binding the verifier-supplied
// WorkspaceID/ItemID context into the reconstructed envelope (rather than
// trusting whatever the delta itself might claim) so a proof cannot vouch for
// a different item or workspace than the one being checked. It returns an
// error (not a panic or zero-value silent pass) for any missing or
// wrong-typed field, since an incomplete delta can never be formally
// admissible.
//
// Unlike a hardcoded EventGatePassed/ran:true assumption, this reads the
// event's real EventType and Delta["ran"] so it can reconstruct and verify
// ANY signed event — not just the FormalAdmit candidate — which is required
// by maxOtherCounter to authenticate "other" (non-candidate) events before
// trusting their counter claims (106-F F1 review finding F6).
func envelopeFromEvent(ev events.Event, ctx FormalContext) (gateproof.Envelope, string, error) {
	delta := ev.Delta
	proof, ok := delta["proof"].(string)
	if !ok || proof == "" {
		return gateproof.Envelope{}, "", errMissingField("proof")
	}
	keyID, _ := delta["key_id"].(string)
	schema, ok := asInt(delta["proof_schema"])
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("proof_schema")
	}
	counter, ok := asInt64(delta["counter"])
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("counter")
	}
	reportDigest, _ := delta["report_digest"].(string)
	headSHA, _ := delta["head_sha"].(string)
	ran, _ := delta["ran"].(bool)

	env := gateproof.Envelope{
		Magic:        gateproof.Magic,
		Purpose:      gateproof.PurposeTask,
		Schema:       schema,
		Alg:          gateproof.AlgHMACSHA256,
		KeyID:        keyID,
		WorkspaceID:  ctx.WorkspaceID,
		ItemID:       ctx.ItemID,
		EventType:    ev.EventType,
		Ran:          ran,
		Actor:        "backlogit",
		TimestampUTC: asString(delta["timestamp_utc"]),
		HeadSHA:      headSHA,
		ReportDigest: reportDigest,
		Counter:      counter,
	}
	return env, proof, nil
}

// errMissingField reports a required delta field that was absent or
// wrong-typed. Kept as a small helper so every field check produces a
// consistent, greppable message shape.
func errMissingField(name string) error {
	return &missingFieldError{field: name}
}

type missingFieldError struct{ field string }

func (e *missingFieldError) Error() string {
	return "candidate delta missing required field: " + e.field
}

func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	default:
		return 0, false
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// maxOtherCounter returns the highest counter value among evs (excluding
// excludeIdx) that is BACKED BY A VERIFIED PROOF under ctx, along with
// whether any such counter was found, and an error if any OTHER event
// CLAIMS a counter (carries a non-empty "counter" delta field) but its OWN
// proof does not verify.
//
// A verification failure on a counter-claiming "other" event is a same-log-
// tampering signal distinct from — and not covered by — the plan's
// already-accepted deletion/truncation tradeoff: an actor can edit an
// EXISTING, RETAINED event's counter field in place (no deletion needed) to
// deflate this comparison's floor, letting a stale, previously-signed,
// lower-counter proof be replayed as the new "latest pass." Authenticating
// every counter-claiming event (not just the winning candidate) before
// trusting it closes that gap: an unverifiable claim proves the intact-log
// premise this whole guarantee depends on no longer holds, so the caller
// must refuse the ENTIRE admission (fail closed) rather than silently
// ignoring the bad data point, which would just admit the attack through a
// different door.
//
// An event with NO counter field at all is skipped — never counted, never
// verified — because that shape is the NORMAL, expected signature of history
// that predates formal enforcement (byte-identical legacy evidence, per
// augmentDeltaWithFormalProof's backward-compatibility contract), not a
// tampering signal.
func maxOtherCounter(evs []events.Event, excludeIdx int, ctx FormalContext) (int64, bool, error) {
	var max int64
	found := false
	for i := range evs {
		if i == excludeIdx {
			continue
		}
		if _, hasCounter := evs[i].Delta["counter"]; !hasCounter {
			continue
		}
		env, macHex, err := envelopeFromEvent(evs[i], ctx)
		if err != nil {
			return 0, false, fmt.Errorf("other event at index %d claims a counter but is malformed: %w", i, err)
		}
		if verifyErr := gateproof.Verify(env, macHex, ctx.Key); verifyErr != nil {
			return 0, false, fmt.Errorf("other event at index %d claims a counter but its proof does not verify: %w", i, verifyErr)
		}
		if !found || env.Counter > max {
			max = env.Counter
			found = true
		}
	}
	return max, found, nil
}
