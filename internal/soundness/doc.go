// Package soundness is the plan and work-item soundness linter. A single rule
// engine validates harvested plans, features, tasks and subtasks before
// shipment assembly. The engine is a library: callers invoke the same Run
// function, so enforcement never diverges. Today the library and the
// `backlogit soundness lint` CLI exist; wiring the gate into shipment assembly
// is a follow-up (the backlog prose loader is not yet present).
package soundness
