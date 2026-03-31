package cli

import (
	"fmt"
	"strings"
)

// ListItem represents a single item in the list output.
type ListItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	ParentID string `json:"parent_id"`
	Priority string `json:"priority"`
	Depth    int    `json:"depth"`
}

// FormatTreeView renders artifacts as an indented tree based on parent-child relationships.
func FormatTreeView(items []ListItem) string {
	var sb strings.Builder
	for _, item := range items {
		indent := strings.Repeat("  ", item.Depth)
		fmt.Fprintf(&sb, "%s%s  %s  [%s]\n", indent, item.ID, item.Title, item.Status)
	}
	return sb.String()
}

// FormatGroupedView renders artifacts grouped by a field with group headers.
func FormatGroupedView(items []ListItem, groupBy string) string {
	groups := make(map[string][]ListItem)
	order := []string{}
	for _, item := range items {
		var key string
		switch groupBy {
		case "status":
			key = item.Status
		case "priority":
			key = item.Priority
		default:
			key = item.Type
		}
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], item)
	}

	var sb strings.Builder
	for _, key := range order {
		grp := groups[key]
		fmt.Fprintf(&sb, "── %s (%d items) ──\n", key, len(grp))
		for _, item := range grp {
			fmt.Fprintf(&sb, "  %s  %s  [%s]\n", item.ID, item.Title, item.Status)
		}
	}
	return sb.String()
}
