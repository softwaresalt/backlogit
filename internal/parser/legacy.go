package parser

// LegacyItem represents a parsed item from a legacy backlog.md file.
type LegacyItem struct {
	Title       string
	Status      string
	ParentTitle string
	Depth       int
	Description string
}

// ParseLegacy parses a monolithic backlog.md using AST heuristics.
//
// Worker: Implement legacy backlog.md parser with heading and checklist conventions.
func ParseLegacy(content string) ([]LegacyItem, error) {
	panic("not implemented: Worker: Implement legacy backlog.md parser")
}
