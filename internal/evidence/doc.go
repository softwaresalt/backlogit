// Package evidence is the single source of derived readiness evidence: GitHub
// review/CI/merge evidence obtained through the gh CLI and backlogit
// shipment/task/provenance evidence read from the workspace files. Evidence is
// read-only and degrades explicitly (StatusDegraded) rather than passing
// falsely when a source is unavailable. A claim validator compares prose
// readiness claims against that evidence and fails closed.
package evidence
