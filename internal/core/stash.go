package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/backlogit/backlogit/internal/db"
	corerrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/models"
	"github.com/backlogit/backlogit/internal/stash"
)

const (
	stashStateActive    = "active"
	stashStateHarvested = "harvested"
)

// FetchStashOptions controls stash fetch filtering and grouping.
type FetchStashOptions struct {
	Priority        string `json:"priority,omitempty"`
	GroupByPriority bool   `json:"group_by_priority,omitempty"`
	Limit           int    `json:"limit,omitempty"` // max entries to return (0 = no limit)
}

// HarvestStashOptions controls single-entry and batch priority harvest behavior.
type HarvestStashOptions struct {
	StashID      string `json:"stash_id,omitempty"`
	Priority     string `json:"priority,omitempty"`
	ArtifactType string `json:"artifact_type"`
	Title        string `json:"title,omitempty"`
	Description  string `json:"description,omitempty"`
	Status       string `json:"status,omitempty"`
	ParentID     string `json:"parent_id,omitempty"`
}

// StashEntryView describes a stash entry plus any linked deliberation artifact.
type StashEntryView struct {
	ID             string           `json:"id"`
	Priority       string           `json:"priority"`
	DeliberationID string           `json:"deliberation_id,omitempty"`
	Kind           string           `json:"kind"`
	Text           string           `json:"text"`
	Deliberation   *models.Artifact `json:"deliberation,omitempty"`
}

// FetchedStashResult describes fetched stash entries, with optional grouping.
type FetchedStashResult struct {
	Entries           []StashEntryView            `json:"entries"`
	GroupBy           string                      `json:"group_by,omitempty"`
	EntriesByPriority map[string][]StashEntryView `json:"entries_by_priority,omitempty"`
}

// StashFilePath returns the canonical hidden stash file path for a workspace root.
func StashFilePath(rootPath string) string {
	return filepath.Join(WorkspaceStorageRoot(rootPath), stash.JSONLFileName)
}

func legacyStashFilePath(rootPath string) string {
	return filepath.Join(WorkspaceStorageRoot(rootPath), "queue", stash.FileName)
}

// EnsureStashFile creates the canonical stash storage if it does not already exist.
func EnsureStashFile(rootPath string) error {
	path := StashFilePath(rootPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create stash dir: %w", err)
	}

	jsonlEntries := make([]stash.Entry, 0)
	if _, err := os.Stat(path); err == nil {
		entries, err := readStashEntries(path)
		if err != nil {
			return err
		}
		jsonlEntries = entries
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat stash file: %w", err)
	}

	legacyPath := legacyStashFilePath(rootPath)
	legacyExists := false
	if _, err := os.Stat(legacyPath); err == nil {
		legacyExists = true
		_, legacyEntries, err := stash.ParseFile(legacyPath)
		if err != nil {
			return err
		}
		jsonlEntries = mergeStashEntries(jsonlEntries, legacyEntries)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat legacy stash file: %w", err)
	}

	if err := writeStashEntries(path, jsonlEntries); err != nil {
		return err
	}
	if legacyExists {
		if err := os.Remove(legacyPath); err != nil {
			return fmt.Errorf("remove legacy stash file: %w", err)
		}
	}
	return nil
}

// FetchStash returns the currently active stash entries.
func FetchStash(ctx context.Context, ws *Workspace, opts FetchStashOptions) (*FetchedStashResult, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if err := EnsureStashFile(ws.RootPath); err != nil {
		return nil, err
	}
	entries, err := readStashEntries(StashFilePath(ws.RootPath))
	if err != nil {
		return nil, err
	}
	if opts.Priority != "" {
		priority, err := stash.NormalizePriority(opts.Priority)
		if err != nil {
			return nil, err
		}
		filtered := make([]stash.Entry, 0, len(entries))
		for _, entry := range entries {
			if entry.Priority == priority {
				filtered = append(filtered, entry)
			}
		}
		entries = filtered
	}

	views, err := expandStashEntries(ctx, ws, entries)
	if err != nil {
		return nil, err
	}

	if opts.Limit > 0 && len(views) > opts.Limit {
		views = views[:opts.Limit]
	}

	result := &FetchedStashResult{Entries: views}
	if opts.GroupByPriority {
		result.GroupBy = "priority"
		grouped := make(map[string][]StashEntryView, len(stash.AllowedPriorities()))
		for _, priority := range stash.AllowedPriorities() {
			grouped[priority] = []StashEntryView{}
		}
		for _, entry := range views {
			grouped[entry.Priority] = append(grouped[entry.Priority], entry)
		}
		result.EntriesByPriority = grouped
	}
	return result, nil
}

// AddStashEntry appends a new active entry to the stash file and indexes it.
func AddStashEntry(ctx context.Context, ws *Workspace, kind, priority, text string) (*stash.Entry, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	normalizedKind, err := stash.NormalizeKind(kind)
	if err != nil {
		return nil, err
	}
	normalizedPriority, err := stash.NormalizePriority(priority)
	if err != nil {
		return nil, err
	}
	trimmedText := strings.TrimSpace(text)
	if trimmedText == "" {
		return nil, fmt.Errorf("stash text is required")
	}

	path := StashFilePath(ws.RootPath)
	unlock, err := lockStashFile(path)
	if err != nil {
		return nil, fmt.Errorf("acquire stash lock: %w", err)
	}
	defer func() { _ = unlock() }()

	// EnsureStashFile is called inside the lock because it also writes the file
	// atomically. Calling it outside would race with concurrent writers on Windows.
	if err := EnsureStashFile(ws.RootPath); err != nil {
		return nil, err
	}

	entries, err := readStashEntries(path)
	if err != nil {
		return nil, err
	}
	existingIDs := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		existingIDs[entry.ID] = struct{}{}
	}
	id, err := stash.GenerateID(existingIDs)
	if err != nil {
		return nil, err
	}
	entry := stash.Entry{ID: id, Priority: normalizedPriority, Kind: normalizedKind, Text: trimmedText}
	entries = append(entries, entry)
	if err := writeStashEntries(path, entries); err != nil {
		return nil, err
	}
	if ws.DB != nil {
		if err := db.UpsertStashEntry(ctx, ws.DB, entry.ID, entry.Priority, entry.Kind, entry.Text, entry.DeliberationID, stashStateActive, stashRelativePath(), time.Now().UTC()); err != nil {
			return nil, err
		}
	}
	return &entry, nil
}

// HarvestedStashResult describes a stash harvest operation.
type HarvestedStashResult struct {
	Entry    StashEntryView `json:"entry"`
	Artifact any            `json:"artifact"`
}

// HarvestedStashBatchResult describes a priority-based stash harvest operation.
type HarvestedStashBatchResult struct {
	Priority string                 `json:"priority"`
	Results  []HarvestedStashResult `json:"results"`
}

// HarvestStashEntry creates a backlogit artifact from a stash entry, links it,
// and removes the active stash line from the hidden stash file.
func HarvestStashEntry(ctx context.Context, ws *Workspace, harvestOpts HarvestStashOptions) (*HarvestedStashResult, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if strings.TrimSpace(harvestOpts.StashID) == "" {
		return nil, fmt.Errorf("stash id is required")
	}
	if strings.TrimSpace(harvestOpts.ArtifactType) == "" {
		return nil, fmt.Errorf("artifact type is required")
	}

	// Lock the stash file for the full read-modify-write cycle to prevent concurrent
	// harvests from reading the same entry and producing duplicate artifacts.
	path := StashFilePath(ws.RootPath)
	unlock, err := lockStashFile(path)
	if err != nil {
		return nil, fmt.Errorf("acquire stash lock: %w", err)
	}
	entry, remaining, err := removeStashEntry(ws.RootPath, harvestOpts.StashID)
	if err != nil {
		_ = unlock()
		return nil, err
	}
	// Rewrite the stash file inside the lock to prevent double-harvest if artifact
	// creation fails. The lock is released after the file is written.
	if err := writeStashEntries(path, remaining); err != nil {
		_ = unlock()
		return nil, fmt.Errorf("rewrite stash file: %w", err)
	}
	_ = unlock()

	itemTitle := strings.TrimSpace(harvestOpts.Title)
	if itemTitle == "" {
		itemTitle = entry.Text
	}
	fields := map[string]any{
		"source_stash_id":       entry.ID,
		"source_stash_priority": entry.Priority,
		"source_stash_kind":     entry.Kind,
		"source_stash_text":     entry.Text,
		"source_stash_path":     stashRelativePath(),
	}
	if entry.DeliberationID != "" {
		fields["source_deliberation_id"] = entry.DeliberationID
	}
	createOpts := []Option{WithFields(fields)}
	if entry.Priority != "" {
		createOpts = append(createOpts, WithPriority(entry.Priority))
	}
	if harvestOpts.Description != "" {
		createOpts = append(createOpts, WithDescription(harvestOpts.Description))
	}
	if harvestOpts.Status != "" {
		createOpts = append(createOpts, WithStatus(harvestOpts.Status))
	}
	if harvestOpts.ParentID != "" {
		createOpts = append(createOpts, WithParent(harvestOpts.ParentID))
	}

	artifact, err := CreateArtifact(ctx, ws, itemTitle, harvestOpts.ArtifactType, createOpts...)
	if err != nil {
		return nil, fmt.Errorf("create artifact from stash: %w", err)
	}
	if ws.DB != nil {
		if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
			return nil, fmt.Errorf("index harvested artifact: %w", err)
		}
	}

	if ws.DB != nil {
		now := time.Now().UTC()
		if err := db.UpsertStashEntry(ctx, ws.DB, entry.ID, entry.Priority, entry.Kind, entry.Text, entry.DeliberationID, stashStateHarvested, stashRelativePath(), now); err != nil {
			return nil, err
		}
		if err := db.LinkStashEntry(ctx, ws.DB, entry.ID, artifact.ID, now); err != nil {
			return nil, err
		}
	}

	entryView, err := expandStashEntry(ctx, ws, entry)
	if err != nil {
		return nil, err
	}
	return &HarvestedStashResult{Entry: entryView, Artifact: artifact}, nil
}

// HarvestStashByPriority harvests all active stash entries matching a priority.
func HarvestStashByPriority(ctx context.Context, ws *Workspace, opts HarvestStashOptions) (*HarvestedStashBatchResult, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	priority, err := stash.NormalizePriority(opts.Priority)
	if err != nil {
		return nil, err
	}
	fetched, err := FetchStash(ctx, ws, FetchStashOptions{Priority: priority})
	if err != nil {
		return nil, err
	}
	results := make([]HarvestedStashResult, 0, len(fetched.Entries))
	for _, entry := range fetched.Entries {
		result, err := HarvestStashEntry(ctx, ws, HarvestStashOptions{
			StashID:      entry.ID,
			ArtifactType: opts.ArtifactType,
			Title:        opts.Title,
			Description:  opts.Description,
			Status:       opts.Status,
			ParentID:     opts.ParentID,
		})
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}
	return &HarvestedStashBatchResult{Priority: priority, Results: results}, nil
}

func removeStashEntry(rootPath, stashID string) (stash.Entry, []stash.Entry, error) {
	if err := EnsureStashFile(rootPath); err != nil {
		return stash.Entry{}, nil, err
	}
	path := StashFilePath(rootPath)
	entries, err := readStashEntries(path)
	if err != nil {
		return stash.Entry{}, nil, err
	}
	needle := strings.ToUpper(strings.TrimSpace(stashID))
	remaining := make([]stash.Entry, 0, len(entries))
	var matched stash.Entry
	found := false
	for _, entry := range entries {
		if strings.EqualFold(entry.ID, needle) {
			matched = entry
			found = true
			continue
		}
		remaining = append(remaining, entry)
	}
	if !found {
		return stash.Entry{}, nil, fmt.Errorf("stash entry not found: %s: %w", stashID, corerrors.ErrNotFound)
	}
	return matched, remaining, nil
}

// GetStashEntry returns a single active stash entry enriched with any linked deliberation.
func GetStashEntry(ctx context.Context, ws *Workspace, stashID string) (*StashEntryView, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if err := EnsureStashFile(ws.RootPath); err != nil {
		return nil, err
	}
	entries, err := readStashEntries(StashFilePath(ws.RootPath))
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.ID, stashID) {
			view, err := expandStashEntry(ctx, ws, entry)
			if err != nil {
				return nil, err
			}
			return &view, nil
		}
	}
	return nil, fmt.Errorf("stash entry not found: %s: %w", stashID, corerrors.ErrNotFound)
}

// LinkDeliberationToStashEntrylinks an existing deliberation artifact to an active stash entry.
func LinkDeliberationToStashEntry(ctx context.Context, ws *Workspace, stashID, deliberationID string) (*StashEntryView, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	normalizedDeliberationID := strings.ToUpper(strings.TrimSpace(deliberationID))
	if normalizedDeliberationID == "" {
		return nil, fmt.Errorf("deliberation id is required")
	}
	path := StashFilePath(ws.RootPath)
	unlock, err := lockStashFile(path)
	if err != nil {
		return nil, fmt.Errorf("acquire stash lock: %w", err)
	}
	defer func() { _ = unlock() }()

	if err := EnsureStashFile(ws.RootPath); err != nil {
		return nil, err
	}
	entries, err := readStashEntries(path)
	if err != nil {
		return nil, err
	}
	found := false
	var updated stash.Entry
	for i := range entries {
		if !strings.EqualFold(entries[i].ID, stashID) {
			continue
		}
		found = true
		if existing := strings.ToUpper(strings.TrimSpace(entries[i].DeliberationID)); existing != "" && !strings.EqualFold(existing, normalizedDeliberationID) {
			return nil, fmt.Errorf("stash entry %s is already linked to deliberation %s: %w", entries[i].ID, existing, corerrors.ErrValidation)
		}
		entries[i].DeliberationID = normalizedDeliberationID
		updated = entries[i]
		break
	}
	if !found {
		return nil, fmt.Errorf("stash entry not found: %s: %w", stashID, corerrors.ErrNotFound)
	}
	if err := writeStashEntries(path, entries); err != nil {
		return nil, err
	}
	if ws.DB != nil {
		if err := db.UpsertStashEntry(ctx, ws.DB, updated.ID, updated.Priority, updated.Kind, updated.Text, updated.DeliberationID, stashStateActive, stashRelativePath(), time.Now().UTC()); err != nil {
			return nil, err
		}
	}
	view, err := expandStashEntry(ctx, ws, updated)
	if err != nil {
		return nil, err
	}
	return &view, nil
}

func writeStringAtomically(path, content string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write file %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename file %s: %w", path, err)
	}
	return nil
}

func stashRelativePath() string {
	return filepath.ToSlash(stash.JSONLFileName)
}

func readStashEntries(path string) ([]stash.Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open stash file: %w", err)
	}
	defer file.Close()

	entries, err := stash.ReadJSONL(file)
	if err != nil {
		return nil, fmt.Errorf("read stash jsonl: %w", err)
	}
	return entries, nil
}

func writeStashEntries(path string, entries []stash.Entry) error {
	var buf bytes.Buffer
	if err := stash.WriteJSONL(&buf, entries); err != nil {
		return fmt.Errorf("serialize stash entries: %w", err)
	}
	return writeStringAtomically(path, buf.String())
}

func mergeStashEntries(primary []stash.Entry, fallback []stash.Entry) []stash.Entry {
	merged := append([]stash.Entry(nil), primary...)
	seen := make(map[string]struct{}, len(primary))
	for _, entry := range primary {
		seen[strings.ToUpper(entry.ID)] = struct{}{}
	}
	for _, entry := range fallback {
		key := strings.ToUpper(entry.ID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, entry)
	}
	return merged
}

func expandStashEntries(ctx context.Context, ws *Workspace, entries []stash.Entry) ([]StashEntryView, error) {
	views := make([]StashEntryView, 0, len(entries))
	for _, entry := range entries {
		view, err := expandStashEntry(ctx, ws, entry)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func expandStashEntry(ctx context.Context, ws *Workspace, entry stash.Entry) (StashEntryView, error) {
	view := StashEntryView{
		ID:             entry.ID,
		Priority:       entry.Priority,
		DeliberationID: entry.DeliberationID,
		Kind:           entry.Kind,
		Text:           entry.Text,
	}
	if ws != nil && ws.DB != nil && entry.DeliberationID != "" {
		artifact, err := db.GetItem(ctx, ws.DB, entry.DeliberationID)
		if err == nil {
			view.Deliberation = artifact
		} else if !errors.Is(err, corerrors.ErrNotFound) {
			return StashEntryView{}, fmt.Errorf("load linked deliberation %s: %w", entry.DeliberationID, err)
		}
	}
	return view, nil
}
