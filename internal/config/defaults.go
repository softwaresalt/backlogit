package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

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
func DefaultRegistry() *RegistryConfig {
	return &RegistryConfig{
		Directories: []DirectoryRule{
			{Path: "tasks", Condition: DirectoryCondition{Type: []string{"task"}}},
			{Path: "stories", Condition: DirectoryCondition{Type: []string{"story"}}},
			{Path: "bugs", Condition: DirectoryCondition{Type: []string{"bug"}}},
			{Path: "epics", Condition: DirectoryCondition{Type: []string{"epic"}}},
		},
	}
}

// WriteDefaults serializes DefaultConfig and DefaultRegistry to the workspace directory.
func WriteDefaults(workspacePath string) error {
	cfgData, err := yaml.Marshal(DefaultConfig())
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	regData, err := yaml.Marshal(DefaultRegistry())
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	if err := os.WriteFile(filepath.Join(workspacePath, "config.yaml"), cfgData, 0o644); err != nil {
		return fmt.Errorf("write config.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(workspacePath, "registry.yaml"), regData, 0o644); err != nil {
		return fmt.Errorf("write registry.yaml: %w", err)
	}
	return nil
}
