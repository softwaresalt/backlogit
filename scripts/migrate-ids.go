package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/models"
)

type artifactMigration struct {
	OldID        string
	NewID        string
	OldPath      string
	NewPath      string
	ArtifactType string
	Title        string
	Frontmatter  map[string]any
	Body         string
	Depth        int
}

func main() {
	workspaceRoot := flag.String("workspace", ".", "Workspace root containing .backlogit/")
	flag.Parse()

	if err := runMigration(*workspaceRoot); err != nil {
		fmt.Fprintf(os.Stderr, "migrate ids: %v\n", err)
		os.Exit(1)
	}
}

func runMigration(root string) error {
	workspaceDir := filepath.Join(root, ".backlogit")
	cfg, err := config.Load(context.Background(), workspaceDir)
	if err != nil {
		return fmt.Errorf("load workspace config: %w", err)
	}

	artifacts, mapping, err := buildMigrationPlan(root, cfg)
	if err != nil {
		return err
	}
	if err := applyArtifactMigration(artifacts, mapping); err != nil {
		return err
	}
	if err := applyStashMigration(filepath.Join(workspaceDir, "queue", ".stash.md"), mapping); err != nil {
		return err
	}
	if err := applyLogMigration(core.WorkspaceLogsRoot(root), mapping); err != nil {
		return err
	}
	return nil
}

func buildMigrationPlan(root string, cfg *config.WorkspaceConfig) ([]artifactMigration, map[string]string, error) {
	paths, err := collectArtifactPaths(filepath.Join(root, ".backlogit", "queue"), filepath.Join(root, ".backlogit", "archive"))
	if err != nil {
		return nil, nil, err
	}

	artifacts := make([]artifactMigration, 0, len(paths))
	mapping := make(map[string]string, len(paths))
	reverse := make(map[string]string, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read artifact %q: %w", path, err)
		}
		fm, body, err := models.ParseFrontmatter(string(raw))
		if err != nil {
			return nil, nil, fmt.Errorf("parse artifact %q: %w", path, err)
		}
		if fm == nil {
			return nil, nil, fmt.Errorf("artifact %q is missing frontmatter", path)
		}

		oldID, _ := fm["id"].(string)
		artifactType, _ := fm["artifact_type"].(string)
		title, _ := fm["title"].(string)
		if oldID == "" || artifactType == "" {
			return nil, nil, fmt.Errorf("artifact %q is missing id or artifact_type", path)
		}

		typeCfg := cfg.ArtifactTypes[artifactType]
		if typeCfg == nil {
			return nil, nil, fmt.Errorf("artifact %q uses unknown type %q", path, artifactType)
		}
		newID, err := migrateArtifactID(oldID, typeCfg.Suffix)
		if err != nil {
			return nil, nil, fmt.Errorf("migrate id %q: %w", oldID, err)
		}

		newStem := core.ResolveFileName(typeCfg, newID, title, cfg.MaxSlugLength)
		newPath := filepath.Join(filepath.Dir(path), newStem+filepath.Ext(path))
		if existing, ok := reverse[newID]; ok && existing != oldID {
			return nil, nil, fmt.Errorf("migration would collide %q and %q at %q", existing, oldID, newID)
		}
		reverse[newID] = oldID
		mapping[oldID] = newID
		artifacts = append(artifacts, artifactMigration{
			OldID:        oldID,
			NewID:        newID,
			OldPath:      path,
			NewPath:      newPath,
			ArtifactType: artifactType,
			Title:        title,
			Frontmatter:  fm,
			Body:         body,
			Depth:        strings.Count(oldID, ".") + 1,
		})
	}

	sort.SliceStable(artifacts, func(i, j int) bool {
		if artifacts[i].Depth == artifacts[j].Depth {
			return artifacts[i].OldPath < artifacts[j].OldPath
		}
		return artifacts[i].Depth < artifacts[j].Depth
	})
	return artifacts, mapping, nil
}

func collectArtifactPaths(roots ...string) ([]string, error) {
	var paths []string
	for _, root := range roots {
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if filepath.Ext(path) != ".md" || filepath.Base(path) == ".stash.md" {
				return nil
			}
			paths = append(paths, path)
			return nil
		}); err != nil {
			return nil, fmt.Errorf("walk %q: %w", root, err)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func applyArtifactMigration(artifacts []artifactMigration, mapping map[string]string) error {
	for _, artifact := range artifacts {
		fm := rewriteMapStringValues(cloneMap(artifact.Frontmatter), mapping)
		fm["id"] = artifact.NewID

		if parentID, ok := fm["parent_id"].(string); ok && parentID != "" {
			if mapped, found := mapping[parentID]; found {
				fm["parent_id"] = mapped
			}
		}
		if dependencies, ok := fm["dependencies"]; ok {
			fm["dependencies"] = rewriteStringSlice(dependencies, mapping)
		}
		if archivedFrom, ok := fm["archived_from"].(string); ok && archivedFrom != "" {
			fm["archived_from"] = filepath.Join(filepath.Dir(archivedFrom), filepath.Base(artifact.NewPath))
		}

		content := models.SerializeFrontmatter(fm, rewriteMappedText(artifact.Body, mapping))
		if err := writeMigratedFile(artifact.OldPath, artifact.NewPath, []byte(content)); err != nil {
			return fmt.Errorf("rewrite artifact %q: %w", artifact.OldID, err)
		}
	}
	return nil
}

func applyStashMigration(stashPath string, mapping map[string]string) error {
	if _, err := os.Stat(stashPath); os.IsNotExist(err) {
		return nil
	}

	raw, err := os.ReadFile(stashPath)
	if err != nil {
		return fmt.Errorf("read stash %q: %w", stashPath, err)
	}
	fm, body, err := models.ParseFrontmatter(string(raw))
	if err != nil {
		return fmt.Errorf("parse stash %q: %w", stashPath, err)
	}
	content := models.SerializeFrontmatter(rewriteMapStringValues(cloneMap(fm), mapping), rewriteMappedText(body, mapping))
	if err := writeMigratedFile(stashPath, stashPath, []byte(content)); err != nil {
		return fmt.Errorf("rewrite stash %q: %w", stashPath, err)
	}
	return nil
}

func applyLogMigration(logsDir string, mapping map[string]string) error {
	if _, err := os.Stat(logsDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.WalkDir(logsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read log %q: %w", path, err)
		}
		lines := strings.Split(strings.TrimRight(string(raw), "\r\n"), "\n")
		rewritten := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var entry map[string]any
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				return fmt.Errorf("parse log line in %q: %w", path, err)
			}
			if itemID, ok := entry["item_id"].(string); ok {
				if mapped, found := mapping[itemID]; found {
					entry["item_id"] = mapped
				}
			}
			data, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("marshal log line in %q: %w", path, err)
			}
			rewritten = append(rewritten, string(data))
		}

		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		newStem := stem
		if mapped, found := mapping[stem]; found {
			newStem = mapped
		}
		newPath := filepath.Join(filepath.Dir(path), newStem+filepath.Ext(path))
		content := strings.Join(rewritten, "\n")
		if content != "" {
			content += "\n"
		}
		if err := writeMigratedFile(path, newPath, []byte(content)); err != nil {
			return fmt.Errorf("rewrite log %q: %w", path, err)
		}
		return nil
	})
}

func migrateArtifactID(artifactID string, suffix string) (string, error) {
	if artifactID == "" {
		return "", fmt.Errorf("artifact id is required")
	}
	if suffix == "" {
		return "", fmt.Errorf("artifact suffix is required")
	}
	if isMigratedID(artifactID, suffix) {
		return artifactID, nil
	}

	parts := strings.Split(artifactID, ".")
	numericParts := make([]string, len(parts))
	for i, part := range parts {
		numeric, err := trailingOrdinal(part)
		if err != nil {
			return "", err
		}
		numericParts[i] = numeric
	}
	numericParts[len(numericParts)-1] += suffix
	return strings.Join(numericParts, "."), nil
}

func isMigratedID(artifactID string, suffix string) bool {
	parts := strings.Split(artifactID, ".")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts[:len(parts)-1] {
		if !isDigits(part) {
			return false
		}
	}
	last := parts[len(parts)-1]
	if !strings.HasSuffix(last, suffix) {
		return false
	}
	return isDigits(strings.TrimSuffix(last, suffix))
}

func trailingOrdinal(segment string) (string, error) {
	end := len(segment)
	start := end
	for start > 0 {
		ch := segment[start-1]
		if ch < '0' || ch > '9' {
			break
		}
		start--
	}
	if start == end {
		return "", fmt.Errorf("segment %q has no trailing ordinal", segment)
	}
	ordinal, err := strconv.Atoi(segment[start:end])
	if err != nil {
		return "", fmt.Errorf("parse ordinal from %q: %w", segment, err)
	}
	return fmt.Sprintf("%03d", ordinal), nil
}

func rewriteStringSlice(value any, mapping map[string]string) []string {
	values := toStringSlice(value)
	rewritten := make([]string, 0, len(values))
	for _, item := range values {
		if mapped, found := mapping[item]; found {
			rewritten = append(rewritten, mapped)
			continue
		}
		rewritten = append(rewritten, item)
	}
	return rewritten
}

func toStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func writeMigratedFile(oldPath string, newPath string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}

	tmpPath := newPath + ".tmp"
	if err := os.WriteFile(tmpPath, content, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if samePath(oldPath, newPath) {
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			_ = os.Remove(tmpPath)
			return fmt.Errorf("remove original before replace: %w", err)
		}
	}
	if err := os.Rename(tmpPath, newPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename temp file: %w", err)
	}
	if !samePath(oldPath, newPath) {
		if err := os.Remove(oldPath); err != nil {
			return fmt.Errorf("remove original file: %w", err)
		}
	}
	return nil
}

func samePath(left string, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

func rewriteMapStringValues(input map[string]any, mapping map[string]string) map[string]any {
	cloned := cloneMap(input)
	for key, value := range cloned {
		cloned[key] = rewriteMappedValue(value, mapping)
	}
	return cloned
}

func rewriteMappedValue(value any, mapping map[string]string) any {
	switch v := value.(type) {
	case string:
		return rewriteMappedText(v, mapping)
	case []string:
		rewritten := make([]string, len(v))
		for i, item := range v {
			rewritten[i] = rewriteMappedText(item, mapping)
		}
		return rewritten
	case []any:
		rewritten := make([]any, len(v))
		for i, item := range v {
			rewritten[i] = rewriteMappedValue(item, mapping)
		}
		return rewritten
	case map[string]any:
		return rewriteMapStringValues(v, mapping)
	default:
		return value
	}
}

func rewriteMappedText(text string, mapping map[string]string) string {
	if text == "" || len(mapping) == 0 {
		return text
	}

	keys := make([]string, 0, len(mapping))
	for oldID := range mapping {
		keys = append(keys, oldID)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})

	rewritten := text
	for _, oldID := range keys {
		pattern := regexp.MustCompile(`(^|[^A-Za-z0-9])(` + regexp.QuoteMeta(oldID) + `)($|[^A-Za-z0-9])`)
		rewritten = pattern.ReplaceAllString(rewritten, `${1}`+mapping[oldID]+`${3}`)
	}
	return rewritten
}

func cloneMap(input map[string]any) map[string]any {
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}
