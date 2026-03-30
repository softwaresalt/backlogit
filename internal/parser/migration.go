package parser

import (
	"context"

	"github.com/backlogit/backlogit/internal/core"
)

// Migrate transforms a legacy backlog.md into atomic .backlogit/ artifacts.
//
// Worker: Implement extraction, decomposition, attribution, scaffolding, and archiving.
func Migrate(ctx context.Context, legacyPath string, workspace *core.Workspace) (int, error) {
	panic("not implemented: Worker: Implement legacy migration pipeline")
}
