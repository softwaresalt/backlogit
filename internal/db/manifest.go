package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var manifestLog = slog.With("component", "manifest")

// FileKind categorises a file in the .backlogit workspace by its functional role.
type FileKind int

const (
	// FileKindArtifact covers Markdown artifact files under queue/, done/, active/, archive/.
	FileKindArtifact FileKind = iota
	// FileKindStash covers the stash.jsonl intake file.
	FileKindStash
	// FileKindLog covers per-item JSONL log files under logs/.
	FileKindLog
	// FileKindConfig covers workspace configuration files such as config.yaml.
	FileKindConfig
	// FileKindOther covers files that do not match any recognised category.
	FileKindOther
)

// artifactDirs are the top-level workspace directories that contain artifact .md files.
var artifactDirs = map[string]bool{
	"queue":   true,
	"done":    true,
	"active":  true,
	"blocked": true,
	"archive": true,
}

// configFiles are the known workspace configuration filenames.
var configFiles = map[string]bool{
	"config.yaml":     true,
	"registry.yaml":   true,
	"hooks.yaml":      true,
	"header-def.yaml": true,
}

// FileEntry records the stat metadata for a single file in the workspace manifest,
// keyed by its workspace-relative path.
type FileEntry struct {
	// RelPath is the workspace-relative path (e.g., "queue/037-F.md").
	RelPath string
	// Kind classifies the file's functional role.
	Kind FileKind
	// Size is the file size in bytes at the time the manifest was built.
	Size int64
	// ModTime is the file modification time at the time the manifest was built.
	ModTime time.Time
	// ItemID is the artifact ID extracted from frontmatter. Empty for non-artifact
	// kinds (stash, log, config, other).
	ItemID string
}

// RelocationEntry records a file that moved within the workspace. A relocation
// is detected when the same ItemID appears in both the deleted and added sets of
// a diff, indicating a path change (e.g., queue/ → done/) rather than a true
// delete-and-recreate.
type RelocationEntry struct {
	// ItemID is the artifact ID shared by the old and new paths.
	ItemID string
	// OldPath is the previous workspace-relative path.
	OldPath string
	// NewPath is the current workspace-relative path.
	NewPath string
	// Entry holds the current file metadata at NewPath.
	Entry FileEntry
}

// DiffResult holds the classified difference between two manifest snapshots.
type DiffResult struct {
	// Added holds files present in current but absent from old.
	Added []FileEntry
	// Changed holds files present in both snapshots but with differing mtime or size.
	Changed []FileEntry
	// Deleted holds files present in old but absent from current, excluding relocated files.
	Deleted []FileEntry
	// Relocated holds files whose ItemID appears in both a delete and an add,
	// indicating a path change rather than removal.
	Relocated []RelocationEntry
}

// ClassifyFile returns the FileKind for a workspace-relative path.
// Paths under queue/, done/, active/, blocked/, or archive/ with a .md extension
// are classified as FileKindArtifact. stash.jsonl is FileKindStash. Paths under
// logs/ with a .jsonl extension are FileKindLog. Known config filenames are
// FileKindConfig. All other paths return FileKindOther.
func ClassifyFile(relPath string) FileKind {
	// Normalise to forward slashes for consistent prefix matching.
	relPath = filepath.ToSlash(relPath)

	if relPath == "stash.jsonl" {
		return FileKindStash
	}

	if configFiles[relPath] {
		return FileKindConfig
	}

	if strings.HasPrefix(relPath, "logs/") && strings.HasSuffix(relPath, ".jsonl") {
		return FileKindLog
	}

	// Artifact: top-level dir is a known artifact directory and the file ends in .md.
	slash := strings.IndexByte(relPath, '/')
	if slash > 0 && strings.HasSuffix(relPath, ".md") {
		dir := relPath[:slash]
		if artifactDirs[dir] {
			return FileKindArtifact
		}
	}

	return FileKindOther
}

// ComputeDiff compares two manifest snapshots and returns a DiffResult that
// classifies each file change as added, changed, deleted, or relocated.
// Relocation detection matches ItemID values across delete and add pairs; a file
// that moved from one directory to another is reported as a single
// RelocationEntry rather than independent delete and add entries.
func ComputeDiff(old, current map[string]FileEntry) DiffResult {
	var result DiffResult

	// Index added entries by ItemID so we can match against deleted entries.
	addedByItemID := make(map[string]FileEntry, len(current))

	for path, curr := range current {
		if prev, exists := old[path]; exists {
			// Path present in both: check for content change via mtime or size.
			if !curr.ModTime.Equal(prev.ModTime) || curr.Size != prev.Size {
				result.Changed = append(result.Changed, curr)
			}
			// Unchanged files are silently skipped.
		} else {
			result.Added = append(result.Added, curr)
			if curr.ItemID != "" {
				addedByItemID[curr.ItemID] = curr
			}
		}
	}

	// Index deleted entries by ItemID for relocation matching.
	deletedByItemID := make(map[string]FileEntry, len(old))
	for path, prev := range old {
		if _, exists := current[path]; !exists {
			result.Deleted = append(result.Deleted, prev)
			if prev.ItemID != "" {
				deletedByItemID[prev.ItemID] = prev
			}
		}
	}

	// Promote (deleted, added) pairs sharing an ItemID into RelocationEntries.
	if len(deletedByItemID) == 0 || len(addedByItemID) == 0 {
		return result
	}

	relocatedIDs := make(map[string]bool)
	for id, addedEntry := range addedByItemID {
		if deletedEntry, matched := deletedByItemID[id]; matched {
			result.Relocated = append(result.Relocated, RelocationEntry{
				ItemID:  id,
				OldPath: deletedEntry.RelPath,
				NewPath: addedEntry.RelPath,
				Entry:   addedEntry,
			})
			relocatedIDs[id] = true
		}
	}

	if len(relocatedIDs) == 0 {
		return result
	}

	// Remove relocated entries from the Added and Deleted slices.
	filtered := result.Added[:0]
	for _, e := range result.Added {
		if !relocatedIDs[e.ItemID] {
			filtered = append(filtered, e)
		}
	}
	result.Added = filtered

	filtered = result.Deleted[:0]
	for _, e := range result.Deleted {
		if !relocatedIDs[e.ItemID] {
			filtered = append(filtered, e)
		}
	}
	result.Deleted = filtered

	return result
}

// ShouldFallback reports whether the diff is large enough to warrant a full
// rehydrate instead of an incremental sync. Returns true and a descriptive
// reason string when either:
//   - the total changed-file count (added+changed+deleted+relocated) meets or exceeds maxChangedFiles, or
//   - manifestSize is non-zero, manifestSize exceeds total (the manifest is meaningfully
//     larger than the change set), and the changed-file count meets or exceeds 50% of manifestSize.
func ShouldFallback(diff DiffResult, manifestSize, maxChangedFiles int) (bool, string) {
	total := len(diff.Added) + len(diff.Changed) + len(diff.Deleted) + len(diff.Relocated)

	if total >= maxChangedFiles {
		return true, fmt.Sprintf("delta (%d files) meets or exceeds absolute threshold (%d)", total, maxChangedFiles)
	}

	if manifestSize > 0 && total > 1 && total*2 >= manifestSize {
		pct := (total * 100) / manifestSize
		return true, fmt.Sprintf("delta (%d files, %d%%) meets or exceeds 50%% of manifest size (%d)", total, pct, manifestSize)
	}

	return false, ""
}

// BuildManifest walks the workspace directory tree and builds an in-memory
// manifest keyed by workspace-relative path. Hidden directories (name starting
// with ".") are skipped. For artifact files, it extracts the ItemID from the
// first 512 bytes of frontmatter without a full parse. Files that cannot be
// stat'd or read are skipped with a slog.Warn entry.
func BuildManifest(workspacePath string) (map[string]FileEntry, error) {
	manifest := make(map[string]FileEntry)

	err := filepath.WalkDir(workspacePath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			manifestLog.Warn("skipping unreadable entry during manifest build", "path", path, "error", walkErr)
			return nil
		}
		if d.IsDir() {
			// Skip hidden directories (e.g., .git) but never skip the workspace
			// root itself, which may be a hidden directory (e.g., .backlogit).
			if path != workspacePath && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, relErr := filepath.Rel(workspacePath, path)
		if relErr != nil {
			manifestLog.Warn("skipping file with non-relative path", "path", path, "error", relErr)
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		info, statErr := d.Info()
		if statErr != nil {
			manifestLog.Warn("skipping file: cannot stat", "relPath", relPath, "error", statErr)
			return nil
		}

		kind := ClassifyFile(relPath)
		entry := FileEntry{
			RelPath: relPath,
			Kind:    kind,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}

		if kind == FileKindArtifact {
			entry.ItemID = extractItemIDFromFrontmatter(path)
		}

		manifest[relPath] = entry
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("build manifest: %w", err)
	}

	return manifest, nil
}

// extractItemIDFromFrontmatter reads up to 512 bytes from the file and extracts
// the `id:` value from YAML frontmatter without a full parse. Returns an empty
// string when the file does not begin with frontmatter or the id field is absent.
func extractItemIDFromFrontmatter(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close() //nolint:errcheck

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	content := string(buf[:n])

	if !strings.HasPrefix(content, "---") {
		return ""
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "id:") {
			val := strings.TrimSpace(strings.TrimPrefix(trimmed, "id:"))
			return strings.Trim(val, `"'`)
		}
	}
	return ""
}

// RehydrateWithManifest performs the same full rehydration as Rehydrate and
// additionally builds and returns a file manifest from the workspace. It is the
// canonical implementation; Rehydrate delegates to it and discards the returned
// manifest for backward compatibility.
func RehydrateWithManifest(ctx context.Context, workspacePath string, database *sql.DB) (int, map[string]FileEntry, error) {
	count, err := Rehydrate(ctx, workspacePath, database)
	if err != nil {
		return 0, nil, err
	}

	manifest, err := BuildManifest(workspacePath)
	if err != nil {
		return count, nil, fmt.Errorf("build manifest after rehydration: %w", err)
	}

	return count, manifest, nil
}
