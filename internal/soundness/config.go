package soundness

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Mode selects how the gate treats findings.
type Mode string

// Gate modes. The empty mode resolves to ModeReportOnly.
const (
	ModeReportOnly Mode = "report-only"
	ModeEnforce    Mode = "enforce"
)

// ErrInvalidConfig marks a configuration that cannot be honored. Misconfiguration
// is always an explicit error, never a silent pass.
var ErrInvalidConfig = errors.New("soundness: invalid config")

// ErrEmptyPlan marks a plan with no units. An empty plan is never a silent pass.
var ErrEmptyPlan = errors.New("soundness: plan has no units")

// ruleUnverifiable is the always-on structural check; it is not a flaggable rule.
const ruleUnverifiable = "unverifiable-unit"

// Budgets are the declarative per-unit size and focus limits.
type Budgets struct {
	MaxFiles            int
	MaxFunctions        int
	MaxScenarios        int
	MaxResponsibilities int
}

// Config configures one engine run. Rules maps rule IDs to an enable flag;
// an absent rule is enabled.
type Config struct {
	Mode                Mode
	Budgets             Budgets
	Rules               map[string]bool
	CanonicalProcedures []string
	Root                string
}

// DefaultConfig returns the default configuration: report-only mode, the
// constitution task-granularity budgets and every rule enabled.
func DefaultConfig() Config {
	return Config{
		Mode: ModeReportOnly,
		Budgets: Budgets{
			MaxFiles:            3,
			MaxFunctions:        5,
			MaxScenarios:        4,
			MaxResponsibilities: 1,
		},
		Rules: map[string]bool{},
	}
}

// ParseConfig parses the declarative text configuration: one "key: value" or
// "key = value" per line, "#" comments. Keys are mode, max_files,
// max_functions, max_scenarios, max_responsibilities, canonical_procedures
// (comma-separated) and rule.<id> (on/off). Unknown or duplicate keys and bad
// values are errors.
func ParseConfig(text string) (Config, error) {
	cfg := DefaultConfig()
	seen := map[string]bool{}
	for i, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sep := strings.IndexAny(line, ":=")
		if sep <= 0 {
			return Config{}, fmt.Errorf("%w: line %d: expected key: value", ErrInvalidConfig, i+1)
		}
		key := strings.TrimSpace(line[:sep])
		val := strings.TrimSpace(line[sep+1:])
		if seen[key] {
			return Config{}, fmt.Errorf("%w: line %d: duplicate key %q", ErrInvalidConfig, i+1, key)
		}
		seen[key] = true
		if err := applyConfigKey(&cfg, key, val); err != nil {
			return Config{}, fmt.Errorf("%w: line %d: %w", ErrInvalidConfig, i+1, err)
		}
	}
	return cfg, nil
}

func applyConfigKey(cfg *Config, key, val string) error {
	budgets := map[string]*int{
		"max_files":            &cfg.Budgets.MaxFiles,
		"max_functions":        &cfg.Budgets.MaxFunctions,
		"max_scenarios":        &cfg.Budgets.MaxScenarios,
		"max_responsibilities": &cfg.Budgets.MaxResponsibilities,
	}
	switch {
	case key == "mode":
		m, err := parseMode(val)
		if err != nil {
			return err
		}
		cfg.Mode = m
	case budgets[key] != nil:
		n, err := strconv.Atoi(val)
		if err != nil || n < 1 {
			return fmt.Errorf("%s must be a positive integer, got %q", key, val)
		}
		*budgets[key] = n
	case key == "canonical_procedures":
		cfg.CanonicalProcedures = nil
		for _, p := range strings.Split(val, ",") {
			if p = strings.TrimSpace(p); p != "" {
				cfg.CanonicalProcedures = append(cfg.CanonicalProcedures, p)
			}
		}
	case strings.HasPrefix(key, "rule."):
		on, err := parseFlag(val)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		cfg.Rules[strings.TrimPrefix(key, "rule.")] = on
	default:
		return fmt.Errorf("unknown key %q", key)
	}
	return nil
}

func parseMode(val string) (Mode, error) {
	switch Mode(val) {
	case "", ModeReportOnly:
		return ModeReportOnly, nil
	case ModeEnforce:
		return ModeEnforce, nil
	default:
		return "", fmt.Errorf("mode must be %q or %q, got %q", ModeReportOnly, ModeEnforce, val)
	}
}

func parseFlag(val string) (bool, error) {
	switch strings.ToLower(val) {
	case "on", "true", "yes", "enabled":
		return true, nil
	case "off", "false", "no", "disabled":
		return false, nil
	default:
		return false, fmt.Errorf("flag must be on or off, got %q", val)
	}
}

// validate rejects configurations that cannot be honored: non-positive
// budgets, an unknown mode, or flags naming rules that do not exist.
func (c Config) validate(known map[string]bool) error {
	if _, err := parseMode(string(c.Mode)); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
	}
	b := c.Budgets
	if b.MaxFiles < 1 || b.MaxFunctions < 1 || b.MaxScenarios < 1 || b.MaxResponsibilities < 1 {
		return fmt.Errorf("%w: budgets must be positive", ErrInvalidConfig)
	}
	for id := range c.Rules {
		if id == ruleUnverifiable {
			return fmt.Errorf("%w: rule %q cannot be disabled", ErrInvalidConfig, id)
		}
		if !known[id] {
			return fmt.Errorf("%w: unknown rule %q", ErrInvalidConfig, id)
		}
	}
	return nil
}

func (c Config) enabled(id string) bool {
	on, ok := c.Rules[id]
	return !ok || on
}
