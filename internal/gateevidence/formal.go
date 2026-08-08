package gateevidence

import (
	"errors"
	"fmt"
	"math"

	bkerrors "github.com/softwaresalt/backlogit/internal/errors"
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
	// Err classifies a refusal as bkerrors.ErrProofInvalid (definitively
	// wrong: tampered field, wrong key, replayed counter, ran=false, or a
	// superseding later event) or bkerrors.ErrProofUnverifiable (could not
	// be evaluated at all: missing/malformed proof fields, or a log-
	// integrity check on an "other" event itself failed). Nil only when
	// Admitted is true. Callers that need MCP-contract-precise dispatch
	// (e.g. validateMemberGateEvidence) should wrap THIS field — not parse
	// Reason — so the specific formal_gate_proof_invalid/
	// formal_gate_proof_unverifiable MCP error codes (106-F F1/U8) are
	// reachable from every FormalAdmit refusal, not just the ones raised
	// directly by gateproof.Verify.
	Err error
	// Event points at the admitted candidate event, or nil when not admitted.
	Event *events.Event
}

// refuse builds a non-admitted FormalResult with a reason and a classified
// cause, reducing call-site boilerplate across FormalAdmit's many refusal
// branches.
func refuse(reason string, cause error) FormalResult {
	return FormalResult{Admitted: false, Reason: reason, Err: cause}
}

// classifyProofErr maps an arbitrary error into the ONE authoritative
// top-level sentinel FormalResult.Err should carry: ErrProofUnverifiable if
// err's chain already says so, ErrProofInvalid otherwise (the more common,
// more specific default — most FormalAdmit refusals are definitive
// rejections, not evaluation failures). A nil err (no specific underlying
// cause to inspect) also classifies as ErrProofInvalid.
func classifyProofErr(err error) error {
	if err != nil && errors.Is(err, bkerrors.ErrProofUnverifiable) {
		return bkerrors.ErrProofUnverifiable
	}
	return bkerrors.ErrProofInvalid
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
		return refuse("no events recorded for this item", bkerrors.ErrProofUnverifiable)
	}

	candidateIdx := -1
	for i := range evs {
		if evs[i].EventType == EventGatePassed {
			candidateIdx = i
		}
	}
	if candidateIdx == -1 {
		return refuse("no EventGatePassed event present (EventGateForced is never formally admissible)", bkerrors.ErrProofUnverifiable)
	}

	for i := candidateIdx + 1; i < len(evs); i++ {
		switch evs[i].EventType {
		case EventGateBlocked, EventGateRequeued, EventGateEscalated:
			return refuse("a later block/requeue/escalate event invalidates the prior pass", bkerrors.ErrProofInvalid)
		}
	}

	candidate := evs[candidateIdx]
	ran, _ := candidate.Delta["ran"].(bool)
	if !ran {
		return refuse("candidate pass event has ran=false", bkerrors.ErrProofInvalid)
	}

	env, macHex, err := envelopeFromEvent(candidate, ctx)
	if err != nil {
		return refuse(err.Error(), bkerrors.ErrProofUnverifiable)
	}

	if verifyErr := gateproof.Verify(env, macHex, ctx.Key); verifyErr != nil {
		return refuse("proof verification failed: "+verifyErr.Error(), classifyProofErr(verifyErr))
	}

	maxOther, hasOther, otherErr := maxOtherCounter(evs, candidateIdx, ctx)
	if otherErr != nil {
		return refuse("log integrity check failed: "+otherErr.Error(), classifyProofErr(otherErr))
	}
	if hasOther && env.Counter <= maxOther {
		return refuse("counter is not strictly greater than another counter recorded in this log (possible replay)", bkerrors.ErrProofInvalid)
	}
	if ctx.HighWaterCounter != nil && env.Counter <= *ctx.HighWaterCounter {
		return refuse("counter is not strictly greater than the external high-water ledger value", bkerrors.ErrProofInvalid)
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
	// key_id, timestamp_utc, and ran are UNCONDITIONALLY written by both
	// signing paths (augmentDeltaWithFormalProof / augmentShipmentDeltaWith-
	// FormalProof) for every signed event — never legitimately absent or a
	// non-native type. A missing key, or one holding the wrong type, can
	// only mean the record was never genuinely signed or was corrupted/
	// tampered after the fact, so it is refused as ErrProofUnverifiable
	// ("could not be evaluated at all" per the design doc's fail-closed
	// matrix) rather than silently defaulting to a zero value and letting a
	// downstream MAC mismatch report the less precise ErrProofInvalid
	// (106-F F1 review finding, round 3).
	keyID, ok := requiredString(delta, "key_id")
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("key_id")
	}
	schema, ok := asInt(delta["proof_schema"])
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("proof_schema")
	}
	counter, ok := asInt64(delta["counter"])
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("counter")
	}
	timestampUTC, ok := requiredString(delta, "timestamp_utc")
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("timestamp_utc")
	}
	ran, ok := requiredBool(delta, "ran")
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("ran")
	}
	// report_digest and head_sha ARE legitimately absent from the delta for
	// some genuine, non-tampered events: appendGateEvidence only writes
	// head_sha when outcome.HeadSHA is non-empty (e.g. never for a no-repo
	// workspace), and report_digest is only meaningfully populated for
	// EventGatePassed. Their absence must not be refused — only a WRONG
	// TYPE, which (unlike absence) can only arise from a corrupted or
	// tampered record, since a genuine signer always writes a string.
	reportDigest, ok := optionalTypedString(delta, "report_digest")
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("report_digest")
	}
	headSHA, ok := optionalTypedString(delta, "head_sha")
	if !ok {
		return gateproof.Envelope{}, "", errMissingField("head_sha")
	}

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
		Actor:        ev.Actor,
		TimestampUTC: timestampUTC,
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

// asInt widens v to int, requiring an EXACT integer value. A float64 (the
// shape every JSON-round-tripped number takes once decoded into a
// map[string]any) with a non-zero fractional part is REJECTED, not silently
// truncated: truncating would let a tampered field value (e.g. an original
// signed 1 edited to 1.5, or 1.999999) reconstruct back to the ORIGINAL
// integer and pass MAC verification despite the on-disk JSON having
// genuinely changed, contradicting the tamper-evidence guarantee (106-F F1
// review finding).
func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		if x != math.Trunc(x) {
			return 0, false
		}
		return int(x), true
	default:
		return 0, false
	}
}

// asInt64 is the int64 counterpart to asInt — see its doc comment for why a
// non-integer float64 is rejected rather than truncated.
func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		if x != math.Trunc(x) {
			return 0, false
		}
		return int64(x), true
	default:
		return 0, false
	}
}

// requiredString returns delta[name] as a string, succeeding only when the
// key is PRESENT and holds a native string value. Use for fields every
// signing path writes unconditionally (key_id, timestamp_utc) — for those,
// absence or a wrong type can only indicate an unsigned, corrupted, or
// tampered record, never a legitimate omission.
func requiredString(delta map[string]any, name string) (string, bool) {
	v, present := delta[name]
	if !present {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// requiredBool is the bool counterpart to requiredString — see its doc
// comment. Used for "ran", which every signing path writes unconditionally.
func requiredBool(delta map[string]any, name string) (bool, bool) {
	v, present := delta[name]
	if !present {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// optionalTypedString returns delta[name] as a string, treating outright
// absence as a legitimate "" (some signing paths conditionally omit a field
// entirely rather than writing an empty string — e.g. head_sha is never
// written at all for a no-repo outcome). A PRESENT value of the wrong type,
// however, can only arise from corruption or tampering: a genuine signer
// only ever writes a string for these fields, so returns false in that case.
func optionalTypedString(delta map[string]any, name string) (string, bool) {
	v, present := delta[name]
	if !present {
		return "", true
	}
	s, ok := v.(string)
	return s, ok
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
