package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/models"
)

type artifactMigration struct {
	OldID        string
	NewID        string
	OldPath      string
	NewPath      string
	CurrentPath  string
	ArtifactType string
	Title        string
	Frontmatter  map[string]any
	Body         string
	Depth        int
	TypeConfig   *config.ArtifactTypeConfig
}

type rewriteContext struct {
	idMap          map[string]string
	pathByID       map[string]string
	suffixByPrefix map[string]string
	sortedIDs      []string
}

var (
	legacyIDPattern    = regexp.MustCompile(`^[A-Z]+[0-9]{3}(?:\.[A-Z]+[0-9]{3})*$`)
	currentIDPattern   = regexp.MustCompile(`^[0-9]{3}(?:\.[0-9]{3})*(?:-[A-Z]+)?$`)
	legacyStemPattern  = regexp.MustCompile(`^[A-Z]+[0-9]{3}(?:\.[A-Z]+[0-9]{3})*`)
	currentStemPattern = regexp.MustCompile(`^[0-9]{3}(?:\.[0-9]{3})*(?:-[A-Z]+)?`)
	legacyTokenPattern = regexp.MustCompile(`[A-Z]+[0-9]{3}(?:\.[A-Z]+[0-9]{3})*`)
)

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

	artifacts, rewriteCtx, err := buildMigrationPlan(root, cfg)
	if err != nil {
		return err
	}
	if err := applyArtifactMigration(artifacts, rewriteCtx, cfg.MaxSlugLength); err != nil {
		return err
	}
	if err := applyStashMigration(filepath.Join(workspaceDir, "queue", ".stash.md"), rewriteCtx); err != nil {
		return err
	}
	if err := applyStashJSONLMigration(filepath.Join(workspaceDir, "stash.jsonl"), rewriteCtx); err != nil {
		return err
	}
	if err := applyLogMigration(core.WorkspaceLogsRoot(root), rewriteCtx); err != nil {
		return err
	}
	return nil
}

func buildMigrationPlan(root string, cfg *config.WorkspaceConfig) ([]artifactMigration, *rewriteContext, error) {
	paths, err := collectArtifactPaths(filepath.Join(root, ".backlogit", "queue"), filepath.Join(root, ".backlogit", "archive"))
	if err != nil {
		return nil, nil, err
	}

	artifacts := make([]artifactMigration, 0, len(paths))
	mapping := make(map[string]string, len(paths))
	reverse := make(map[string]string, len(paths))
	pathByID := make(map[string]string, len(paths))
	suffixByPrefix := make(map[string]string, len(cfg.ArtifactTypes))
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
		suffixByPrefix[typeCfg.Prefix] = typeCfg.Suffix
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
		pathByID[newID] = normalizeWorkspacePath(root, newPath)
		artifacts = append(artifacts, artifactMigration{
			OldID:        oldID,
			NewID:        newID,
			OldPath:      path,
			NewPath:      newPath,
			CurrentPath:  normalizeWorkspacePath(root, newPath),
			ArtifactType: artifactType,
			Title:        title,
			Frontmatter:  fm,
			Body:         body,
			Depth:        strings.Count(oldID, ".") + 1,
			TypeConfig:   typeCfg,
		})
	}

	sort.SliceStable(artifacts, func(i, j int) bool {
		if artifacts[i].Depth == artifacts[j].Depth {
			return artifacts[i].OldPath < artifacts[j].OldPath
		}
		return artifacts[i].Depth < artifacts[j].Depth
	})

	sortedIDs := make([]string, 0, len(mapping))
	for oldID := range mapping {
		sortedIDs = append(sortedIDs, oldID)
	}
	sort.SliceStable(sortedIDs, func(i, j int) bool {
		if len(sortedIDs[i]) == len(sortedIDs[j]) {
			return sortedIDs[i] < sortedIDs[j]
		}
		return len(sortedIDs[i]) > len(sortedIDs[j])
	})

	return artifacts, &rewriteContext{
		idMap:          mapping,
		pathByID:       pathByID,
		suffixByPrefix: suffixByPrefix,
		sortedIDs:      sortedIDs,
	}, nil
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

func applyArtifactMigration(artifacts []artifactMigration, rewriteCtx *rewriteContext, maxSlugLength int) error {
	for _, artifact := range artifacts {
		fm := cloneMap(artifact.Frontmatter)
		fm["id"] = artifact.NewID
		for key, value := range fm {
			switch key {
			case "id":
				continue
			case "parent_id":
				if parentID, ok := value.(string); ok {
					if normalized, ok := normalizeArtifactID(parentID, rewriteCtx); ok {
						fm[key] = normalized
					}
				}
			case "dependencies":
				fm[key] = rewriteIDSlice(value, rewriteCtx)
			case "references":
				fm[key] = rewriteReferenceSlice(value, rewriteCtx)
			case "archived_from":
				if archivedFrom, ok := value.(string); ok && archivedFrom != "" {
					fm[key] = rewriteArchivedFromPath(archivedFrom, artifact, maxSlugLength)
				}
			default:
				fm[key] = rewriteGenericValue(value, rewriteCtx)
			}
		}

		content := models.SerializeFrontmatter(fm, rewriteMappedText(artifact.Body, rewriteCtx))
		if err := writeMigratedFile(artifact.OldPath, artifact.NewPath, []byte(content)); err != nil {
			return fmt.Errorf("rewrite artifact %q: %w", artifact.OldID, err)
		}
	}
	return nil
}

func applyStashMigration(stashPath string, rewriteCtx *rewriteContext) error {
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
	rewrittenFM := cloneMap(fm)
	for key, value := range rewrittenFM {
		rewrittenFM[key] = rewriteGenericValue(value, rewriteCtx)
	}
	content := models.SerializeFrontmatter(rewrittenFM, rewriteMappedText(body, rewriteCtx))
	if err := writeMigratedFile(stashPath, stashPath, []byte(content)); err != nil {
		return fmt.Errorf("rewrite stash %q: %w", stashPath, err)
	}
	return nil
}

func applyStashJSONLMigration(jsonlPath string, rewriteCtx *rewriteContext) error {
	if _, err := os.Stat(jsonlPath); os.IsNotExist(err) {
		return nil
	}

	raw, err := os.ReadFile(jsonlPath)
	if err != nil {
		return fmt.Errorf("read stash jsonl %q: %w", jsonlPath, err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\r\n"), "\n")
	rewritten := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return fmt.Errorf("parse stash jsonl line in %q: %w", jsonlPath, err)
		}
		entry = rewriteLogLikeMap(entry, rewriteCtx)
		data, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("marshal stash jsonl line in %q: %w", jsonlPath, err)
		}
		rewritten = append(rewritten, string(data))
	}
	content := strings.Join(rewritten, "\n")
	if content != "" {
		content += "\n"
	}
	if err := writeMigratedFile(jsonlPath, jsonlPath, []byte(content)); err != nil {
		return fmt.Errorf("rewrite stash jsonl %q: %w", jsonlPath, err)
	}
	return nil
}

func applyLogMigration(logsDir string, rewriteCtx *rewriteContext) error {
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
			entry = rewriteLogLikeMap(entry, rewriteCtx)
			data, err := json.Marshal(entry)
			if err != nil {
				return fmt.Errorf("marshal log line in %q: %w", path, err)
			}
			rewritten = append(rewritten, string(data))
		}

		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		newStem := stem
		if mapped, ok := normalizeArtifactID(stem, rewriteCtx); ok {
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

// rewriteIDSlice rewrites artifact IDs in a dependency list. It handles both
// the legacy bare-string format and the typed-object format (F4).
func rewriteIDSlice(value any, rewriteCtx *rewriteContext) any {
	iface, ok := value.([]any)
	if !ok {
		values := toStringSlice(value)
		rewritten := make([]string, 0, len(values))
		for _, item := range values {
			if mapped, mapOK := normalizeArtifactID(item, rewriteCtx); mapOK {
				rewritten = append(rewritten, mapped)
			} else {
				rewritten = append(rewritten, item)
			}
		}
		return rewritten
	}
	hasObjects := false
	for _, elem := range iface {
		if _, isStr := elem.(string); !isStr {
			hasObjects = true
			break
		}
	}
	if !hasObjects {
		values := toStringSlice(value)
		rewritten := make([]string, 0, len(values))
		for _, item := range values {
			if mapped, mapOK := normalizeArtifactID(item, rewriteCtx); mapOK {
				rewritten = append(rewritten, mapped)
			} else {
				rewritten = append(rewritten, item)
			}
		}
		return rewritten
	}
	rewritten := make([]any, 0, len(iface))
	for _, elem := range iface {
		switch entry := elem.(type) {
		case string:
			if mapped, mapOK := normalizeArtifactID(entry, rewriteCtx); mapOK {
				rewritten = append(rewritten, mapped)
			} else {
				rewritten = append(rewritten, entry)
			}
		case map[string]any:
			newEntry := make(map[string]any, len(entry))
			for k, v := range entry {
				newEntry[k] = v
			}
			if idStr, idOK := entry["id"].(string); idOK {
				if mapped, mapOK := normalizeArtifactID(idStr, rewriteCtx); mapOK {
					newEntry["id"] = mapped
				}
			}
			rewritten = append(rewritten, newEntry)
		default:
			rewritten = append(rewritten, elem)
		}
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

func rewriteGenericValue(value any, rewriteCtx *rewriteContext) any {
	switch v := value.(type) {
	case string:
		return rewriteMappedText(v, rewriteCtx)
	case []string:
		rewritten := make([]string, len(v))
		for i, item := range v {
			rewritten[i] = rewriteMappedText(item, rewriteCtx)
		}
		return rewritten
	case []any:
		rewritten := make([]any, len(v))
		for i, item := range v {
			rewritten[i] = rewriteGenericValue(item, rewriteCtx)
		}
		return rewritten
	case map[string]any:
		rewritten := cloneMap(v)
		for key, item := range rewritten {
			rewritten[key] = rewriteGenericValue(item, rewriteCtx)
		}
		return rewritten
	default:
		return value
	}
}

func rewriteReferenceSlice(value any, rewriteCtx *rewriteContext) []string {
	values := toStringSlice(value)
	rewritten := make([]string, 0, len(values))
	for _, item := range values {
		rewritten = append(rewritten, resolveArtifactPathReference(item, rewriteCtx))
	}
	return rewritten
}

func rewriteArchivedFromPath(original string, artifact artifactMigration, maxSlugLength int) string {
	normalized := normalizeMetadataPath(original)
	if normalized == "" {
		return normalized
	}
	dir := path.Dir(normalized)
	if dir == "." {
		dir = ""
	}
	fileName := core.ResolveFileName(artifact.TypeConfig, artifact.NewID, artifact.Title, maxSlugLength) + filepath.Ext(artifact.NewPath)
	fileName = filepath.ToSlash(fileName)
	if dir == "" {
		return fileName
	}
	return path.Join(dir, fileName)
}

func rewriteLogLikeMap(entry map[string]any, rewriteCtx *rewriteContext) map[string]any {
	rewritten := cloneMap(entry)
	for key, value := range rewritten {
		rewritten[key] = rewriteLogLikeValue(key, value, rewriteCtx)
	}
	return rewritten
}

func rewriteLogLikeValue(key string, value any, rewriteCtx *rewriteContext) any {
	switch v := value.(type) {
	case string:
		if strings.HasSuffix(key, "_path") {
			return resolveArtifactPathReference(v, rewriteCtx)
		}
		if strings.HasSuffix(key, "_id") {
			if normalized, ok := normalizeArtifactID(v, rewriteCtx); ok {
				return normalized
			}
		}
		return rewriteMappedText(v, rewriteCtx)
	case []string:
		rewritten := make([]string, len(v))
		for i, item := range v {
			if strings.HasSuffix(key, "_path") {
				rewritten[i] = resolveArtifactPathReference(item, rewriteCtx)
				continue
			}
			rewritten[i] = rewriteMappedText(item, rewriteCtx)
		}
		return rewritten
	case []any:
		rewritten := make([]any, len(v))
		for i, item := range v {
			rewritten[i] = rewriteLogLikeValue(key, item, rewriteCtx)
		}
		return rewritten
	case map[string]any:
		return rewriteLogLikeMap(v, rewriteCtx)
	default:
		return value
	}
}

func rewriteMappedText(text string, rewriteCtx *rewriteContext) string {
	if text == "" || rewriteCtx == nil {
		return text
	}

	rewritten := text
	for _, oldID := range rewriteCtx.sortedIDs {
		pattern := regexp.MustCompile(`(^|[^A-Za-z0-9])(` + regexp.QuoteMeta(oldID) + `)($|[^A-Za-z0-9])`)
		rewritten = pattern.ReplaceAllString(rewritten, `${1}`+rewriteCtx.idMap[oldID]+`${3}`)
	}
	return legacyTokenPattern.ReplaceAllStringFunc(rewritten, func(match string) string {
		if normalized, ok := normalizeArtifactID(match, rewriteCtx); ok {
			return normalized
		}
		return match
	})
}

func normalizeArtifactID(value string, rewriteCtx *rewriteContext) (string, bool) {
	if value == "" || rewriteCtx == nil {
		return "", false
	}
	if mapped, ok := rewriteCtx.idMap[value]; ok {
		return mapped, true
	}
	if currentIDPattern.MatchString(value) {
		return value, true
	}
	if !legacyIDPattern.MatchString(value) {
		return "", false
	}
	last := value
	if idx := strings.LastIndex(value, "."); idx >= 0 {
		last = value[idx+1:]
	}
	prefix := leadingLetters(last)
	suffix, ok := rewriteCtx.suffixByPrefix[prefix]
	if !ok || suffix == "" {
		return "", false
	}
	normalized, err := migrateArtifactID(value, suffix)
	if err != nil {
		return "", false
	}
	return normalized, true
}

func resolveArtifactPathReference(value string, rewriteCtx *rewriteContext) string {
	normalized := normalizeMetadataPath(value)
	if normalized == "" || rewriteCtx == nil {
		return normalized
	}
	stem := strings.TrimSuffix(path.Base(normalized), path.Ext(normalized))
	if artifactID, ok := extractArtifactIDFromStem(stem, rewriteCtx); ok {
		if currentPath, found := rewriteCtx.pathByID[artifactID]; found {
			return currentPath
		}
	}
	return normalized
}

func extractArtifactIDFromStem(stem string, rewriteCtx *rewriteContext) (string, bool) {
	if match := currentStemPattern.FindString(stem); match != "" {
		if normalized, ok := normalizeArtifactID(match, rewriteCtx); ok {
			return normalized, true
		}
	}
	if match := legacyStemPattern.FindString(stem); match != "" {
		if normalized, ok := normalizeArtifactID(match, rewriteCtx); ok {
			return normalized, true
		}
	}
	return "", false
}

func normalizeMetadataPath(value string) string {
	if value == "" {
		return ""
	}
	return path.Clean(strings.ReplaceAll(value, "\\", "/"))
}

func normalizeWorkspacePath(root string, value string) string {
	rel, err := filepath.Rel(root, value)
	if err != nil {
		return filepath.ToSlash(filepath.Clean(value))
	}
	return filepath.ToSlash(rel)
}

func leadingLetters(value string) string {
	end := 0
	for end < len(value) {
		ch := value[end]
		if (ch < 'A' || ch > 'Z') && (ch < 'a' || ch > 'z') {
			break
		}
		end++
	}
	return value[:end]
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
