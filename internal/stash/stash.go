package stash

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	blErrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/models"
)

const (
	// FileName is the canonical hidden stash file name in the queue directory.
	FileName = ".stash.md"
	// DefaultTitle is the title used in the stash frontmatter.
	DefaultTitle = "Stash"
	// DefaultDescription describes the stash file purpose.
	DefaultDescription = "Candidate backlog ideas, issues, risks, and tasks for future planning"
)

var entryPattern = regexp.MustCompile(`(?i)^\s*-\s+\[\s*\]\s+\[([A-Z0-9]{1,8})\](?:\s+\[priority:([a-z]+)\])?(?:\s+\[deliberation:([A-Z0-9.]+)\])?\s+([a-z]+):\s+(.+?)\s*$`)

// Entry represents a single active stash item stored in the stash file.
type Entry struct {
	ID             string `json:"id"`
	Priority       string `json:"priority"`
	DeliberationID string `json:"deliberation_id,omitempty"`
	Kind           string `json:"kind"`
	Text           string `json:"text"`
}

var allowedKinds = []string{"feature", "task", "bug", "epic"}
var allowedPriorities = []string{"low", "medium", "high", "critical"}

const DefaultPriority = "medium"

// DefaultContent returns the default hidden stash markdown file contents.
func DefaultContent() string {
	return models.SerializeFrontmatter(defaultFrontmatter(), "## Stash\n")
}

// ParseFile reads and parses active stash entries from a stash markdown file.
func ParseFile(path string) (map[string]any, []Entry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read stash file: %w", err)
	}
	return ParseContent(string(raw))
}

// ParseContent parses stash frontmatter and active entries from markdown content.
func ParseContent(content string) (map[string]any, []Entry, error) {
	fm, body, err := models.ParseFrontmatter(content)
	if err != nil {
		return nil, nil, fmt.Errorf("parse stash frontmatter: %w", err)
	}
	if fm == nil {
		fm = defaultFrontmatter()
	}

	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	entries := make([]Entry, 0)
	for _, line := range lines {
		matches := entryPattern.FindStringSubmatch(line)
		if len(matches) != 6 {
			continue
		}
		priority, err := NormalizePriority(matches[2])
		if err != nil {
			slog.Warn("skipping stash entry: invalid priority", "line", line, "error", err)
			continue
		}
		kind, err := NormalizeKind(matches[4])
		if err != nil {
			slog.Warn("skipping stash entry: invalid kind", "line", line, "error", err)
			continue
		}
		entries = append(entries, Entry{
			ID:             strings.ToUpper(matches[1]),
			Priority:       priority,
			DeliberationID: strings.ToUpper(strings.TrimSpace(matches[3])),
			Kind:           kind,
			Text:           strings.TrimSpace(matches[5]),
		})
	}
	return fm, entries, nil
}

// RenderContent renders the stash markdown file from frontmatter and entries.
func RenderContent(frontmatter map[string]any, entries []Entry) string {
	fm := cloneFrontmatter(frontmatter)
	if fm == nil {
		fm = defaultFrontmatter()
	}
	if _, ok := fm["title"]; !ok {
		fm["title"] = DefaultTitle
	}
	if _, ok := fm["description"]; !ok {
		fm["description"] = DefaultDescription
	}
	fm["updated_at"] = time.Now().UTC()

	bodyLines := []string{"## Stash"}
	if len(entries) > 0 {
		bodyLines = append(bodyLines, "")
		for _, entry := range entries {
			bodyLines = append(bodyLines, FormatEntry(entry))
		}
	}
	body := strings.Join(bodyLines, "\n") + "\n"
	return models.SerializeFrontmatter(fm, body)
}

// FormatEntry renders a stash entry as a checkbox list line.
func FormatEntry(entry Entry) string {
	priority, err := NormalizePriority(entry.Priority)
	if err != nil {
		priority = DefaultPriority
	}
	line := fmt.Sprintf(
		"- [ ] [%s] [priority:%s]",
		strings.ToUpper(entry.ID),
		priority,
	)
	if deliberationID := strings.ToUpper(strings.TrimSpace(entry.DeliberationID)); deliberationID != "" {
		line += fmt.Sprintf(" [deliberation:%s]", deliberationID)
	}
	line += fmt.Sprintf(
		" %s: %s",
		strings.ToLower(entry.Kind),
		strings.TrimSpace(entry.Text),
	)
	return line
}

// GenerateID returns a unique uppercase alphanumeric stash ID with up to 8 characters.
func GenerateID(existingIDs map[string]struct{}) (string, error) {
	for range 32 {
		buf := make([]byte, 4)
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("generate stash id: %w", err)
		}
		id := strings.ToUpper(hex.EncodeToString(buf))
		if _, exists := existingIDs[id]; !exists {
			return id, nil
		}
	}
	return "", fmt.Errorf("generate stash id: exhausted retries")
}

// NormalizeKind validates and normalizes a stash item kind.
func NormalizeKind(kind string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(kind))
	switch normalized {
	case "feature", "task", "bug", "epic":
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported stash kind %q: %w", kind, blErrors.ErrValidation)
	}
}

// NormalizePriority validates and normalizes a stash priority.
func NormalizePriority(priority string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(priority))
	if normalized == "" {
		return DefaultPriority, nil
	}
	switch normalized {
	case "low", "medium", "high", "critical":
		return normalized, nil
	default:
		return "", fmt.Errorf("unsupported stash priority %q: %w", priority, blErrors.ErrValidation)
	}
}

// AllowedKinds returns the supported stash item kinds.
func AllowedKinds() []string {
	result := make([]string, len(allowedKinds))
	copy(result, allowedKinds)
	return result
}

// AllowedPriorities returns the supported stash priority levels.
func AllowedPriorities() []string {
	result := make([]string, len(allowedPriorities))
	copy(result, allowedPriorities)
	return result
}

// SortEntries orders entries deterministically by their appearance-friendly fields.
func SortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID < entries[j].ID
	})
}

func defaultFrontmatter() map[string]any {
	return map[string]any{
		"title":       DefaultTitle,
		"description": DefaultDescription,
	}
}

func cloneFrontmatter(frontmatter map[string]any) map[string]any {
	if frontmatter == nil {
		return nil
	}
	cloned := make(map[string]any, len(frontmatter))
	for key, value := range frontmatter {
		cloned[key] = value
	}
	return cloned
}
