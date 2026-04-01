package config

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
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
//
// Worker: Read the file at workspacePath/migration.yaml, unmarshal YAML,
// validate with struct tags, return the config or a descriptive error.
func LoadMigrationConfig(workspacePath string) (*MigrationConfig, error) {
	_ = workspacePath
	panic("not implemented: Worker: Read workspacePath/migration.yaml with os.ReadFile. Unmarshal with yaml.v3. Call Validate(). Return (*MigrationConfig, nil) on success or (nil, error) with context wrapping.")
}

// DefaultMigrationConfig returns a sensible default migration configuration
// for common Backlog.md layouts.
//
// Worker: Return a MigrationConfig with these defaults:
//   - document_classes: specs (requirements/*.md → story), plans (plans/*.md → epic),
//     work_items (tasks/*.md, bugs/*.md → task/bug), decisions (decisions/*.md → adr),
//     notes (notes/*.md, docs/*.md → note)
//   - default_layout: "flat"
//   - source_paths: common directory patterns
func DefaultMigrationConfig() *MigrationConfig {
	panic("not implemented: Worker: Build and return a *MigrationConfig with sensible defaults for each document class. Map glob patterns to artifact types. Set default_layout to 'flat'.")
}

// WriteMigrationDefaults writes the default migration.yaml to the workspace if it does not exist.
//
// Worker: Call DefaultMigrationConfig(), marshal to YAML, write to workspacePath/migration.yaml
// using writeFileIfNotExists pattern from defaults.go. Return nil if file already exists.
func WriteMigrationDefaults(workspacePath string) error {
	_ = workspacePath
	panic("not implemented: Worker: Get DefaultMigrationConfig(), yaml.Marshal it, write to filepath.Join(workspacePath, 'migration.yaml') using writeFileIfNotExists. Return nil on success or if file exists.")
}

// ResolveArtifactType maps a document class name to its configured artifact type.
//
// Worker: Search document_classes for matching name, return the artifact_type.
// Return error if class name is not found.
func (c *MigrationConfig) ResolveArtifactType(className string) (string, error) {
	_ = className
	panic(fmt.Sprintf("not implemented: Worker: Iterate c.DocumentClasses, find entry where Name == className, return ArtifactType. Return error if not found."))
}

// MatchClass finds the document class that matches the given file path based on glob patterns.
//
// Worker: For each DocumentClassConfig, check if any of its GlobPatterns match the path
// using filepath.Match. Return the first matching class name, or empty string if none match.
func (c *MigrationConfig) MatchClass(filePath string) string {
	_ = filePath
	panic("not implemented: Worker: Iterate c.DocumentClasses, for each check c.GlobPatterns against filePath using filepath.Match. Return first match's Name, or empty string.")
}

// Ensure imports are used.
var (
	_ = os.ReadFile
	_ = yaml.Unmarshal
	_ = fmt.Errorf
)
