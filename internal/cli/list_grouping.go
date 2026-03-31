package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// EnhancedListOptions extends the basic list command with grouping capabilities.
type EnhancedListOptions struct {
	GroupBy  string `json:"group_by"`
	Collapse bool   `json:"collapse"`
	Tree     bool   `json:"tree"`
}

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

// NewListEnhancedCmd creates enhanced `backlogit list` with grouping and tree view.
func NewListEnhancedCmd() *cobra.Command {
	opts := &EnhancedListOptions{}

	cmd := &cobra.Command{
		Use:   "list-enhanced",
		Short: "List artifacts with grouping and tree view",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.GroupBy, "group-by", "", "group output by field (type, status, priority)")
	cmd.Flags().BoolVar(&opts.Collapse, "collapse", false, "collapse groups by default")
	cmd.Flags().BoolVar(&opts.Tree, "tree", false, "render as a parent-child tree")
	return cmd
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
