package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/softwaresalt/backlogit/internal/stash"
	"gopkg.in/yaml.v3"
)

// defaultHeaderDef returns the default HeaderDefConfig for a new workspace.
func defaultHeaderDef() *HeaderDefConfig {
	statusField := &FieldDef{
		Type:    "enum",
		Values:  []string{"queued", "active", "blocked", "review", "done", "accepted", "rejected", "archived"},
		Default: "queued",
	}
	// size + provenance schema (108-F). Size estimation is TASK-ONLY: only the
	// task type carries these fields; features/shipments expose a computed-on-read
	// rollup (SizeComposition) rather than a stored estimate. size is an optional
	// T-shirt enum with no default so legacy tasks without it stay valid.
	// size_source records who authored the estimate. size_ruleset_version is an
	// opaque version identifier for the ruleset that produced an estimate — a
	// stored (never executed) string whose provenance-completeness (a source
	// requires a non-empty ruleset) is enforced at the audited size seam.
	sizeField := &FieldDef{
		Type:     "enum",
		Values:   []string{"XS", "S", "M", "L", "XL"},
		Optional: true,
	}
	sizeSourceField := &FieldDef{
		Type:     "enum",
		Values:   []string{"human", "agent", "derived"},
		Optional: true,
	}
	sizeRulesetVersionField := &FieldDef{
		Type:     "string",
		Optional: true,
	}
	return &HeaderDefConfig{
		Defaults: SystemDefaults{
			ID:          FieldDef{Type: "string", Immutable: true},
			CreatedDate: FieldDef{Type: "datetime", Immutable: true},
			UpdatedDate: FieldDef{Type: "datetime", Immutable: true},
		},
		Types: map[string]*TypeDefConfig{
			"feature": {
				Prefix:   "F",
				Suffix:   "-F",
				IDFormat: "{NNN}{suffix}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"harness_status": {
						Type:     "enum",
						Values:   []string{"pending", "scaffolded", "passing", "failing"},
						Default:  "pending",
						Optional: true,
					},
				},
			},
			"deliberation": {
				Prefix:   "DL",
				Suffix:   "-DL",
				IDFormat: "{NNN}{suffix}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"priority": {
						Type:    "enum",
						Values:  []string{"low", "medium", "high", "critical"},
						Default: "medium",
					},
				},
			},
			"task": {
				Prefix:   "T",
				Suffix:   "-T",
				IDFormat: "{NNN}{suffix}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"priority": {
						Type:    "enum",
						Values:  []string{"low", "medium", "high", "critical"},
						Default: "medium",
					},
					"size":                 sizeField,
					"size_source":          sizeSourceField,
					"size_ruleset_version": sizeRulesetVersionField,
				},
			},
			"review": {
				Prefix:   "R",
				Suffix:   "-R",
				IDFormat: "{NNN}{suffix}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"source_branch": {
						Type:     "string",
						Optional: true,
					},
				},
			},
			"subtask": {
				Prefix:   "ST",
				Suffix:   "-ST",
				IDFormat: "{NNN}{suffix}",
				Fields: map[string]*FieldDef{
					"status": statusField,
				},
			},
			"bug": {
				Prefix:   "B",
				Suffix:   "-B",
				IDFormat: "{NNN}{suffix}",
				Fields: map[string]*FieldDef{
					"status": statusField,
					"severity": {
						Type:    "enum",
						Values:  []string{"low", "medium", "high", "critical"},
						Default: "medium",
					},
				},
			},
			"shipment": {
				Prefix:   "S",
				Suffix:   "-S",
				IDFormat: "{NNN}{suffix}",
				Fields: map[string]*FieldDef{
					"status": {
						Type:    "enum",
						Values:  []string{"queued", "active", "shipped", "abandoned"},
						Default: "queued",
					},
					"branch": {
						Type:     "string",
						Optional: true,
					},
					"items": {
						Type:     "list",
						Optional: true,
					},
				},
			},
		},
	}
}

// defaultTemplates returns the default template content for each artifact type.
func defaultTemplates() map[string]string {
	return map[string]string{
		"feature": `---
name: feature-template
type: feature
description: "A feature-level work item"
sections:
  - name: description
    required: true
    description: "Detailed description of the feature"
  - name: goals
    required: false
    description: "Goals and intended outcomes"
  - name: dod
    required: false
    description: "Definition of Done for this feature"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Goals

<!-- BEGIN:goals -->
<!-- END:goals -->

## Definition of Done

<!-- BEGIN:dod -->
<!-- END:dod -->
`,
		"deliberation": `---
name: deliberation-template
type: deliberation
description: "A collaborative deliberation artifact linked to a stashed idea or issue"
sections:
  - name: problem-frame
    required: true
    description: "The operator and agent's shared understanding of the problem"
  - name: options
    required: false
    description: "Approaches or alternatives considered during deliberation"
  - name: chosen-direction
    required: false
    description: "Selected direction and decision rationale"
  - name: open-questions
    required: false
    description: "Questions or risks that remain unresolved"
  - name: notes
    required: false
    description: "Supporting research, references, or follow-up notes"
---
# {title}

## Problem Frame

<!-- BEGIN:problem-frame -->
<!-- END:problem-frame -->

## Options

<!-- BEGIN:options -->
<!-- END:options -->

## Chosen Direction

<!-- BEGIN:chosen-direction -->
<!-- END:chosen-direction -->

## Open Questions

<!-- BEGIN:open-questions -->
<!-- END:open-questions -->

## Notes

<!-- BEGIN:notes -->
<!-- END:notes -->
`,
		"task": `---
name: task-template
type: task
description: "A discrete unit of work"
sections:
  - name: description
    required: true
    description: "Detailed description of the work item"
  - name: acceptance-criteria
    required: false
    description: "Conditions that must be met for completion"
  - name: implementation-notes
    required: false
    description: "Technical notes and implementation details"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Acceptance Criteria

<!-- BEGIN:acceptance-criteria -->
<!-- END:acceptance-criteria -->

## Implementation Notes

<!-- BEGIN:implementation-notes -->
<!-- END:implementation-notes -->
`,
		"review": `---
name: review-template
type: review
description: "A review artifact tied to a feature branch lifecycle"
sections:
  - name: summary
    required: true
    description: "High-level review outcome and scope"
  - name: findings
    required: false
    description: "Findings, recommendations, and reviewer notes"
  - name: decisions
    required: false
    description: "Disposition of findings and next actions"
---
# {title}

## Summary

<!-- BEGIN:summary -->
<!-- END:summary -->

## Findings

<!-- BEGIN:findings -->
<!-- END:findings -->

## Decisions

<!-- BEGIN:decisions -->
<!-- END:decisions -->
`,
		"subtask": `---
name: subtask-template
type: subtask
description: "A discrete unit of work"
sections:
  - name: description
    required: true
    description: "Detailed description of the discrete work item"
  - name: implementation-notes
    required: false
    description: "Technical notes and implementation details"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Implementation Notes

<!-- BEGIN:implementation-notes -->
<!-- END:implementation-notes -->
`,
		"shipment": `---
name: shipment-template
type: shipment
description: "A shipment artifact representing a branch and pull request scope"
sections:
  - name: description
    required: true
    description: "Detailed description of the shipment scope"
  - name: items
    required: false
    description: "Work items included in this shipment"
  - name: blocked-returns
    required: false
    description: "Items removed from the shipment and returned to backlog"
---
# {title}

## Description

<!-- BEGIN:description -->
<!-- END:description -->

## Items

<!-- BEGIN:items -->
<!-- END:items -->

## Blocked Returns

<!-- BEGIN:blocked-returns -->
<!-- END:blocked-returns -->
`,
	}
}

// DefaultConfig returns a sensible default workspace configuration.
func DefaultConfig() *WorkspaceConfig {
	cfg := defaultConfigBase()
	applyBugLevelConfig(cfg)
	return cfg
}

func defaultConfigBase() *WorkspaceConfig {
	return &WorkspaceConfig{
		BugLevel:            3,
		MaxSlugLength:       60,
		CheckpointRetention: CheckpointRetention{RetentionDays: 7},
		ArtifactTypes: map[string]*ArtifactTypeConfig{
			"feature": {
				Prefix:          "F",
				Suffix:          "-F",
				NameFormat:      "{NNN}{suffix}",
				AllowedChildren: []string{"task", "review"},
			},
			"deliberation": {
				Prefix:     "DL",
				Suffix:     "-DL",
				NameFormat: "{NNN}{suffix}",
			},
			"task": {
				Prefix:          "T",
				Suffix:          "-T",
				NameFormat:      "{NNN}{suffix}",
				AllowedChildren: []string{"subtask"},
			},
			"review": {
				Prefix:         "R",
				Suffix:         "-R",
				NameFormat:     "{NNN}{suffix}",
				FileNameFormat: "{id}-{title_slug}",
			},
			"subtask": {
				Prefix:     "ST",
				Suffix:     "-ST",
				NameFormat: "{NNN}{suffix}",
			},
			"bug": {
				Prefix:     "B",
				Suffix:     "-B",
				NameFormat: "{NNN}{suffix}",
			},
			"shipment": {
				Prefix:     "S",
				Suffix:     "-S",
				NameFormat: "{NNN}{suffix}",
			},
		},
		Fields: map[string]*FieldConfig{
			"status": {
				Type:    "enum",
				Values:  []string{"queued", "active", "blocked", "review", "done", "accepted", "rejected", "archived"},
				Default: "queued",
			},
		},
		QueueLayout: &QueueLayoutConfig{
			RootDir:    "queue",
			NameFormat: "{NNN}",
			Levels: []HierarchyLevel{
				{Level: 1, Types: []string{"feature", "deliberation", "shipment"}},
				{Level: 2, Types: []string{"task", "review"}},
				{Level: 3, Types: []string{"subtask"}},
			},
		},
	}
}

// DefaultRegistry returns default directory routing rules.
// Status-based rules are listed first so they take priority over type-based rules.
func DefaultRegistry() *RegistryConfig {
	return &RegistryConfig{
		Directories: []DirectoryRule{
			{Path: "archive", Condition: DirectoryCondition{Status: []string{"done", "accepted", "rejected", "archived"}}},
			{Path: "queue", Condition: DirectoryCondition{Status: []string{"queued", "active", "blocked", "review"}}},
		},
	}
}

// WriteDefaults serializes DefaultConfig and DefaultRegistry to the workspace directory.
// Also writes header-def.yaml and default templates. Existing files are not overwritten.
func WriteDefaults(workspacePath string) error {
	cfgData, err := yaml.Marshal(defaultConfigBase())
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	regData, err := yaml.Marshal(DefaultRegistry())
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}
	headerDefData, err := yaml.Marshal(defaultHeaderDef())
	if err != nil {
		return fmt.Errorf("marshal header-def: %w", err)
	}

	if err := writeFileIfNotExists(filepath.Join(workspacePath, "config.yaml"), cfgData); err != nil {
		return fmt.Errorf("write config.yaml: %w", err)
	}
	if err := writeFileIfNotExists(filepath.Join(workspacePath, "registry.yaml"), regData); err != nil {
		return fmt.Errorf("write registry.yaml: %w", err)
	}
	if err := writeFileIfNotExists(filepath.Join(workspacePath, "header-def.yaml"), headerDefData); err != nil {
		return fmt.Errorf("write header-def.yaml: %w", err)
	}

	// Create templates directory and write default template files.
	templatesDir := filepath.Join(workspacePath, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		return fmt.Errorf("create templates dir: %w", err)
	}
	for typeName, content := range defaultTemplates() {
		path := filepath.Join(templatesDir, typeName+".md")
		if err := writeFileIfNotExists(path, []byte(content)); err != nil {
			return fmt.Errorf("write template %s: %w", typeName, err)
		}
	}

	queueDir := filepath.Join(workspacePath, "queue")
	if err := os.MkdirAll(queueDir, 0o755); err != nil {
		return fmt.Errorf("create queue dir: %w", err)
	}
	if err := writeFileIfNotExists(filepath.Join(workspacePath, stash.JSONLFileName), []byte("")); err != nil {
		return fmt.Errorf("write %s: %w", stash.JSONLFileName, err)
	}

	hooksData, err := defaultHooksYAML()
	if err != nil {
		return fmt.Errorf("marshal hooks.yaml: %w", err)
	}
	if err := writeFileIfNotExists(filepath.Join(workspacePath, "hooks.yaml"), hooksData); err != nil {
		return fmt.Errorf("write hooks.yaml: %w", err)
	}

	return nil
}

// DefaultHooksConfig returns the default v1 HooksConfig with blocked_stale threshold
// and subscriptions for the Stage and Ship agents.
func DefaultHooksConfig() *HooksConfig {
	return &HooksConfig{
		Enabled: true,
		EventThresholds: HookEventThresholds{
			BlockedStaleDays: 7,
		},
		AgentSubscriptions: map[string][]string{
			"stage": {"feature_review_ready", "blocked_stale"},
			"ship":  {"post_merge_closure", "feature_review_ready"},
		},
		Lifecycle: LifecycleHooksConfig{
			ValidateTransition: true,
			EmitEvents:         true,
			Transitions: map[string][]string{
				"queued":  {"active", "blocked"},
				"active":  {"done", "blocked", "review", "shipped", "abandoned"},
				"blocked": {"active"},
				"review":  {"done", "accepted", "rejected"},
				"done":    {"archived"},
			},
			PreTaskCompletionGate: defaultPreTaskCompletionGate(),
		},
		Notifications: NotificationsConfig{
			RateLimit: 10,
		},
	}
}

// defaultPreTaskCompletionGate returns the default v1 gate broker config
// (enabled:auto so a repo without configured autoharness gates still completes).
func defaultPreTaskCompletionGate() PreTaskCompletionGateConfig {
	g := PreTaskCompletionGateConfig{}
	g.Normalize()
	return g
}

// defaultHooksYAML marshals the default HooksConfig to YAML bytes.
func defaultHooksYAML() ([]byte, error) {
	data, err := yaml.Marshal(DefaultHooksConfig())
	if err != nil {
		return nil, fmt.Errorf("marshal hooks config: %w", err)
	}
	return data, nil
}

// writeFileIfNotExists writes data to path only if the file does not already exist.
func writeFileIfNotExists(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil // file already exists; skip
	}
	return os.WriteFile(path, data, 0o644)
}
