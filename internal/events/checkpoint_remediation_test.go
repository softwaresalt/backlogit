package events

import (
	"go/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestU1d_RemediationIntentCarrierDeclared asserts checkpoint_schema.go
// declares the exported, non-executable RemediationIntent carrier (147-F /
// U1d, cycle-17 gate finding H1). This is a source-shape harness: it parses
// the pre-delta file and asserts through go/ast, referencing no undeclared
// identifier, so it compiles before the declaration exists and fails on an
// assertion rather than a build error.
func TestU1d_RemediationIntentCarrierDeclared(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_schema.go")
	typeSpec := findPackageTypeIn(file, "RemediationIntent")
	if !assert.NotNil(t, typeSpec, "RemediationIntent struct is not declared in checkpoint_schema.go") {
		return
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !assert.True(t, ok, "RemediationIntent must be a struct type") {
		return
	}
	wantTags := map[string]string{
		"Verb":             `"verb"`,
		"TargetFilename":   `"target_filename"`,
		"RequiresApproval": `"requires_approval"`,
		"ApprovalClass":    `"approval_class"`,
		"Reason":           `"reason"`,
	}
	found := map[string]bool{}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			if _, want := wantTags[name.Name]; want && field.Tag != nil {
				found[name.Name] = true
			}
		}
	}
	for name := range wantTags {
		assert.True(t, found[name], "RemediationIntent is missing declared field %q", name)
	}
}

// TestU1d_CheckpointSummaryCarriesIntentField asserts CheckpointSummary
// carries a RemediationIntent field tagged json:"remediation_intent" with no
// omitempty elision, so a nil intent marshals as null rather than being
// dropped.
func TestU1d_CheckpointSummaryCarriesIntentField(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_schema.go")
	tag := findStructFieldTag(file, "CheckpointSummary", "RemediationIntent")
	if !assert.NotEmpty(t, tag, "CheckpointSummary has no field tagged json:\"remediation_intent\"") {
		return
	}
	assert.Contains(t, tag, `json:"remediation_intent"`,
		"CheckpointSummary.RemediationIntent must be tagged exactly json:\"remediation_intent\" (no omitempty)")
	assert.NotContains(t, tag, "omitempty",
		"CheckpointSummary.RemediationIntent must not elide the key via omitempty")
}

// TestU1d_RemediationIntentHoldsNoShellText asserts RemediationIntent's field
// set is exactly the five structured keys the plan declares, so no field can
// carry a paste-runnable shell command string.
func TestU1d_RemediationIntentHoldsNoShellText(t *testing.T) {
	file := parseEventsSource(t, "checkpoint_schema.go")
	typeSpec := findPackageTypeIn(file, "RemediationIntent")
	if !assert.NotNil(t, typeSpec, "RemediationIntent struct is not declared in checkpoint_schema.go") {
		return
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !assert.True(t, ok, "RemediationIntent must be a struct type") {
		return
	}
	allowed := map[string]bool{
		"Verb": true, "TargetFilename": true, "RequiresApproval": true,
		"ApprovalClass": true, "Reason": true,
	}
	for _, field := range structType.Fields.List {
		for _, name := range field.Names {
			assert.True(t, allowed[name.Name],
				"RemediationIntent field %q is not one of the five structured keys; it must not carry shell text", name.Name)
		}
	}
}
