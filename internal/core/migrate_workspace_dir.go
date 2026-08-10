package core

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// MigrateWorkspaceDirOptions configures the workspace directory migration.
type MigrateWorkspaceDirOptions struct {
	DryRun bool // preview only, no changes
}

// MigrateWorkspaceDirResult describes what was or would be moved.
type MigrateWorkspaceDirResult struct {
	Source      string
	Destination string
	DryRun      bool
	AlreadyDone bool
	Files       []string
}

// MigrateWorkspaceDir renames the legacy .backlogit storage directory to the
// new .backlog default. It performs a pure move with no content rewrites.
func MigrateWorkspaceDir(rootPath string, opts MigrateWorkspaceDirOptions) (*MigrateWorkspaceDirResult, error) {
	cleanRoot, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}

	source := filepath.Join(cleanRoot, workspaceRootCandidates[1])
	destination := filepath.Join(cleanRoot, workspaceRootCandidates[0])
	result := &MigrateWorkspaceDirResult{
		Source:      source,
		Destination: destination,
		DryRun:      opts.DryRun,
	}

	destExists, err := existingDirectory(destination)
	if err != nil {
		return nil, fmt.Errorf("stat destination workspace dir: %w", err)
	}
	sourceExists, err := existingDirectory(source)
	if err != nil {
		return nil, fmt.Errorf("stat source workspace dir: %w", err)
	}

	if destExists && sourceExists {
		return nil, fmt.Errorf("workspace roots %s and %s both exist", source, destination)
	}
	if destExists {
		files, err := listWorkspaceDirFiles(destination)
		if err != nil {
			return nil, fmt.Errorf("list existing destination files: %w", err)
		}
		result.AlreadyDone = true
		result.Files = files
		return result, nil
	}
	if !sourceExists {
		return result, nil
	}

	files, err := listWorkspaceDirFiles(source)
	if err != nil {
		return nil, fmt.Errorf("list source files: %w", err)
	}
	result.Files = files
	if opts.DryRun {
		return result, nil
	}

	if err := moveWorkspaceDir(cleanRoot, source, destination); err != nil {
		return nil, err
	}

	return result, nil
}

func moveWorkspaceDir(rootPath, source, destination string) error {
	sourceRel, sourceRelErr := filepath.Rel(rootPath, source)
	destRel, destRelErr := filepath.Rel(rootPath, destination)
	if sourceRelErr == nil && destRelErr == nil &&
		!strings.HasPrefix(sourceRel, "..") && !strings.HasPrefix(destRel, "..") {
		if err := gitMoveWorkspaceDir(rootPath, sourceRel, destRel); err == nil {
			return nil
		}
	}

	if err := os.Rename(source, destination); err != nil {
		return fmt.Errorf("rename workspace dir %s -> %s: %w", source, destination, err)
	}
	return nil
}

func gitMoveWorkspaceDir(rootPath, sourceRel, destRel string) error {
	cmd := exec.Command("git", "-C", rootPath, "mv", sourceRel, destRel)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git mv %s -> %s: %w: %s", sourceRel, destRel, err, strings.TrimSpace(string(output)))
	}
	return nil
}

func existingDirectory(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	reparsePoint, inspectErr := isSymlinkOrReparse(info, path)
	if inspectErr != nil {
		return false, fmt.Errorf("inspect directory %s: %w", path, inspectErr)
	}
	if reparsePoint {
		return false, fmt.Errorf("%s is a symlink or reparse point; remove it before migrating", path)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("%s exists but is not a directory", path)
	}
	return true, nil
}

func listWorkspaceDirFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return fmt.Errorf("relativize %s: %w", path, err)
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}
