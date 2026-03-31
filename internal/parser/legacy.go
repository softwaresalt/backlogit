package parser

import (
	"regexp"
	"strings"
)

var checklistRe = regexp.MustCompile(`^\s*-\s+\[([xX ])\]\s+(.+)$`)

// LegacyItem represents a parsed item from a legacy backlog.md file.
type LegacyItem struct {
	Title       string
	Status      string
	ParentTitle string
	Depth       int
	Description string
}

// ParseLegacy parses a monolithic backlog.md using heading and checklist heuristics.
func ParseLegacy(content string) ([]LegacyItem, error) {
	var items []LegacyItem
	var currentSection string

	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "#") {
			currentSection = strings.TrimSpace(strings.TrimLeft(line, "#"))
			continue
		}
		if m := checklistRe.FindStringSubmatch(line); m != nil {
			checked := strings.ToLower(m[1]) == "x"
			title := strings.TrimSpace(m[2])
			status := "queued"
			if checked {
				status = "done"
			} else {
				switch strings.ToLower(currentSection) {
				case "in progress", "in_progress", "active":
					status = "active"
				case "blocked":
					status = "blocked"
				}
			}
			items = append(items, LegacyItem{
				Title:       title,
				Status:      status,
				ParentTitle: currentSection,
			})
		}
	}

	return items, nil
}
