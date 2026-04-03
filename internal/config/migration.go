package config

import (
	"fmt"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"

	"os"
)

// MigrationConfig holds the parsed migration.yaml configuration.
type MigrationConfig struct {
	DocumentClasses []DocumentClassConfig `yaml:"document_classes" validate:"required,min=1,dive"`
	DefaultLayout   string                `yaml:"default_layout" validate:"required,oneof=flat structured mixed"`
	SourcePaths     []SourcePathConfig    `yaml:"source_paths" validate:"dive"`
}

// DocumentClassConfig defines how a class of documents is detected and mapped.
type DocumentClassConfig struct {
	Name         string   `yaml:"name" validate:"required"`
	GlobPatterns []string `yaml:"glob_patterns" validate:"required,min=1"`
	ArtifactType string   `yaml:"artifact_type" validate:"required"`
	Keywords     []string `yaml:"keywords"`
}

// SourcePathConfig maps a directory path pattern to a document class.
type SourcePathConfig struct {
	Path  string `yaml:"path" validate:"required"`
	Class string `yaml:"class" validate:"required"`
}

// Validate checks all struct tags and returns a descriptive error on failure.
func (c *MigrationConfig) Validate() error {
	return validator.New().Struct(c)
}

// LoadMigrationConfig reads and validates migration.yaml from the workspace.
func LoadMigrationConfig(workspacePath string) (*MigrationConfig, error) {
	data, err := os.ReadFile(filepath.Join(workspacePath, "migration.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read migration config: %w", err)
	}
	var cfg MigrationConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse migration config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate migration config: %w", err)
	}
	return &cfg, nil
}

// DefaultMigrationConfig returns a sensible default migration configuration
// for common Backlog.md layouts.
func DefaultMigrationConfig() *MigrationConfig {
	return &MigrationConfig{
		DefaultLayout: "structured",
		DocumentClasses: []DocumentClassConfig{
			{
				Name:         "work_items",
				GlobPatterns: []string{"tasks/*.md", "drafts/*.md", "completed/*.md", "archive/*.md"},
				ArtifactType: "task",
				Keywords:     []string{"task", "acceptance criteria", "implementation plan"},
			},
			{
				Name:         "milestones",
				GlobPatterns: []string{"milestones/*.md"},
				ArtifactType: "feature",
				Keywords:     []string{"plan", "milestone"},
			},
		},
		SourcePaths: []SourcePathConfig{
			{Path: "tasks/", Class: "work_items"},
			{Path: "drafts/", Class: "work_items"},
			{Path: "completed/", Class: "work_items"},
			{Path: "archive/", Class: "work_items"},
			{Path: "milestones/", Class: "milestones"},
		},
	}
}

// WriteMigrationDefaults writes the default migration.yaml to the workspace if it does not exist.
func WriteMigrationDefaults(workspacePath string) error {
	cfg := DefaultMigrationConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal migration defaults: %w", err)
	}
	return writeFileIfNotExists(filepath.Join(workspacePath, "migration.yaml"), data)
}

// ResolveArtifactType maps a document class name to its configured artifact type.
func (c *MigrationConfig) ResolveArtifactType(className string) (string, error) {
	for _, dc := range c.DocumentClasses {
		if dc.Name == className {
			return dc.ArtifactType, nil
		}
	}
	return "", fmt.Errorf("document class %q not found", className)
}

// MatchClass finds the document class that matches the given file path based on glob patterns.
// Returns the first matching class name, or empty string if none match.
func (c *MigrationConfig) MatchClass(filePath string) string {
	for _, dc := range c.DocumentClasses {
		for _, pattern := range dc.GlobPatterns {
			if matched, err := filepath.Match(pattern, filePath); err == nil && matched {
				return dc.Name
			}
		}
	}
	return ""
}
