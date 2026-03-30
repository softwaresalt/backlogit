package config

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

var logger = slog.With("package", "config")

// Load reads and validates config.yaml from the workspace directory.
func Load(_ context.Context, workspacePath string) (*WorkspaceConfig, error) {
	cfgPath := filepath.Join(workspacePath, "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg WorkspaceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyEnvOverrides(&cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	logger.Info("config loaded", "workspace", workspacePath)
	return &cfg, nil
}

func applyEnvOverrides(cfg *WorkspaceConfig) {
	if v, ok := os.LookupEnv("BACKLOGIT_MAX_SLUG_LENGTH"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSlugLength = n
		}
	}
}
