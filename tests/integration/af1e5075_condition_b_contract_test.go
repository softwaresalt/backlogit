package integration_test

import "testing"

// TestAF1E5075ConditionBContract pins the condition (b) (P)+(C) contract (U2).
//
// Transition-bound: the 154-S-specific assertions (the 154-S paths and state
// table) and the (C) ownership assertions (the attestation rule and the
// "does not transfer (C)" boundary) are tied to the 074-DL transition and may
// be retired when a recorded operator or Stage decision confirms that the
// pipeline-topology gate checks consumption.
func TestAF1E5075ConditionBContract(t *testing.T) {
	runAF1E5075Subtests(t, func(t *testing.T, text string) {
		t.Run("TriggerAndP", func(t *testing.T) {
			requireAF1E5075Tokens(t, text, []string{
				"blocks-edge closure includes 154-S",
				".backlogit/archive/154-S.md",
				".backlogit/queue/154-S.md",
				"never an index read",
				"`154-S` not shipped: `CONDITION_B_UNSATISFIED`",
				"`154-S` archived without `archived_status: shipped`: `CONDITION_B_UNSATISFIED`",
				"`154-S` shipped without a valid attestation: `CONDITION_B_UNSATISFIED`",
				"`154-S` shipped with a valid attestation: proceed",
			})
		})
		t.Run("AttestationRule", func(t *testing.T) {
			requireAF1E5075Tokens(t, text, []string{
				"CONDITION_B_ATTESTED:",
				"CONDITION_B_REVOKED:",
				".backlogit/logs/154-S.jsonl",
				"RFC3339",
				"newest wins",
				"revocation wins",
				"operator (relayed by Orchestrator)",
				"verbatim",
			})
		})
		t.Run("Boundaries", func(t *testing.T) {
			requireAF1E5075Tokens(t, text, []string{
				"manifest-declared capability is not consumption proof",
				"does not transfer (C)",
				"terminal halt",
				"shipment_predecessor_not_shipped",
				"workspace-local overlay",
			})
		})
	})
}
