package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MigrationReport summarizes the results of a flat-to-hierarchical migration.
type MigrationReport struct {
	FilesMoved   int
	FilesSkipped int
	Errors       []string
	DryRun       bool
}

// migrationStateEntry records a single file move for rollback purposes.
type migrationStateEntry struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// MigrateFlatToHierarchical converts an existing flat per-type directory layout
// to the flat .backlogit/queue/ structure.
// When dryRun is true, no files are moved but the report reflects what would happen.
func MigrateFlatToHierarchical(ws *Workspace, dryRun bool) (*MigrationReport, error) {
	report := &MigrationReport{DryRun: dryRun}

	backlogDir := workspaceStorageRoot(ws)
	entries, err := os.ReadDir(backlogDir)
	if err != nil {
		return nil, fmt.Errorf("read workspace dir: %w", err)
	}

	var moves []migrationStateEntry

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "queue" || entry.Name() == "archive" || entry.Name()[0] == '.' {
			continue
		}
		srcDir := filepath.Join(backlogDir, entry.Name())
		files, err := os.ReadDir(srcDir)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("read %s: %v", srcDir, err))
			continue
		}
		queueDir := filepath.Join(backlogDir, "queue")
		for _, f := range files {
			if f.IsDir() || filepath.Ext(f.Name()) != ".md" {
				report.FilesSkipped++
				continue
			}
			from := filepath.Join(srcDir, f.Name())
			to := filepath.Join(queueDir, f.Name())
			moves = append(moves, migrationStateEntry{From: from, To: to})
			if !dryRun {
				if err := os.MkdirAll(queueDir, 0o755); err != nil {
					return nil, fmt.Errorf("mkdir %s: %w", queueDir, err)
				}
				if err := os.Rename(from, to); err != nil {
					report.Errors = append(report.Errors, fmt.Sprintf("move %s: %v", f.Name(), err))
					continue
				}
			}
			report.FilesMoved++
		}
	}

	if !dryRun {
		statePath := filepath.Join(backlogDir, ".migration-state")
		data, err := json.Marshal(moves)
		if err != nil {
			return nil, fmt.Errorf("marshal migration state: %w", err)
		}
		tmpPath := statePath + ".tmp"
		if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
			return nil, fmt.Errorf("write migration state: %w", err)
		}
		if err := os.Rename(tmpPath, statePath); err != nil {
			os.Remove(tmpPath) //nolint:errcheck
			return nil, fmt.Errorf("commit migration state: %w", err)
		}
	}

	return report, nil
}

// RollbackMigration reverses a previous migration using the state file
// (.backlogit/.migration-state) to restore the pre-migration layout.
func RollbackMigration(ws *Workspace) error {
	statePath := filepath.Join(workspaceStorageRoot(ws), ".migration-state")
	data, err := os.ReadFile(statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read migration state: %w", err)
	}

	var moves []migrationStateEntry
	if err := json.Unmarshal(data, &moves); err != nil {
		return fmt.Errorf("parse migration state: %w", err)
	}

	for i := len(moves) - 1; i >= 0; i-- {
		m := moves[i]
		if err := os.MkdirAll(filepath.Dir(m.From), 0o755); err != nil {
			return fmt.Errorf("mkdir for rollback: %w", err)
		}
		if err := os.Rename(m.To, m.From); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rollback rename: %w", err)
		}
	}

	return os.Remove(statePath)
}
