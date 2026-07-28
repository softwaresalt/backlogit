package core

import "github.com/softwaresalt/backlogit/internal/events"

// NewWorkspaceEventWriter constructs an events.EventWriter for the workspace,
// applying the durable_writes option when the workspace config enables it. It is
// the single flag-reading construction point that the core event producers and
// the MCP server funnel through, so enabling durable_writes actually reaches
// every event producer instead of silently no-op'ing at an un-rewired
// construction site. It is nil-safe: a nil workspace or a nil config yields a
// durable-off writer (byte-for-byte the prior default construction).
func NewWorkspaceEventWriter(ws *Workspace, logsDir string) *events.EventWriter {
	return events.NewEventWriter(logsDir, events.WithDurableWrites(WorkspaceDurableWrites(ws)))
}

// WorkspaceDurableWrites reports whether the workspace opts into the
// durable_writes fsync protocol. It is nil-safe for callers that hold an
// incomplete workspace (for example an MCP server bound to a root before the
// workspace is initialized).
func WorkspaceDurableWrites(ws *Workspace) bool {
	return ws != nil && ws.Config != nil && ws.Config.DurableWrites
}
