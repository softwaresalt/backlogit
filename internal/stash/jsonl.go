package stash

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// WriteJSONL serializes a slice of stash entries to the given writer in JSONL format.
// Each entry is written as a single JSON object per line.
func WriteJSONL(w io.Writer, entries []Entry) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return fmt.Errorf("encode stash entry: %w", err)
		}
	}
	return nil
}

// utf8BOM is the byte-order mark prepended by some Windows editors.
var utf8BOM = "\xef\xbb\xbf"

// ReadJSONL deserializes stash entries from a JSONL reader.
// Each line must be a valid JSON object representing one Entry.
// Empty lines are skipped. A leading UTF-8 BOM on the first line is stripped.
// Returns an error on malformed JSON.
func ReadJSONL(r io.Reader) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(r)
	first := true
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if first {
			line = strings.TrimPrefix(line, utf8BOM)
			first = false
		}
		if line == "" {
			continue
		}
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return nil, fmt.Errorf("unmarshal stash entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read stash jsonl: %w", err)
	}
	return entries, nil
}

// yamlEntry is a local struct for parsing YAML-list style stash entries.
type yamlEntry struct {
	ID           string `yaml:"id"`
	Priority     string `yaml:"priority"`
	Kind         string `yaml:"kind"`
	Text         string `yaml:"text"`
	Deliberation string `yaml:"deliberation"`
}

// MigrateStashMDToJSONL reads the legacy .stash.md file at srcPath, parses entries,
// and writes them as JSONL to dstPath using atomic temp-file-then-rename.
// Returns the number of migrated entries and any error.
func MigrateStashMDToJSONL(srcPath, dstPath string) (int, error) {
	// Try regex-based parser first (existing format).
	_, regexEntries, regexErr := ParseFile(srcPath)

	// Also try YAML-list parser for the newer format.
	yamlEntries, yamlErr := parseStashMDAsYAML(srcPath)

	// Prefer whichever parser found more entries, falling back gracefully.
	var entries []Entry
	if regexErr == nil && len(regexEntries) >= len(yamlEntries) {
		entries = regexEntries
	} else if yamlErr == nil {
		entries = yamlEntries
	} else if regexErr == nil {
		entries = regexEntries
	} else {
		return 0, fmt.Errorf("parse stash.md: %w", regexErr)
	}

	// Write JSONL to temp file then rename atomically.
	tmpPath := dstPath + ".tmp"
	tmp, err := os.Create(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("create temp jsonl: %w", err)
	}
	if err := WriteJSONL(tmp, entries); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return 0, fmt.Errorf("write jsonl: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("close temp jsonl: %w", err)
	}
	if err := os.Rename(tmpPath, dstPath); err != nil {
		os.Remove(tmpPath)
		return 0, fmt.Errorf("rename jsonl: %w", err)
	}

	slog.Info("stash migration complete", "src", filepath.Base(srcPath), "count", len(entries))
	return len(entries), nil
}

// parseStashMDAsYAML parses the stash.md body as a YAML list of entries.
// This handles the newer YAML-style stash format.
func parseStashMDAsYAML(srcPath string) ([]Entry, error) {
	raw, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("read stash file: %w", err)
	}

	// Strip frontmatter and get body.
	content := string(raw)
	body := content
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			body = parts[2]
		}
	}

	body = strings.TrimSpace(body)
	if body == "" {
		return nil, nil
	}

	var yamlEntries []yamlEntry
	if err := yaml.Unmarshal([]byte(body), &yamlEntries); err != nil {
		return nil, fmt.Errorf("parse yaml entries: %w", err)
	}

	entries := make([]Entry, 0, len(yamlEntries))
	for _, ye := range yamlEntries {
		entries = append(entries, Entry{
			ID:             ye.ID,
			Priority:       ye.Priority,
			Kind:           ye.Kind,
			Text:           ye.Text,
			DeliberationID: ye.Deliberation,
		})
	}
	return entries, nil
}
