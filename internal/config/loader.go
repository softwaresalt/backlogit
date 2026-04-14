package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
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

	if cfg.BugLevel == 0 {
		cfg.BugLevel = 3
	}

	applyEnvOverrides(&cfg)
	applyBugLevelConfig(&cfg)

	if err := cfg.Validate(); err != nil {
		var validationErrs validator.ValidationErrors
		if errors.As(err, &validationErrs) {
			for _, validationErr := range validationErrs {
				if validationErr.StructField() == "BugLevel" {
					return nil, fmt.Errorf("validate config: bug_level must be 2 or 3: %w", err)
				}
			}
		}
		return nil, fmt.Errorf("validate config: %w", err)
	}

	logger.Info("config loaded", "workspace", workspacePath)
	return &cfg, nil
}

// LoadRegistry reads registry.yaml from the workspace directory.
// If the file is missing, DefaultRegistry is returned so callers always have a valid config.
func LoadRegistry(workspacePath string) (*RegistryConfig, error) {
	regPath := filepath.Join(workspacePath, "registry.yaml")
	data, err := os.ReadFile(regPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRegistry(), nil
		}
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var reg RegistryConfig
	if err := yaml.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	if err := validate.Struct(&reg); err != nil {
		return nil, fmt.Errorf("validate registry: %w", err)
	}
	for _, rule := range reg.Directories {
		if filepath.IsAbs(rule.Path) {
			return nil, fmt.Errorf("registry path %q must be relative", rule.Path)
		}
		clean := filepath.Clean(rule.Path)
		if len(clean) >= 2 && clean[:2] == ".." {
			return nil, fmt.Errorf("registry path %q must not traverse outside the workspace", rule.Path)
		}
	}
	return &reg, nil
}

// LoadHooks reads hooks.yaml from the workspace directory.
// If the file is missing, DefaultHooksConfig is returned so callers always have a valid config.
func LoadHooks(workspacePath string) (*HooksConfig, error) {
	hooksPath := filepath.Join(workspacePath, "hooks.yaml")
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultHooksConfig(), nil
		}
		return nil, fmt.Errorf("read hooks: %w", err)
	}
	var cfg HooksConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse hooks: %w", err)
	}
	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("validate hooks: %w", err)
	}
	// Reject header values that don't use env var expansion (security guardrail).
	// Only $VAR or ${VAR} syntax is supported; os.ExpandEnv handles resolution.
	for i, ep := range cfg.Notifications.Endpoints {
		for key, val := range ep.Headers {
			if !strings.HasPrefix(val, "$") {
				return nil, fmt.Errorf("validate hooks: endpoint[%d] header %q value must start with '$' for environment variable expansion (literal secrets and 'env:' prefixes are not allowed in hooks.yaml)", i, key)
			}
		}
	}
	return &cfg, nil
}

func applyEnvOverrides(cfg *WorkspaceConfig) {
	if v, ok := os.LookupEnv("BACKLOGIT_MAX_SLUG_LENGTH"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxSlugLength = n
		}
	}
}
