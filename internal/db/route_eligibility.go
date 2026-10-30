package db

import (
	"strings"
	"unicode"

	"github.com/softwaresalt/backlogit/internal/models"
)

// IsTaskRouteEligible reports whether item is a task carrying complete, valid
// size and complexity provenance: size, size_source, size_ruleset_version,
// complexity, complexity_source and complexity_ruleset_version must all be
// present and valid. Values that are missing, blank, non-string, outside their
// enum, or (for ruleset versions) containing control characters make the task
// ineligible.
func IsTaskRouteEligible(item *models.Artifact) bool {
	if item == nil || item.ArtifactType != "task" {
		return false
	}
	cf := item.CustomFields
	return routeEnumField(cf, "size", "XS", "S", "M", "L", "XL") &&
		routeEnumField(cf, "size_source", "human", "agent", "derived") &&
		routeRulesetField(cf, "size_ruleset_version") &&
		routeEnumField(cf, "complexity", "trivial", "low", "medium", "high") &&
		routeEnumField(cf, "complexity_source", "human", "agent", "derived") &&
		routeRulesetField(cf, "complexity_ruleset_version")
}

func routeEnumField(cf map[string]any, key string, allowed ...string) bool {
	v, ok := cf[key].(string)
	if !ok {
		return false
	}
	for _, a := range allowed {
		if v == a {
			return true
		}
	}
	return false
}

func routeRulesetField(cf map[string]any, key string) bool {
	v, ok := cf[key].(string)
	if !ok || strings.TrimSpace(v) == "" {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
