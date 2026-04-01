package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// defaultHeaderDef returns the default HeaderDefConfig for a new workspace.
// Uses 3 initial artifact types: task, bug, epic (revision-3).
func defaultHeaderDef() *HeaderDefConfig {
	statusField := &FieldDef{
		Type:    "enum",
		Values:  []string{"queued", "active", "blocked", "review", "done", "accepted", "rejected"},
		Default: "queued",
	}
	return &HeaderDefConfig{
		Defaults: SystemDefaults{
			ID:          FieldDef{Type: "string", Immutable: true},
			CreatedDate: FieldDef{Type: "datetime", Immutable: true},
			UpdatedDate: FieldDef{Type: "datetime", Immutable: true},
		},
		Types: map[string]*TypeDefConfig{
			"task": {
				Prefix:   "OP",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"priority": {
						Type:    "enum",
						Values:  []string{"low", "medium", "high"},
						Default: "medium",
					},
				},
			},
			"bug": {
				Prefix:   "OP",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"severity": {
						Type:    "enum",
						Values:  []string{"low", "medium", "high", "critical"},
						Default: "medium",
					},
				},
			},
			"epic": {
				Prefix:   "OP",
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
		"task": `---
name: task-template
type: task
sections:
  - name: description
    required: true
  - name: acceptance-criteria
    required: false
  - name: implementation-notes
    required: false
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
		"bug": `---
name: bug-template
type: bug
sections:
  - name: description
    required: true
  - name: steps-to-reproduce
    required: true
  - name: expected-behavior
    required: false
  - name: actual-behavior
    required: false
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Steps to Reproduce

<!-- BEGIN:steps-to-reproduce -->
<!-- END:steps-to-reproduce -->

## Expected Behavior

<!-- BEGIN:expected-behavior -->
<!-- END:expected-behavior -->

## Actual Behavior

<!-- BEGIN:actual-behavior -->
<!-- END:actual-behavior -->
`,
		"epic": `---
name: epic-template
type: epic
sections:
  - name: description
    required: true
  - name: goals
    required: false
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Goals

<!-- BEGIN:goals -->
<!-- END:goals -->
`,
	}
}

// DefaultConfig returns a sensible default workspace configuration.
func DefaultConfig() *WorkspaceConfig {
	return &WorkspaceConfig{
		MaxSlugLength: 60,
		ArtifactTypes: map[string]*ArtifactTypeConfig{
			"task": {
				Prefix:     "T",
				NameFormat: "{prefix}{NNN}-{title_slug}",
			},
			"story": {
				Prefix:     "S",
				NameFormat: "{prefix}{NNN}-{title_slug}",
			},
			"bug": {
				Prefix:     "B",
				NameFormat: "{prefix}{NNN}-{title_slug}",
			},
			"epic": {
				Prefix:     "E",
				NameFormat: "{prefix}{NNN}-{title_slug}",
			},
		},
		Fields: map[string]*FieldConfig{
			"status": {
				Type:    "enum",
				Values:  []string{"queued", "active", "blocked", "review", "done"},
				Default: "queued",
			},
		},
	}
}

// DefaultRegistry returns default directory routing rules.
// Status-based rules are listed first so they take priority over type-based rules.
func DefaultRegistry() *RegistryConfig {
	return &RegistryConfig{
		Directories: []DirectoryRule{
			{Path: "archive", Condition: DirectoryCondition{Status: []string{"done", "accepted", "rejected"}}},
			{Path: "review", Condition: DirectoryCondition{Status: []string{"review"}}},
			{Path: "tasks", Condition: DirectoryCondition{Type: []string{"task"}}},
			{Path: "stories", Condition: DirectoryCondition{Type: []string{"story"}}},
			{Path: "bugs", Condition: DirectoryCondition{Type: []string{"bug"}}},
			{Path: "epics", Condition: DirectoryCondition{Type: []string{"epic"}}},
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

	return nil
}

// writeFileIfNotExists writes data to path only if the file does not already exist.
func writeFileIfNotExists(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil // file already exists; skip
	}
	return os.WriteFile(path, data, 0o644)
}
