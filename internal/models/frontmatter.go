package models

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ParseFrontmatter extracts YAML frontmatter from Markdown content.
// Returns the parsed key-value pairs, the remaining body text, and any error.
func ParseFrontmatter(content string) (map[string]any, string, error) {
	if !strings.HasPrefix(content, "---\n") {
		return nil, content, nil
	}
	rest := content[4:]
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
