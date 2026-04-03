package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/backlogit/backlogit/internal/stash"
	"gopkg.in/yaml.v3"
)

// defaultHeaderDef returns the default HeaderDefConfig for a new workspace.
// Uses 3 initial artifact types: feature, task, subtask.
func defaultHeaderDef() *HeaderDefConfig {
	statusField := &FieldDef{
		Type:    "enum",
		Values:  []string{"queued", "active", "blocked", "review", "done", "accepted", "rejected", "archived"},
		Default: "queued",
	}
	return &HeaderDefConfig{
		Defaults: SystemDefaults{
			ID:          FieldDef{Type: "string", Immutable: true},
			CreatedDate: FieldDef{Type: "datetime", Immutable: true},
			UpdatedDate: FieldDef{Type: "datetime", Immutable: true},
		},
		Types: map[string]*TypeDefConfig{
			"feature": {
				Prefix:   "F",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"harness_status": {
						Type:     "enum",
						Values:   []string{"pending", "scaffolded", "passing", "failing"},
						Default:  "pending",
						Optional: true,
					},
				},
			},
			"task": {
				Prefix:   "T",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"priority": {
						Type:    "enum",
						Values:  []string{"low", "medium", "high", "critical"},
						Default: "medium",
					},
				},
			},
			"subtask": {
				Prefix:   "ST",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*FieldDef{
					"status": statusField,
				},
			},
		},
	}
}

// defaultTemplates returns the default template content for each artifact type.
func defaultTemplates() map[string]string {
	return map[string]string{
		"feature": `---
name: feature-template
type: feature
description: "A feature-level work item"
sections:
  - name: description
    required: true
    description: "Detailed description of the feature"
  - name: goals
    required: false
    description: "Goals and intended outcomes"
  - name: dod
    required: false
    description: "Definition of Done for this feature"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Goals

<!-- BEGIN:goals -->
<!-- END:goals -->

## Definition of Done

<!-- BEGIN:dod -->
<!-- END:dod -->
`,
		"task": `---
name: task-template
type: task
description: "A discrete unit of work"
sections:
  - name: description
    required: true
    description: "Detailed description of the work item"
  - name: acceptance-criteria
    required: false
    description: "Conditions that must be met for completion"
  - name: implementation-notes
    required: false
    description: "Technical notes and implementation details"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Acceptance Criteria

<!-- BEGIN:acceptance-criteria -->
<!-- END:acceptance-criteria -->

## Implementation Notes

<!-- BEGIN:implementation-notes -->
<!-- END:implementation-notes -->
`,
		"subtask": `---
name: subtask-template
type: subtask
description: "A discrete unit of work"
sections:
  - name: description
    required: true
    description: "Detailed description of the discrete work item"
  - name: implementation-notes
    required: false
    description: "Technical notes and implementation details"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Implementation Notes

<!-- BEGIN:implementation-notes -->
<!-- END:implementation-notes -->
`,
	}
}

// DefaultConfig returns a sensible default workspace configuration.
func DefaultConfig() *WorkspaceConfig {
	return &WorkspaceConfig{
		MaxSlugLength: 60,
		ArtifactTypes: map[string]*ArtifactTypeConfig{
			"feature": {
				Prefix:          "F",
				NameFormat:      "{prefix}{NNN}",
				AllowedChildren: []string{"task"},
			},
			"task": {
				Prefix:          "T",
				NameFormat:      "{prefix}{NNN}",
				AllowedChildren: []string{"subtask"},
			},
			"subtask": {
				Prefix:     "ST",
				NameFormat: "{prefix}{NNN}",
			},
		},
		Fields: map[string]*FieldConfig{
			"status": {
				Type:    "enum",
				Values:  []string{"queued", "active", "blocked", "review", "done", "accepted", "rejected", "archived"},
				Default: "queued",
			},
		},
		QueueLayout: &QueueLayoutConfig{
			RootDir:    "queue",
			NameFormat: "{NNN}",
			Levels: []HierarchyLevel{
				{Level: 1, Types: []string{"feature"}},
				{Level: 2, Types: []string{"task"}},
				{Level: 3, Types: []string{"subtask"}},
			},
		},
	}
}

// DefaultRegistry returns default directory routing rules.
// Status-based rules are listed first so they take priority over type-based rules.
func DefaultRegistry() *RegistryConfig {
	return &RegistryConfig{
		Directories: []DirectoryRule{
			{Path: "archive", Condition: DirectoryCondition{Status: []string{"done", "accepted", "rejected", "archived"}}},
			{Path: "queue", Condition: DirectoryCondition{Status: []string{"queued", "active", "blocked", "review"}}},
		},
	}
}

// WriteDefaults serializes DefaultConfig and DefaultRegistry to the workspace directory.
// Also writes header-def.yaml and default templates. Existing files are not overwritten.
func WriteDefaults(workspacePath string) error {
	cfgData, err := yaml.Marshal(DefaultConfig())
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	regData, err := yaml.Marshal(DefaultRegistry())
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	headerDefData, err := yaml.Marshal(defaultHeaderDef())
	if err != nil {
		return fmt.Errorf("marshal header-def: %w", err)
	}

	if err := writeFileIfNotExists(filepath.Join(workspacePath, "config.yaml"), cfgData); err != nil {
		return fmt.Errorf("write config.yaml: %w", err)
	}
	if err := writeFileIfNotExists(filepath.Join(workspacePath, "registry.yaml"), regData); err != nil {
		return fmt.Errorf("write registry.yaml: %w", err)
	}
	if err := writeFileIfNotExists(filepath.Join(workspacePath, "header-def.yaml"), headerDefData); err != nil {
		return fmt.Errorf("write header-def.yaml: %w", err)
	}

	// Create templates directory and write default template files.
	templatesDir := filepath.Join(workspacePath, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		return fmt.Errorf("create templates dir: %w", err)
	}
	for typeName, content := range defaultTemplates() {
		path := filepath.Join(templatesDir, typeName+".md")
		if err := writeFileIfNotExists(path, []byte(content)); err != nil {
			return fmt.Errorf("write template %s: %w", typeName, err)
		}
	}

	queueDir := filepath.Join(workspacePath, "queue")
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		return fmt.Errorf("create queue dir: %w", err)
	}
	if err := writeFileIfNotExists(filepath.Join(queueDir, ".stash.md"), []byte(stash.DefaultContent())); err != nil {
		return fmt.Errorf("write .stash.md: %w", err)
	}

	return nil
}

// writeFileIfNotExists writes data to path only if the file does not already exist.
func writeFileIfNotExists(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil // file already exists; skip
	}
	return os.WriteFile(path, data, 0o644)
}
