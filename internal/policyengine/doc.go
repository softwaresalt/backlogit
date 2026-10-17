// Package policyengine is the S11 deterministic workflow-policy enforcement
// engine. It turns the prose gates of the workflow policies (P-002/P-004 test
// first, P-009 merge commits, review and CI readiness) into a machine-enforced
// prerequisite graph: harness-scaffold, red-phase-verify, green-phase-verify,
// review-complete, ci-pass and merge-ready, ordered by edges declared in a
// text registry.
//
// The package consumes the shared evidencedag core (S10) as a library: the
// registry is decoded with evidencedag.DecodeStrict, validated as an
// evidencedag.Declaration, evaluated and rolled up by evidencedag.Gate. It adds
// only the policy node types, their evidence rules and the override and
// shipment roll-up policy. It never reimplements traversal, evaluation or
// roll-up.
//
// Every gate fails closed: absent, ambiguous, unverifiable or out-of-order
// evidence blocks. Report-only mode is an explicit, rollout-time relaxation of
// the final decision, never a relaxation of what is reported.
package policyengine
