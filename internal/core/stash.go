package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/backlogit/backlogit/internal/db"
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

// FetchedStashResult describes fetched stash entries, with optional grouping.
type FetchedStashResult struct {
	Entries           []stash.Entry            `json:"entries"`
	GroupBy           string                   `json:"group_by,omitempty"`
	EntriesByPriority map[string][]stash.Entry `json:"entries_by_priority,omitempty"`
}

// StashFilePath returns the canonical hidden stash file path for a workspace root.
func StashFilePath(rootPath string) string {
	return filepath.Join(WorkspaceStorageRoot(rootPath), "queue", stash.FileName)
}

// EnsureStashFile creates the hidden stash file if it does not already exist.
func EnsureStashFile(rootPath string) error {
	path := StashFilePath(rootPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create stash dir: %w", err)
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat stash file: %w", err)
	}
	return writeStringAtomically(path, stash.DefaultContent())
}

// FetchStash returns the currently active stash entries.
func FetchStash(_ context.Context, ws *Workspace, opts FetchStashOptions) (*FetchedStashResult, error) {
	if ws == nil {
		return nil, fmt.Errorf("workspace is required")
	}
	if err := EnsureStashFile(ws.RootPath); err != nil {
		return nil, err
	}
	_, entries, err := stash.ParseFile(StashFilePath(ws.RootPath))
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

	result := &FetchedStashResult{Entries: entries}
	if opts.GroupByPriority {
		result.GroupBy = "priority"
		grouped := make(map[string][]stash.Entry, len(stash.AllowedPriorities()))
		for _, priority := range stash.AllowedPriorities() {
			grouped[priority] = []stash.Entry{}
		}
		for _, entry := range entries {
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
	if err := EnsureStashFile(ws.RootPath); err != nil {
		return nil, err
	}

	path := StashFilePath(ws.RootPath)
	fm, entries, err := stash.ParseFile(path)
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
	content := stash.RenderContent(fm, entries)
	if err := writeStringAtomically(path, content); err != nil {
		return nil, err
	}
	if ws.DB != nil {
		if err := db.UpsertStashEntry(ctx, ws.DB, entry.ID, entry.Priority, entry.Kind, entry.Text, stashStateActive, stashRelativePath(), time.Now().UTC()); err != nil {
			return nil, err
		}
	}
	return &entry, nil
}

// HarvestedStashResult describes a stash harvest operation.
type HarvestedStashResult struct {
	Entry    stash.Entry `json:"entry"`
	Artifact any         `json:"artifact"`
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
	entry, remaining, fm, err := removeStashEntry(ws.RootPath, harvestOpts.StashID)
	if err != nil {
		return nil, err
	}

	itemTitle := strings.TrimSpace(harvestOpts.Title)
	if itemTitle == "" {
		itemTitle = entry.Text
	}
	createOpts := []Option{
		WithFields(map[string]any{
			"source_stash_id":       entry.ID,
			"source_stash_priority": entry.Priority,
			"source_stash_kind":     entry.Kind,
			"source_stash_text":     entry.Text,
		}),
	}
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
	if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
		return nil, fmt.Errorf("index harvested artifact: %w", err)
	}

	if err := writeStringAtomically(StashFilePath(ws.RootPath), stash.RenderContent(fm, remaining)); err != nil {
		return nil, err
	}
	if ws.DB != nil {
		now := time.Now().UTC()
		if err := db.UpsertStashEntry(ctx, ws.DB, entry.ID, entry.Priority, entry.Kind, entry.Text, stashStateHarvested, stashRelativePath(), now); err != nil {
			return nil, err
		}
		if err := db.LinkStashEntry(ctx, ws.DB, entry.ID, artifact.ID, now); err != nil {
			return nil, err
		}
	}

	return &HarvestedStashResult{Entry: entry, Artifact: artifact}, nil
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

func removeStashEntry(rootPath, stashID string) (stash.Entry, []stash.Entry, map[string]any, error) {
	if err := EnsureStashFile(rootPath); err != nil {
		return stash.Entry{}, nil, nil, err
	}
	path := StashFilePath(rootPath)
	fm, entries, err := stash.ParseFile(path)
	if err != nil {
		return stash.Entry{}, nil, nil, err
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
		return stash.Entry{}, nil, nil, fmt.Errorf("stash entry not found: %s", stashID)
	}
	return matched, remaining, fm, nil
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
	return filepath.ToSlash(filepath.Join("queue", stash.FileName))
}
