// Package evidencedag is the shipment-level fault-line evidence DAG framework
// (S10). A shipment declares, in text, a DAG of required checks (nodes) with
// evidence dependencies (edges). Each node is bound to a detector (S4-S9) whose
// evidence conforms to the shared S4 U4 contract in internal/faultline. The
// DAG is the single authoritative review-ready gate: a shipment is review-ready
// only when every applicable terminal node is verified or validly waived.
//
// The package is a library; wiring it into shipment assembly is a follow-up.
// It owns the one graph-evaluation core (validation, topological evaluation,
// transitive propagation, incremental caching, roll-up) that S11 consumes and
// extends through Compose rather than re-implementing traversal or roll-up.
//
// Everything fails closed: anything that cannot be positively established is
// unmet or unverifiable, and every input and output is bounded.
package evidencedag
