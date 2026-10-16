// Package soundness is the plan and work-item soundness linter. A single rule
// engine validates harvested plans, features, tasks and subtasks before
// shipment assembly. The engine is a library: the pre-assembly gate and any
// other caller invoke the same Run function, so enforcement never diverges.
package soundness
