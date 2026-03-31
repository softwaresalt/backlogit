package core

// MigrationReport summarizes the results of a flat-to-hierarchical migration.
type MigrationReport struct {
	FilesMoved   int
	FilesSkipped int
	Errors       []string
	DryRun       bool
}

// MigrateFlatToHierarchical converts an existing flat per-type directory layout
// (.backlogit/tasks/, bugs/, etc.) to the hierarchical .backlogit/queue/ structure.
//
// Worker: Scan all existing type-specific directories, read each artifact's frontmatter
// to determine type and parent relationships, assign hierarchical IDs, move files to
// .backlogit/queue/ with new naming, update frontmatter ID fields, write state file for
// crash recovery, and trigger full rehydration after completion.
func MigrateFlatToHierarchical(ws *Workspace, dryRun bool) (*MigrationReport, error) {
	panic("not implemented: Worker: Implement flat-to-hierarchical migration pipeline with state file tracking and atomic file moves")
}

// RollbackMigration reverses a previous migration using the state file
// (.backlogit/.migration-state) to restore the pre-migration layout.
//
// Worker: Read .migration-state file, reverse each recorded file move using
// atomic rename, remove the queue/ directory if empty, and delete the state file.
func RollbackMigration(ws *Workspace) error {
	panic("not implemented: Worker: Implement migration rollback using .migration-state file to reverse file moves")
}
