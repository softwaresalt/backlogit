package core

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/softwaresalt/backlogit/internal/config"
)

// NewReadOnlyWorkspace opens a workspace for read-only previews (for example
// shipment manifest dry-runs). It never runs shipment recovery, never opens or
// migrates the SQLite cache, and never writes to the storage root, so a pending
// recovery journal is left untouched. Callers must only use core paths that
// read canonical Markdown from disk (the DryRun paths); DB is nil.
func NewReadOnlyWorkspace(ctx context.Context, rootPath string) (*Workspace, error) {
	backlogitDir, err := ResolveStorageRoot(rootPath)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	cfg, err := config.Load(ctx, backlogitDir)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return &Workspace{
		StorageRoot:                 backlogitDir,
		RootPath:                    filepath.Dir(backlogitDir),
		Config:                      cfg,
		writeStashEntriesAtomically: writeStringAtomically,
	}, nil
}
