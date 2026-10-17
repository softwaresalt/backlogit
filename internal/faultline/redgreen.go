package faultline

// STRUCTURAL STUB for the red phase (S9 / 161.003-T). The v2 manifest, the
// family registration and the real CanonicalMap/Validate land in the green
// commit.

import "fmt"

// EvidenceSchemaVersionV2 is the schema version whose closed family set is
// {parity, red_green}. EvidenceSchemaVersion (v1, {parity}) is immutable.
const EvidenceSchemaVersionV2 = 2

// NodeFamilyRedGreen is the v2 node family for red-test honesty evidence (S9).
const NodeFamilyRedGreen = "red_green"

// RedGreenTest is one test's observed baseline (red) and head (green) result.
type RedGreenTest struct {
	Package          string   `json:"package"`
	Test             string   `json:"test"`
	BaselineResult   string   `json:"baseline_result"`
	HeadResult       string   `json:"head_result"`
	Honest           bool     `json:"honest"`
	RejectionClasses []string `json:"rejection_classes"`
}

// RedGreenEvidence is the v2 "red_green" family payload.
type RedGreenEvidence struct {
	BaseSHA string         `json:"base_sha"`
	HeadSHA string         `json:"head_sha"`
	Tests   []RedGreenTest `json:"tests"`
}

// CanonicalMap implements FamilyPayload.
func (p *RedGreenEvidence) CanonicalMap() map[string]any { return map[string]any{} }

// Validate implements FamilyPayload.
func (p *RedGreenEvidence) Validate(EvidenceView) error {
	return fmt.Errorf("%w: red_green not implemented", ErrValidation)
}
