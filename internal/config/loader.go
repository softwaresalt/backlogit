package config

import "context"

// Load reads and validates config.yaml, registry.yaml, and hooks.yaml from the workspace.
//
// Worker: Implement YAML loading, env var overrides, and struct validation.
func Load(_ context.Context, workspacePath string) (*WorkspaceConfig, error) {
	panic("not implemented: Worker: Implement YAML config loading with env var overrides and validation")
}
