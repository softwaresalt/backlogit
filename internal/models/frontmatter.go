package models

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ParseFrontmatter extracts YAML frontmatter from Markdown content.
// Returns the parsed key-value pairs, the remaining body text, and any error.
// Both LF and CRLF line endings are supported.
func ParseFrontmatter(content string) (map[string]any, string, error) {
	// Normalize CRLF to LF for consistent parsing.
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, content, nil
	}
	rest := normalized[4:]
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return nil, content, nil
	}
	yamlBlock := rest[:end]
	after := rest[end+4:]                   // skip "\n---"
	after = strings.TrimPrefix(after, "\n") // skip line ending of closing ---
	after = strings.TrimPrefix(after, "\n") // skip optional blank separator
	body := after

	var fm map[string]any
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}
	return fm, body, nil
}

// ArtifactFromFrontmatter converts raw frontmatter to a typed Artifact struct.
func ArtifactFromFrontmatter(fm map[string]any, body string) (*Artifact, error) {
	a := &Artifact{
		Description: body,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if v, ok := fm["id"].(string); ok {
		a.ID = v
	}
	if v, ok := fm["title"].(string); ok {
		a.Title = v
	}
	if v, ok := fm["status"].(string); ok {
		a.Status = ArtifactStatus(v)
	}
	if v, ok := fm["artifact_type"].(string); ok {
		a.ArtifactType = v
	}
	if v, ok := fm["parent_id"].(string); ok {
		a.ParentID = v
	}
	if v, ok := fm["sprint"].(string); ok {
		a.Sprint = v
	}
	if v, ok := fm["priority"].(string); ok {
		a.Priority = v
	}
	if v, ok := fm["custom_fields"].(map[string]any); ok {
		a.CustomFields = v
	}
	if v, ok := fm["created_at"].(time.Time); ok {
		a.CreatedAt = v
	}
	if v, ok := fm["updated_at"].(time.Time); ok {
		a.UpdatedAt = v
	}

	// New fields added in TASK-002.01.03.
	if v, ok := fm["assigned_to"].(string); ok {
		a.AssignedTo = v
	}
	if v, ok := fm["owner"].(string); ok {
		a.Owner = v
	}
	if v, ok := fm["commit"].(string); ok {
		a.Commit = v
	}
	// YAML unmarshals sequence values as []interface{}, not []string.
	if v, ok := fm["labels"]; ok {
		a.Labels = toStringSlice(v)
	}
	if v, ok := fm["dependencies"]; ok {
		a.Dependencies = toStringSlice(v)
	}
	if v, ok := fm["references"]; ok {
		a.References = toStringSlice(v)
	}

	if err := a.Validate(); err != nil {
		return nil, fmt.Errorf("artifact validation: %w", err)
	}
	return a, nil
}

// SerializeFrontmatter produces a Markdown string with YAML frontmatter and body.
func SerializeFrontmatter(fields map[string]any, body string) string {
	data, err := yaml.Marshal(fields)
	if err != nil {
		data = []byte{}
	}
	return "---\n" + string(data) + "---\n\n" + body
}

// toStringSlice converts a YAML-decoded sequence value to []string.
// YAML unmarshals sequences as []interface{}; this helper handles both
// that case and the native []string case. Returns nil for a nil input.
func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	if ss, ok := v.([]string); ok {
		return ss
	}
	if iface, ok := v.([]any); ok {
		result := make([]string, len(iface))
		for i, elem := range iface {
			result[i] = fmt.Sprintf("%v", elem)
		}
		return result
	}
	return nil
}
