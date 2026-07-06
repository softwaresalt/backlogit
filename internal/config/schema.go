package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func init() {
	validate.RegisterStructValidation(validateArtifactTypeConfig, ArtifactTypeConfig{})
	validate.RegisterStructValidation(validateArtifactTypeConfig, &ArtifactTypeConfig{})
}

// Validate checks all struct tags and returns a descriptive error on failure.
func (c *WorkspaceConfig) Validate() error {
	return validate.Struct(c)
}

// WorkspaceConfig holds the parsed workspace configuration.
type WorkspaceConfig struct {
	ArtifactTypes       map[string]*ArtifactTypeConfig `yaml:"artifact_types" validate:"required,min=1"`
	Fields              map[string]*FieldConfig        `yaml:"fields"`
	BugLevel            int                            `yaml:"bug_level,omitempty" validate:"omitempty,oneof=2 3"`
	MaxSlugLength       int                            `yaml:"max_slug_length" validate:"gte=10,lte=200"`
	QueueLayout         *QueueLayoutConfig             `yaml:"queue_layout"`
	CheckpointRetention CheckpointRetention            `yaml:"checkpoint_retention,omitempty"`
	Telemetry           *TelemetryConfig               `yaml:"telemetry,omitempty"`
}

// TelemetryConfig holds workspace-scoped telemetry settings.
type TelemetryConfig struct {
	// AttributionPrefixes extends or overrides the built-in MCP server attribution
	// registry. Keys are tool name prefixes (e.g. "myserver-") or exact names;
	// values are the server label to attribute. Custom entries take priority over
	// built-in defaults when the same key appears in both.
	AttributionPrefixes map[string]string `yaml:"attribution_prefixes,omitempty"`
}

// CheckpointRetention configures checkpoint file retention policy.
type CheckpointRetention struct {
	RetentionDays int `yaml:"retention_days" validate:"omitempty,gte=1"`
}

// ArtifactTypeConfig defines an artifact type's behavior.
type ArtifactTypeConfig struct {
	Prefix          string   `yaml:"prefix" validate:"required"`
	Suffix          string   `yaml:"suffix,omitempty"`
	NameFormat      string   `yaml:"name_format" validate:"required"`
	FileNameFormat  string   `yaml:"file_name_format,omitempty"`
	AllowedChildren []string `yaml:"allowed_children"`
}

func validateArtifactTypeConfig(sl validator.StructLevel) {
	cfg, ok := sl.Current().Interface().(ArtifactTypeConfig)
	if !ok {
		ptr, ok := sl.Current().Interface().(*ArtifactTypeConfig)
		if !ok || ptr == nil {
			return
		}
		cfg = *ptr
	}
	if strings.Contains(cfg.NameFormat, "{suffix}") && strings.TrimSpace(cfg.Suffix) == "" {
		sl.ReportError(cfg.Suffix, "suffix", "Suffix", "suffixrequired", "")
	}
}

// FieldConfig defines a custom field's schema.
type FieldConfig struct {
	Type     string   `yaml:"type" validate:"required,oneof=enum string int"`
	Values   []string `yaml:"values"`
	Default  string   `yaml:"default"`
	Optional bool     `yaml:"optional"`
	// ExternalMap holds translation rules for external systems (e.g., Jira, ADO).
	// Uses map[string]any because external system payloads have heterogeneous value types.
	ExternalMap map[string]any `yaml:"external_map"`
}

// RegistryConfig holds directory routing rules.
type RegistryConfig struct {
	Directories []DirectoryRule `yaml:"directories" validate:"required"`
}

// DirectoryRule maps conditions to a target directory path.
type DirectoryRule struct {
	Path      string             `yaml:"path" validate:"required"`
	Condition DirectoryCondition `yaml:"condition"`
}

// DirectoryCondition specifies when a directory rule applies.
type DirectoryCondition struct {
	Status []string `yaml:"status"`
	Type   []string `yaml:"type"`
}

// QueueLayoutConfig defines the hierarchical file organization for .backlogit/queue/.
type QueueLayoutConfig struct {
	RootDir    string           `yaml:"root_dir" validate:"required"`
	Levels     []HierarchyLevel `yaml:"levels" validate:"required,min=1,dive"`
	NameFormat string           `yaml:"name_format"`
}

// HierarchyLevel maps a hierarchy depth to one or more artifact types.
type HierarchyLevel struct {
	Level int      `yaml:"level" validate:"required,gte=1,lte=5"`
	Types []string `yaml:"types" validate:"required,min=1"`
}

// HooksConfig configures the agent hook event system.
type HooksConfig struct {
	Enabled            bool                 `yaml:"enabled"`
	EventThresholds    HookEventThresholds  `yaml:"event_thresholds,omitempty"`
	AgentSubscriptions map[string][]string  `yaml:"agent_subscriptions,omitempty"`
	Lifecycle          LifecycleHooksConfig `yaml:"lifecycle,omitempty"`
	Notifications      NotificationsConfig  `yaml:"notifications,omitempty"`
}

// HookEventThresholds controls derived signal computation for v1 event types.
// Deferred signals (stash_overflow, shipment_ready) will add thresholds in v2.
type HookEventThresholds struct {
	BlockedStaleDays int `yaml:"blocked_stale_days" validate:"gte=0"`
}

// LifecycleHooksConfig controls built-in lifecycle hook behavior.
type LifecycleHooksConfig struct {
	ValidateTransition    bool                        `yaml:"validate_transition"`
	EmitEvents            bool                        `yaml:"emit_events"`
	Transitions           map[string][]string         `yaml:"transitions,omitempty"`
	PreTaskCompletionGate PreTaskCompletionGateConfig `yaml:"pre_task_completion_gate,omitempty"`
}

// PreTaskCompletionGateConfig configures the built-in pre-task-completion gate
// broker (082-F): backlogit synchronously invokes `autoharness gate check`
// before writing task/subtask -> a terminal status and shipment -> shipped.
type PreTaskCompletionGateConfig struct {
	// Enabled is three-valued: "auto" (enforce when autoharness is resolvable,
	// else fail open), "true" (strict, fail closed), or "false" (kill switch).
	Enabled string `yaml:"enabled,omitempty"`
	// TerminalStatuses are the statuses whose entry triggers the gate. Default ["done"].
	TerminalStatuses []string `yaml:"terminal_statuses,omitempty"`
	// AutoharnessBinary is the gate executable. Resolved via PATH by default; an
	// absolute path or a ".." traversal is rejected at validation.
	AutoharnessBinary string `yaml:"autoharness_binary,omitempty"`
	// BaseRef is the default-branch base ref, or "auto" to discover it.
	BaseRef string `yaml:"base_ref,omitempty"`
	// TimeoutSeconds bounds the gate run. Must be within [1, 3600]; the completion
	// path refreshes the task-lock sidecar (heartbeat) so a value above the 60s
	// lock stale-TTL cannot let a concurrent process reap the live lock mid-gate.
	TimeoutSeconds int `yaml:"timeout_seconds,omitempty"`
	// ForceCLIOnly is a hard v1 invariant: force is operator-only via the CLI.
	// Nil defaults to true; an explicit false is rejected.
	ForceCLIOnly *bool `yaml:"force_cli_only,omitempty"`
	// EvidenceRequired makes the backlogit gate-evidence append part of the
	// transition contract. Nil defaults to true.
	EvidenceRequired *bool `yaml:"evidence_required,omitempty"`
}

// gateKnownStatuses is the set of valid artifact statuses accepted in
// terminal_statuses. Kept local to config to avoid a dependency on internal/models.
var gateKnownStatuses = map[string]bool{
	"queued": true, "active": true, "blocked": true, "review": true,
	"done": true, "accepted": true, "rejected": true, "archived": true,
	"shipped": true, "abandoned": true,
}

// Normalize fills zero-valued fields with their documented defaults. It is
// idempotent and safe to call on an absent (zero-value) block.
func (g *PreTaskCompletionGateConfig) Normalize() {
	if g.Enabled == "" {
		g.Enabled = "auto"
	}
	if len(g.TerminalStatuses) == 0 {
		g.TerminalStatuses = []string{"done"}
	}
	if g.AutoharnessBinary == "" {
		g.AutoharnessBinary = "autoharness"
	}
	if g.BaseRef == "" {
		g.BaseRef = "auto"
	}
	if g.TimeoutSeconds == 0 {
		g.TimeoutSeconds = 600
	}
	if g.ForceCLIOnly == nil {
		t := true
		g.ForceCLIOnly = &t
	}
	if g.EvidenceRequired == nil {
		t := true
		g.EvidenceRequired = &t
	}
}

// ForceCLIOnlyValue returns the effective force_cli_only after normalization.
func (g PreTaskCompletionGateConfig) ForceCLIOnlyValue() bool {
	return g.ForceCLIOnly == nil || *g.ForceCLIOnly
}

// EvidenceRequiredValue returns the effective evidence_required after normalization.
func (g PreTaskCompletionGateConfig) EvidenceRequiredValue() bool {
	return g.EvidenceRequired == nil || *g.EvidenceRequired
}

// Validate enforces the gate config invariants after Normalize.
func (g PreTaskCompletionGateConfig) Validate() error {
	switch g.Enabled {
	case "auto", "true", "false":
	default:
		return fmt.Errorf("pre_task_completion_gate.enabled must be one of auto|true|false, got %q", g.Enabled)
	}
	if len(g.TerminalStatuses) == 0 {
		return fmt.Errorf("pre_task_completion_gate.terminal_statuses must not be empty")
	}
	for _, s := range g.TerminalStatuses {
		if !gateKnownStatuses[s] {
			return fmt.Errorf("pre_task_completion_gate.terminal_statuses has unknown status %q", s)
		}
	}
	if g.TimeoutSeconds < 1 || g.TimeoutSeconds > 3600 {
		return fmt.Errorf("pre_task_completion_gate.timeout_seconds must be within [1, 3600], got %d", g.TimeoutSeconds)
	}
	if g.ForceCLIOnly != nil && !*g.ForceCLIOnly {
		return fmt.Errorf("pre_task_completion_gate.force_cli_only must be true (CLI-only force is a v1 invariant)")
	}
	if err := validateGateBinary(g.AutoharnessBinary); err != nil {
		return err
	}
	return nil
}

// validateGateBinary constrains the config-controlled executable the broker
// auto-invokes. hooks.yaml is repo-controlled and auto-loaded, so a path-qualified
// binary would let os/exec run an arbitrary in-repo file (relative to the workspace)
// instead of a PATH lookup — a local code-execution vector. Require a bare
// executable name resolved via PATH: reject absolute paths, ".." traversal, and any
// remaining path component (relative subdir or Windows volume).
func validateGateBinary(bin string) error {
	if bin == "" {
		return fmt.Errorf("pre_task_completion_gate.autoharness_binary must not be empty")
	}
	if filepath.IsAbs(bin) {
		return fmt.Errorf("pre_task_completion_gate.autoharness_binary must not be an absolute path: %q", bin)
	}
	cleaned := filepath.ToSlash(filepath.Clean(bin))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return fmt.Errorf("pre_task_completion_gate.autoharness_binary must not contain '..' traversal: %q", bin)
	}
	if strings.ContainsAny(bin, `/\`) || filepath.VolumeName(bin) != "" {
		return fmt.Errorf("pre_task_completion_gate.autoharness_binary must be a bare executable name resolved via PATH, not a path: %q", bin)
	}
	return nil
}

// NotificationsConfig configures external webhook notification dispatch.
type NotificationsConfig struct {
	Endpoints []WebhookEndpoint `yaml:"endpoints,omitempty"`
	RateLimit int               `yaml:"rate_limit_per_second,omitempty" validate:"omitempty,gte=1,lte=100"`
}

// WebhookEndpoint defines a single webhook notification target.
type WebhookEndpoint struct {
	URL         string            `yaml:"url" validate:"required"`
	EventFilter []string          `yaml:"event_filter,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	TimeoutSecs int               `yaml:"timeout_secs,omitempty" validate:"omitempty,gte=1,lte=60"`
}
