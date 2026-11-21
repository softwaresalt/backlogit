package integration_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type gsdTierRoute struct {
	Model           string `yaml:"model"`
	ModelProvider   string `yaml:"model_provider"`
	ModelFamily     string `yaml:"model_family"`
	ReasoningEffort string `yaml:"reasoning_effort"`
	ContextTier     string `yaml:"context_tier"`
}

type gsdRoleRoute struct {
	ModelProvider   string `yaml:"model_provider"`
	ModelFamily     string `yaml:"model_family"`
	ReasoningEffort string `yaml:"reasoning_effort"`
	ContextTier     string `yaml:"context_tier"`
}

type gsdEscalationCapableRoute struct {
	ModelProvider   string       `yaml:"model_provider"`
	ModelFamily     string       `yaml:"model_family"`
	ReasoningEffort string       `yaml:"reasoning_effort"`
	ContextTier     string       `yaml:"context_tier"`
	Escalation      gsdRoleRoute `yaml:"escalation"`
}

type gsdAnchorReviewRoute struct {
	ModelProvider   string `yaml:"model_provider"`
	ModelFamily     string `yaml:"model_family"`
	ReasoningEffort string `yaml:"reasoning_effort"`
}

type gsdAltDocReviewRoute struct {
	ModelProvider string `yaml:"model_provider"`
	ModelFamily   string `yaml:"model_family"`
}

type gsdModelRouting struct {
	Tier1        gsdTierRoute              `yaml:"tier1"`
	Tier2        gsdTierRoute              `yaml:"tier2"`
	Tier3        gsdTierRoute              `yaml:"tier3"`
	Orchestrator gsdRoleRoute              `yaml:"orchestrator"`
	Stage        gsdEscalationCapableRoute `yaml:"stage"`
	Ship         gsdEscalationCapableRoute `yaml:"ship"`
	AnchorReview gsdAnchorReviewRoute      `yaml:"anchor_review"`
	AltDocReview gsdAltDocReviewRoute      `yaml:"alt_doc_review"`
}

type gsdConfigDoc struct {
	ModelRouting gsdModelRouting `yaml:"model_routing"`
}

type manifestArtifact struct {
	Path         string `yaml:"path"`
	Checksum     string `yaml:"checksum"`
	DriftAllowed bool   `yaml:"drift_allowed"`
	DriftReason  string `yaml:"drift_reason"`
	Note         string `yaml:"note"`
}

type harnessManifestDoc struct {
	SchemaVersion string             `yaml:"schema_version"`
	TunedAt       string             `yaml:"tuned_at"`
	ConfigHash    string             `yaml:"config_hash"`
	VariablesUsed map[string]string  `yaml:"variables_used"`
	Artifacts     []manifestArtifact `yaml:"artifacts"`
}

type agentFrontmatterDoc struct {
	Name                        string `yaml:"name"`
	Tools                       string `yaml:"tools"`
	ModelTier                   int    `yaml:"model_tier"`
	MaxSubagentTier             int    `yaml:"max_subagent_tier"`
	ReasoningEffort             string `yaml:"reasoning_effort"`
	ModelProvider               string `yaml:"model_provider"`
	ModelFamily                 string `yaml:"model_family"`
	ContextTier                 string `yaml:"context_tier"`
	AnchorReviewProvider        string `yaml:"anchor_review_provider"`
	AnchorReviewFamily          string `yaml:"anchor_review_family"`
	AnchorReviewReasoningEffort string `yaml:"anchor_review_reasoning_effort"`
}

func decodeUniqueYAML[T any](t *testing.T, surfaceName, raw string) T {
	t.Helper()

	var node yaml.Node
	require.NoError(t, yaml.Unmarshal([]byte(raw), &node), "%s must parse as YAML", surfaceName)
	require.NoError(t, checkUniqueYAMLMappingKeys(&node), "%s must not contain duplicate YAML mapping keys", surfaceName)

	var out T
	require.NoError(t, yaml.Unmarshal([]byte(raw), &out), "%s must decode into target struct", surfaceName)
	return out
}

func extractAgentFrontmatterYAML(t *testing.T, relPath, content string) string {
	t.Helper()

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	require.True(t, strings.HasPrefix(normalized, "---\n"), "%s must start with YAML frontmatter delimiter", relPath)
	rest := normalized[len("---\n"):]
	endIdx := strings.Index(rest, "\n---\n")
	require.NotEqual(t, -1, endIdx, "%s must contain closing YAML frontmatter delimiter", relPath)
	return rest[:endIdx]
}

func extractMarkdownH2Section(t *testing.T, relPath, content, heading string) string {
	t.Helper()

	marker := "\n" + heading + "\n"
	startIdx := strings.Index(content, marker)
	require.NotEqual(t, -1, startIdx, "%s must contain heading %q", relPath, heading)
	section := content[startIdx+1:]
	if nextIdx := strings.Index(section[len(heading):], "\n## "); nextIdx != -1 {
		return section[:len(heading)+nextIdx]
	}
	return section
}

func parseCommaTools(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func TestU12_GSDModelRoutingManifestReconciliation(t *testing.T) {
	repoRoot := testRepoRoot(t)
	staleTokens := []string{
		"gemini-4-argon",
		"claude-sonnet-5.5",
		"gpt-6.1-sol",
		"gemini-3.8-flash",
	}

	t.Run("config_and_manifest_variables_reconciled", func(t *testing.T) {
		var dupDoc yaml.Node
		require.NoError(t, yaml.Unmarshal([]byte("a: 1\na: 2\n"), &dupDoc))
		require.Error(t, checkUniqueYAMLMappingKeys(&dupDoc), "duplicate YAML key must be rejected")

		rawConfig := readGovernedSurface(t, repoRoot, ".gsd/config.yaml")
		cfg := decodeUniqueYAML[gsdConfigDoc](t, ".gsd/config.yaml", rawConfig)

		assert.Equal(t, gsdTierRoute{
			Model:           "claude-haiku-5.5",
			ModelProvider:   "anthropic",
			ModelFamily:     "claude-haiku-5.5",
			ReasoningEffort: "high",
			ContextTier:     "long_context",
		}, cfg.ModelRouting.Tier1)
		assert.Equal(t, gsdTierRoute{
			Model:           "mai-code-2",
			ModelProvider:   "microsoft",
			ModelFamily:     "mai-code-2",
			ReasoningEffort: "high",
			ContextTier:     "long_context",
		}, cfg.ModelRouting.Tier2)
		assert.Equal(t, gsdTierRoute{
			Model:           "gemini-4-carbon",
			ModelProvider:   "google",
			ModelFamily:     "gemini-4-carbon",
			ReasoningEffort: "high",
			ContextTier:     "long_context",
		}, cfg.ModelRouting.Tier3)
		assert.Equal(t, gsdRoleRoute{
			ModelProvider:   "google",
			ModelFamily:     "gemini-4-carbon",
			ReasoningEffort: "medium",
			ContextTier:     "long_context",
		}, cfg.ModelRouting.Orchestrator)
		assert.Equal(t, gsdEscalationCapableRoute{
			ModelProvider:   "google",
			ModelFamily:     "gemini-4-carbon",
			ReasoningEffort: "medium",
			ContextTier:     "long_context",
			Escalation: gsdRoleRoute{
				ModelProvider:   "openai",
				ModelFamily:     "gpt-6.2-sol",
				ReasoningEffort: "high",
				ContextTier:     "long_context",
			},
		}, cfg.ModelRouting.Stage)
		assert.Equal(t, gsdEscalationCapableRoute{
			ModelProvider:   "google",
			ModelFamily:     "gemini-4-hydrogen",
			ReasoningEffort: "high",
			ContextTier:     "long_context",
			Escalation: gsdRoleRoute{
				ModelProvider:   "openai",
				ModelFamily:     "gpt-6.2-sol",
				ReasoningEffort: "medium",
				ContextTier:     "long_context",
			},
		}, cfg.ModelRouting.Ship)
		assert.Equal(t, gsdAnchorReviewRoute{
			ModelProvider:   "openai",
			ModelFamily:     "gpt-6.2-sol",
			ReasoningEffort: "high",
		}, cfg.ModelRouting.AnchorReview)
		assert.Equal(t, gsdAltDocReviewRoute{
			ModelProvider: "google",
			ModelFamily:   "gemini-4-hydrogen",
		}, cfg.ModelRouting.AltDocReview)

		rawManifest := readGovernedSurface(t, repoRoot, ".gsd/harness-manifest.yaml")
		manifest := decodeUniqueYAML[harnessManifestDoc](t, ".gsd/harness-manifest.yaml", rawManifest)

		assert.NotEqual(t, "2b114a784fdf277e9552280767f798d706695eb8d5e191573486409011096157", manifest.ConfigHash,
			"config_hash must be rotated from the pre-219-F hash")
		assert.Equal(t, lfSHA256Hex(rawConfig), manifest.ConfigHash,
			"config_hash must match LF-normalized .gsd/config.yaml")
		_, err := time.Parse(time.RFC3339, manifest.TunedAt)
		assert.NoError(t, err, "tuned_at must be a valid RFC3339 timestamp")

		expectedVars := map[string]string{
			"MODEL_ROUTING_TIER1":               cfg.ModelRouting.Tier1.ModelFamily,
			"MODEL_ROUTING_TIER2":               cfg.ModelRouting.Tier2.ModelFamily,
			"MODEL_ROUTING_TIER3":               cfg.ModelRouting.Tier3.ModelFamily,
			"ANCHOR_REVIEW_PROVIDER":            cfg.ModelRouting.AnchorReview.ModelProvider,
			"ANCHOR_REVIEW_FAMILY":              cfg.ModelRouting.AnchorReview.ModelFamily,
			"ANCHOR_REVIEW_REASONING_EFFORT":    cfg.ModelRouting.AnchorReview.ReasoningEffort,
			"ALT_DOC_REVIEW_PROVIDER":           cfg.ModelRouting.AltDocReview.ModelProvider,
			"ALT_DOC_REVIEW_FAMILY":             cfg.ModelRouting.AltDocReview.ModelFamily,
			"ESCALATION_FAMILY":                 cfg.ModelRouting.Tier3.ModelFamily,
			"ESCALATION_PROVIDER":               cfg.ModelRouting.Tier3.ModelProvider,
			"ESCALATION_REASONING_EFFORT":       cfg.ModelRouting.Tier3.ReasoningEffort,
			"STAGE_ESCALATION_FAMILY":           cfg.ModelRouting.Stage.Escalation.ModelFamily,
			"STAGE_ESCALATION_PROVIDER":         cfg.ModelRouting.Stage.Escalation.ModelProvider,
			"STAGE_ESCALATION_REASONING_EFFORT": cfg.ModelRouting.Stage.Escalation.ReasoningEffort,
			"STAGE_ESCALATION_CONTEXT_TIER":     cfg.ModelRouting.Stage.Escalation.ContextTier,
			"SHIP_ESCALATION_FAMILY":            cfg.ModelRouting.Ship.Escalation.ModelFamily,
			"SHIP_ESCALATION_PROVIDER":          cfg.ModelRouting.Ship.Escalation.ModelProvider,
			"SHIP_ESCALATION_REASONING_EFFORT":  cfg.ModelRouting.Ship.Escalation.ReasoningEffort,
			"SHIP_ESCALATION_CONTEXT_TIER":      cfg.ModelRouting.Ship.Escalation.ContextTier,
		}
		for key, want := range expectedVars {
			got, exists := manifest.VariablesUsed[key]
			assert.True(t, exists, "variables_used must define %s", key)
			assert.Equal(t, want, got, "variables_used[%s] must match .gsd/config.yaml", key)
		}
	})

	t.Run("tiered_agents_doc_review_and_adversarial_review_aligned", func(t *testing.T) {
		rawConfig := readGovernedSurface(t, repoRoot, ".gsd/config.yaml")
		cfg := decodeUniqueYAML[gsdConfigDoc](t, ".gsd/config.yaml", rawConfig)

		rawManifest := readGovernedSurface(t, repoRoot, ".gsd/harness-manifest.yaml")
		manifest := decodeUniqueYAML[harnessManifestDoc](t, ".gsd/harness-manifest.yaml", rawManifest)

		for key, val := range manifest.VariablesUsed {
			for _, stale := range staleTokens {
				assert.NotContains(t, val, stale, "active variables_used[%s] must not contain stale token %q", key, stale)
			}
		}

		type expectedAgentRoute struct {
			path            string
			tier            int
			provider        string
			family          string
			reasoningEffort string
			contextTier     string
			readOnlyLeaf    bool
			enumeratedKBDB  bool
			checkAnchor     bool
		}

		tieredAgents := []expectedAgentRoute{
			{
				path:            ".github/agents/_orchestrator.agent.md",
				tier:            2,
				provider:        cfg.ModelRouting.Orchestrator.ModelProvider,
				family:          cfg.ModelRouting.Orchestrator.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Orchestrator.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Orchestrator.ContextTier,
				enumeratedKBDB:  true,
			},
			{
				path:            ".github/agents/_stage.agent.md",
				tier:            3,
				provider:        cfg.ModelRouting.Stage.ModelProvider,
				family:          cfg.ModelRouting.Stage.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Stage.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Stage.ContextTier,
				enumeratedKBDB:  true,
			},
			{
				path:            ".github/agents/_ship.agent.md",
				tier:            2,
				provider:        cfg.ModelRouting.Ship.ModelProvider,
				family:          cfg.ModelRouting.Ship.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Ship.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Ship.ContextTier,
			},
			{
				path:            ".github/agents/review/adversarial-review.agent.md",
				tier:            3,
				provider:        cfg.ModelRouting.Tier3.ModelProvider,
				family:          cfg.ModelRouting.Tier3.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier3.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier3.ContextTier,
				checkAnchor:     true,
			},
			{
				path:            ".github/agents/subagents/go-engineer.agent.md",
				tier:            2,
				provider:        cfg.ModelRouting.Tier2.ModelProvider,
				family:          cfg.ModelRouting.Tier2.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier2.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier2.ContextTier,
			},
			{
				path:            ".github/agents/review/security-lens-reviewer.agent.md",
				tier:            2,
				provider:        cfg.ModelRouting.Tier2.ModelProvider,
				family:          cfg.ModelRouting.Tier2.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier2.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier2.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/security-reviewer.agent.md",
				tier:            2,
				provider:        cfg.ModelRouting.Tier2.ModelProvider,
				family:          cfg.ModelRouting.Tier2.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier2.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier2.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/security-sentinel.agent.md",
				tier:            3,
				provider:        cfg.ModelRouting.Tier3.ModelProvider,
				family:          cfg.ModelRouting.Tier3.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier3.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier3.ContextTier,
			},
			{
				path:            ".github/agents/subagents/security-sentinel.agent.md",
				tier:            3,
				provider:        cfg.ModelRouting.Tier3.ModelProvider,
				family:          cfg.ModelRouting.Tier3.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier3.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier3.ContextTier,
			},
			{
				path:            ".github/agents/research/learnings-researcher.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/subagents/prompt-builder.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
			},
			{
				path:            ".github/agents/review/agent-native-parity-reviewer.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/architecture-strategist.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/concurrency-reviewer.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/constitution-reviewer.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/correctness-reviewer.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/go-quality-reviewer.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/maintainability-reviewer.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/schema-cli-docs-coupling-reviewer.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/scope-boundary-auditor.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
			{
				path:            ".github/agents/review/template-integrity-reviewer.agent.md",
				tier:            1,
				provider:        cfg.ModelRouting.Tier1.ModelProvider,
				family:          cfg.ModelRouting.Tier1.ModelFamily,
				reasoningEffort: cfg.ModelRouting.Tier1.ReasoningEffort,
				contextTier:     cfg.ModelRouting.Tier1.ContextTier,
				readOnlyLeaf:    true,
			},
		}

		expectedKBDBVerbs := []string{
			"kbdb/search_local_docs",
			"kbdb/search_semantic",
			"kbdb/research_topic",
			"kbdb/traverse_doc_links",
			"kbdb/list_sources",
			"kbdb/get_chunk_by_id",
			"kbdb/get_document",
			"kbdb/get_status",
		}

		for _, spec := range tieredAgents {
			content := readGovernedSurface(t, repoRoot, spec.path)
			frontRaw := extractAgentFrontmatterYAML(t, spec.path, content)
			fm := decodeUniqueYAML[agentFrontmatterDoc](t, spec.path+" frontmatter", frontRaw)

			assert.Equal(t, spec.tier, fm.ModelTier, "%s model_tier mismatch", spec.path)
			assert.Equal(t, spec.provider, fm.ModelProvider, "%s model_provider mismatch", spec.path)
			assert.Equal(t, spec.family, fm.ModelFamily, "%s model_family mismatch", spec.path)
			assert.Equal(t, spec.reasoningEffort, fm.ReasoningEffort, "%s reasoning_effort mismatch", spec.path)
			assert.Equal(t, spec.contextTier, fm.ContextTier, "%s context_tier mismatch", spec.path)

			for _, stale := range staleTokens {
				assert.NotContains(t, frontRaw, stale, "%s frontmatter must not contain stale token %q", spec.path, stale)
			}

			if spec.checkAnchor {
				assert.Equal(t, cfg.ModelRouting.AnchorReview.ModelProvider, fm.AnchorReviewProvider,
					"%s anchor_review_provider mismatch", spec.path)
				assert.Equal(t, cfg.ModelRouting.AnchorReview.ModelFamily, fm.AnchorReviewFamily,
					"%s anchor_review_family mismatch", spec.path)
				assert.Equal(t, cfg.ModelRouting.AnchorReview.ReasoningEffort, fm.AnchorReviewReasoningEffort,
					"%s anchor_review_reasoning_effort mismatch", spec.path)
			}

			tools := parseCommaTools(fm.Tools)
			if spec.readOnlyLeaf {
				assert.Equal(t, []string{"read", "search"}, tools,
					"%s read-only leaf persona must retain explicit read, search tools", spec.path)
			}
			if spec.enumeratedKBDB {
				assert.NotContains(t, tools, "kbdb/*", "%s must not grant kbdb/* wildcard", spec.path)
				for _, verb := range expectedKBDBVerbs {
					assert.Contains(t, tools, verb, "%s must retain enumerated %s tool", spec.path, verb)
				}
			}
		}

		orchContent := readGovernedSurface(t, repoRoot, ".github/agents/_orchestrator.agent.md")
		orchRouting := extractMarkdownH2Section(t, ".github/agents/_orchestrator.agent.md", orchContent, "## Model Routing")
		for _, want := range []string{"mai-code-2", "gemini-4-carbon", "gemini-4-hydrogen", "openai"} {
			assert.True(t, strings.Contains(orchRouting, want), "_orchestrator.agent.md ## Model Routing must reference %s", want)
		}
		for _, forbidden := range []string{"context_tier: default", "claude-sonnet-5.5", "gemini-4-argon"} {
			assert.False(t, strings.Contains(orchRouting, forbidden), "_orchestrator.agent.md ## Model Routing must not contain %q", forbidden)
		}

		stageContent := readGovernedSurface(t, repoRoot, ".github/agents/_stage.agent.md")
		assert.False(t, strings.Contains(stageContent, "gemini-4-argon"), "_stage.agent.md must contain zero gemini-4-argon")

		advAgentContent := readGovernedSurface(t, repoRoot, ".github/agents/review/adversarial-review.agent.md")
		advInstrContent := readGovernedSurface(t, repoRoot, ".github/instructions/adversarial-review.instructions.md")
		staleGPT61 := regexp.MustCompile(`(?i)gpt-6\.1`)

		for _, pair := range []struct {
			path    string
			content string
		}{
			{".github/agents/review/adversarial-review.agent.md", advAgentContent},
			{".github/instructions/adversarial-review.instructions.md", advInstrContent},
		} {
			assert.True(t, strings.Contains(pair.content, "gpt-6.2-sol"), "%s must reference gpt-6.2-sol", pair.path)
			assert.True(t, strings.Contains(pair.content, "GPT-6.2 Sol"), "%s must reference GPT-6.2 Sol", pair.path)
			assert.False(t, staleGPT61.MatchString(pair.content), "%s must contain zero case-insensitive gpt-6.1 tokens", pair.path)
			for _, stale := range staleTokens {
				assert.False(t, strings.Contains(pair.content, stale), "%s must not contain stale token %q", pair.path, stale)
			}
		}
		assert.True(t, strings.Contains(advAgentContent, "mai-code-2"), "adversarial-review.agent.md routing table must use mai-code-2 (Tier 2)")
		assert.True(t, strings.Contains(advAgentContent, "gemini-4-carbon"), "adversarial-review.agent.md routing table must use gemini-4-carbon (Tier 3)")

		docReviewContent := readGovernedSurface(t, repoRoot, ".github/skills/doc-review/SKILL.md")
		assert.True(t, strings.Contains(docReviewContent, "gemini-4-hydrogen"), "doc-review/SKILL.md must reference gemini-4-hydrogen")
		for _, stale := range staleTokens {
			assert.False(t, strings.Contains(docReviewContent, stale), "doc-review/SKILL.md must contain zero %q occurrences", stale)
		}
	})

	t.Run("manifest_checksums_and_drift_records_intact", func(t *testing.T) {
		rawManifest := readGovernedSurface(t, repoRoot, ".gsd/harness-manifest.yaml")
		manifest := decodeUniqueYAML[harnessManifestDoc](t, ".gsd/harness-manifest.yaml", rawManifest)
		require.NotEmpty(t, manifest.Artifacts, "harness-manifest.yaml must list artifacts")

		assert.True(t, strings.Contains(rawManifest, "DOCS_PLANS corrected to docs/exec-plans per workspace config."),
			"historical tuning_history note must be preserved")

		checksumPattern := regexp.MustCompile(`^[0-9a-f]{64}$`)
		byPath := make(map[string]manifestArtifact, len(manifest.Artifacts))
		for _, entry := range manifest.Artifacts {
			require.NotEmpty(t, entry.Path, "manifest artifact path must be non-empty")
			_, dup := byPath[entry.Path]
			require.False(t, dup, "manifest must not contain duplicate artifact path %s", entry.Path)
			byPath[entry.Path] = entry

			assert.Regexp(t, checksumPattern, entry.Checksum,
				"manifest checksum must be 64 lowercase hex characters for %s", entry.Path)
			diskContent := readGovernedSurface(t, repoRoot, entry.Path)
			assert.Equal(t, lfSHA256Hex(diskContent), entry.Checksum,
				"manifest checksum must match LF-normalized %s", entry.Path)
		}

		preservedDrift := []struct {
			path      string
			substring string
		}{
			{".github/agents/_orchestrator.agent.md", "Do not auto-revert."},
			{".github/agents/_stage.agent.md", "Do not auto-revert."},
			{".github/agents/_ship.agent.md", "Do not auto-revert."},
			{".github/agents/review/adversarial-review.agent.md", "Do not auto-revert."},
			{".github/instructions/adversarial-review.instructions.md", "Do not auto-revert."},
			{".github/skills/doc-review/SKILL.md", "Do not auto-revert."},
			{".github/agents/subagents/prompt-builder.agent.md", "Do not auto-revert"},
			{".github/agents/subagents/go-engineer.agent.md", "Do not auto-revert"},
			{".github/agents/review/correctness-reviewer.agent.md", "Do not auto-revert."},
			{".github/agents/review/maintainability-reviewer.agent.md", "Do not auto-revert."},
			{".github/agents/review/security-sentinel.agent.md", "2026-10-04 operator directive: backlogit/* granted"},
		}
		for _, item := range preservedDrift {
			entry, exists := byPath[item.path]
			require.True(t, exists, "manifest entry must exist for %s", item.path)
			assert.True(t, entry.DriftAllowed, "drift_allowed must remain true for %s", item.path)
			assert.Contains(t, entry.DriftReason, item.substring,
				"drift_reason for %s must preserve %q", item.path, item.substring)
		}
	})
}
