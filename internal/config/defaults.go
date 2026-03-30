package config

// DefaultConfig returns a sensible default workspace configuration.
//
// Worker: Implement default artifact types, fields, and naming templates.
func DefaultConfig() *WorkspaceConfig {
	panic("not implemented: Worker: Implement default workspace configuration")
}

// DefaultRegistry returns default directory routing rules.
//
// Worker: Implement default status-to-directory mappings.
func DefaultRegistry() *RegistryConfig {
	panic("not implemented: Worker: Implement default registry configuration")
}

// WriteDefaults writes default config.yaml and registry.yaml to the workspace.
//
// Worker: Serialize defaults to YAML files.
func WriteDefaults(workspacePath string) error {
	panic("not implemented: Worker: Implement writing default configuration files")
}
