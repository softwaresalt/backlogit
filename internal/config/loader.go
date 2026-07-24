package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
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

	if cfg.CheckpointRetention.RetentionDays == 0 {
		cfg.CheckpointRetention.RetentionDays = 7
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

	// Config-load containment (108-F SE-7a): reject an absolute path or a lexical
	// `..` escape in the queue search root. This is the first layer of the
	// two-layer containment guard; lookup-time realpath re-containment (SE-7b)
	// is enforced separately in the core lookup path. Mirrors the registry
	// directory guard above.
	if cfg.QueueLayout != nil {
		if err := ensureContainedRelPath("queue_layout.root_dir", cfg.QueueLayout.RootDir); err != nil {
			return nil, fmt.Errorf("validate config: %w", err)
		}
	}

	logger.Info("config loaded", "workspace", workspacePath)
	return &cfg, nil
}

// ensureContainedRelPath rejects an absolute path or a lexical `..` escape in a
// workspace-relative configuration path. Backslash- and slash-style escapes are
// both caught because a cleaned traversal path always begins with "..". This is
// the config-load layer of the 108-F two-layer containment guard (SE-7a).
func ensureContainedRelPath(label, p string) error {
	if p == "" {
		return nil
	}
	if filepath.IsAbs(p) {
		return fmt.Errorf("%s %q must be workspace-relative; an absolute path resolves outside the workspace", label, p)
	}
	clean := filepath.Clean(p)
	if len(clean) >= 2 && clean[:2] == ".." {
		return fmt.Errorf("%s %q must not traverse outside the workspace", label, p)
	}
	return nil
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

// priorGeneratedDefaultTransitions is the exact lifecycle.transitions map that
// DefaultHooksConfig() and WriteDefaults produced before the 124.002-T
// transition-map widening (blocked/active lacked queued targets).
// Used by upgradeLegacyTransitions to discriminate a legacy-generated config
// from an operator-customized one. Must NOT be edited — immutable historical snapshot.
var priorGeneratedDefaultTransitions = map[string][]string{
	"queued":  {"active", "blocked"},
	"active":  {"done", "blocked", "review", "shipped", "abandoned"},
	"blocked": {"active"},
	"review":  {"done", "accepted", "rejected"},
	"done":    {"archived"},
}

// upgradeLegacyTransitions upgrades a persisted Lifecycle.Transitions map when
// it is an exact match to the pre-124.002-T generated default. Any map that
// differs in any way is treated as operator-customized and returned unchanged,
// preserving the operator's intentional policy. Mirrors the
// PreTaskCompletionGate.Normalize() precedent (082-F).
//
// An absent (nil/empty) map is returned unchanged because the runtime falls
// back to DefaultTransitions() via ValidateStatusTransition(nil).
func upgradeLegacyTransitions(persisted map[string][]string) map[string][]string {
	if len(persisted) == 0 {
		return persisted
	}
	if !reflect.DeepEqual(persisted, priorGeneratedDefaultTransitions) {
		// Differs from the known prior generated default in some way.
		// Treat as operator-customized; do not inject queued.
		return persisted
	}
	// Exact match to the prior generated default → legacy-generated config.
	// Upgrade to the current default (adds blocked->queued and active->queued).
	return DefaultHooksConfig().Lifecycle.Transitions
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
	// Normalize and validate the pre-task-completion gate block (082-F). An absent
	// block normalizes to the documented defaults (enabled:auto, force_cli_only:true).
	cfg.Lifecycle.PreTaskCompletionGate.Normalize()
	if err := cfg.Lifecycle.PreTaskCompletionGate.Validate(); err != nil {
		return nil, fmt.Errorf("validate hooks: %w", err)
	}
	// Upgrade a legacy-generated transitions map to include the new default
	// entries (blocked->queued, active->queued). Only fires when the persisted
	// map is byte-for-byte equal to the pre-124.002-T generated default;
	// operator-customized maps are left untouched (124.004-T).
	cfg.Lifecycle.Transitions = upgradeLegacyTransitions(cfg.Lifecycle.Transitions)
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
