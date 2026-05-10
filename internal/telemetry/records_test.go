package telemetry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// ---- DeriveModelClass -------------------------------------------------------

func TestDeriveModelClass_Sonnet(t *testing.T) {
	assert.Equal(t, "sonnet", telemetry.DeriveModelClass("claude-sonnet-4.6"))
}

func TestDeriveModelClass_Haiku(t *testing.T) {
	assert.Equal(t, "haiku", telemetry.DeriveModelClass("claude-haiku-4.5"))
}

func TestDeriveModelClass_Opus(t *testing.T) {
	assert.Equal(t, "opus", telemetry.DeriveModelClass("claude-opus-4.7"))
}

func TestDeriveModelClass_GPT(t *testing.T) {
	assert.Equal(t, "gpt", telemetry.DeriveModelClass("gpt-5.4"))
}

func TestDeriveModelClass_GPTMini(t *testing.T) {
	assert.Equal(t, "gpt", telemetry.DeriveModelClass("gpt-5.4-mini"))
}

func TestDeriveModelClass_OSeries_o1(t *testing.T) {
	assert.Equal(t, "o-series", telemetry.DeriveModelClass("o1"))
}

func TestDeriveModelClass_OSeries_o3mini(t *testing.T) {
	assert.Equal(t, "o-series", telemetry.DeriveModelClass("o3-mini"))
}

func TestDeriveModelClass_OSeries_o4(t *testing.T) {
	assert.Equal(t, "o-series", telemetry.DeriveModelClass("o4-mini"))
}

func TestDeriveModelClass_Unknown_Fallback(t *testing.T) {
	assert.Equal(t, "other", telemetry.DeriveModelClass("some-unknown-model"))
}

func TestDeriveModelClass_Empty(t *testing.T) {
	assert.Equal(t, "", telemetry.DeriveModelClass(""))
}

// ---- DeriveReasoningLevel ---------------------------------------------------

func TestDeriveReasoningLevel_o1_High(t *testing.T) {
	assert.Equal(t, "high", telemetry.DeriveReasoningLevel("o1"))
}

func TestDeriveReasoningLevel_o3_High(t *testing.T) {
	assert.Equal(t, "high", telemetry.DeriveReasoningLevel("o3"))
}

func TestDeriveReasoningLevel_o4_High(t *testing.T) {
	assert.Equal(t, "high", telemetry.DeriveReasoningLevel("o4"))
}

func TestDeriveReasoningLevel_o1mini_Low(t *testing.T) {
	assert.Equal(t, "low", telemetry.DeriveReasoningLevel("o1-mini"))
}

func TestDeriveReasoningLevel_o3mini_Low(t *testing.T) {
	assert.Equal(t, "low", telemetry.DeriveReasoningLevel("o3-mini"))
}

func TestDeriveReasoningLevel_o4mini_Low(t *testing.T) {
	assert.Equal(t, "low", telemetry.DeriveReasoningLevel("o4-mini"))
}

func TestDeriveReasoningLevel_Sonnet_Empty(t *testing.T) {
	assert.Equal(t, "", telemetry.DeriveReasoningLevel("claude-sonnet-4.6"))
}

func TestDeriveReasoningLevel_GPT_Empty(t *testing.T) {
	assert.Equal(t, "", telemetry.DeriveReasoningLevel("gpt-5.4"))
}

func TestDeriveReasoningLevel_Empty_Empty(t *testing.T) {
	assert.Equal(t, "", telemetry.DeriveReasoningLevel(""))
}

// ---- PrimaryModel -----------------------------------------------------------

func TestPrimaryModel_SingleModel(t *testing.T) {
	m := map[string]int{"claude-sonnet-4.6": 1500}
	assert.Equal(t, "claude-sonnet-4.6", telemetry.PrimaryModel(m))
}

func TestPrimaryModel_MultiModel_ReturnsHighest(t *testing.T) {
	m := map[string]int{
		"claude-sonnet-4.6": 1500,
		"claude-haiku-4.5":  200,
		"gpt-5.4":           400,
	}
	assert.Equal(t, "claude-sonnet-4.6", telemetry.PrimaryModel(m))
}

func TestPrimaryModel_NilMap_ReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", telemetry.PrimaryModel(nil))
}

func TestPrimaryModel_EmptyMap_ReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", telemetry.PrimaryModel(map[string]int{}))
}

func TestPrimaryModel_Tie_DeterministicByName(t *testing.T) {
	// When two models have the same token count, alphabetically first wins.
	m := map[string]int{
		"aaa-model": 1000,
		"zzz-model": 1000,
	}
	assert.Equal(t, "aaa-model", telemetry.PrimaryModel(m))
}

// ---- DeriveBranchType -------------------------------------------------------

func TestDeriveBranchType_Empty(t *testing.T) {
	assert.Equal(t, "", telemetry.DeriveBranchType(""))
}

func TestDeriveBranchType_Feature(t *testing.T) {
	assert.Equal(t, "feature", telemetry.DeriveBranchType("feature/057-f-schema-discoverability"))
}

func TestDeriveBranchType_Stage(t *testing.T) {
	assert.Equal(t, "stage", telemetry.DeriveBranchType("chore/stage-056-s-schema-discoverability"))
}

func TestDeriveBranchType_Ship(t *testing.T) {
	assert.Equal(t, "ship", telemetry.DeriveBranchType("ship/055s-lifecycle-hygiene"))
}

func TestDeriveBranchType_PostMerge(t *testing.T) {
	assert.Equal(t, "post-merge", telemetry.DeriveBranchType("post-merge/autoharness-tune-2026-04-26"))
}

func TestDeriveBranchType_Chore(t *testing.T) {
	assert.Equal(t, "chore", telemetry.DeriveBranchType("chore/copilot-review-fixes-108-109"))
}

func TestDeriveBranchType_Main(t *testing.T) {
	assert.Equal(t, "main", telemetry.DeriveBranchType("main"))
}

func TestDeriveBranchType_Master(t *testing.T) {
	assert.Equal(t, "main", telemetry.DeriveBranchType("master"))
}

func TestDeriveBranchType_Other(t *testing.T) {
	assert.Equal(t, "other", telemetry.DeriveBranchType("release/v1.0"))
}
