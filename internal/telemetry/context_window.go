package telemetry

// ModelContextLimits maps model identifiers to their maximum context window
// size in tokens. Used to derive utilisation metrics when raw context window
// data is unavailable from log sources.
//
// Unknown models fall back to DefaultContextLimit. The table is a single var
// declaration and trivially extensible as new models ship.
var ModelContextLimits = map[string]int{
	"claude-sonnet-4":   200000,
	"claude-sonnet-4.5": 200000,
	"claude-haiku-4.5":  200000,
	"claude-opus-4":     200000,
	"claude-opus-4.5":   200000,
	"gpt-4.1":           1000000,
	"gpt-4.1-mini":      500000,
	"gpt-5":             1000000,
	"gpt-5.1":           1000000,
	"o4-mini":           200000,
}

// DefaultContextLimit is the conservative fallback used for unknown models.
const DefaultContextLimit = 200000

// ContextWindowMetrics holds derived context utilisation for a single session.
type ContextWindowMetrics struct {
	// PeakUtilization is the highest prompt_tokens/max_tokens ratio observed
	// across all model calls in the session (0.0–1.0+).
	PeakUtilization float64 `json:"peak_utilization"`
	// RemainingCapacity is max_tokens - peak_prompt_tokens at the point of peak
	// utilisation.
	RemainingCapacity int `json:"remaining_capacity"`
	// DepletionRate is total_tokens / model_calls (average tokens consumed per
	// model turn).
	DepletionRate float64 `json:"depletion_rate"`
	// MaxContextTokens is the model-limit value used for the calculations.
	MaxContextTokens int `json:"max_context_tokens"`
	// PeakPromptTokens is the highest prompt token count observed in any single
	// model call.
	PeakPromptTokens int `json:"peak_prompt_tokens"`
	// CompactionCount is the number of context-compaction events recorded for
	// this session.
	CompactionCount int `json:"compaction_count"`
}

// ComputeContextMetrics derives context window utilisation from a slice of
// model calls and compaction events. Returns nil when modelCalls is empty to
// avoid division by zero and to distinguish "no data" from "zero utilisation".
func ComputeContextMetrics(modelCalls []ModelCall, compactionEvents []CompactionEvent) *ContextWindowMetrics {
	if len(modelCalls) == 0 {
		return nil
	}
	// Find the model call with the highest prompt token count.
	peakIdx := 0
	for i, mc := range modelCalls {
		if mc.PromptTokens > modelCalls[peakIdx].PromptTokens {
			peakIdx = i
		}
	}
	peakCall := modelCalls[peakIdx]
	maxTokens := ContextLimitForModel(peakCall.Model)
	peakPrompt := peakCall.PromptTokens

	// Depletion rate = average total tokens consumed per model call.
	totalSum := 0
	for _, mc := range modelCalls {
		totalSum += mc.TotalTokens
	}
	depletionRate := float64(totalSum) / float64(len(modelCalls))

	return &ContextWindowMetrics{
		PeakUtilization:   float64(peakPrompt) / float64(maxTokens),
		RemainingCapacity: maxTokens - peakPrompt,
		DepletionRate:     depletionRate,
		MaxContextTokens:  maxTokens,
		PeakPromptTokens:  peakPrompt,
		CompactionCount:   len(compactionEvents),
	}
}

// ContextLimitForModel returns the maximum context window size for model.
// Falls back to DefaultContextLimit for unknown models.
func ContextLimitForModel(model string) int {
	if limit, ok := ModelContextLimits[model]; ok {
		return limit
	}
	return DefaultContextLimit
}
