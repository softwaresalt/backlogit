package config

import (
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
	ValidateTransition bool                `yaml:"validate_transition"`
	EmitEvents         bool                `yaml:"emit_events"`
	Transitions        map[string][]string `yaml:"transitions,omitempty"`
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
