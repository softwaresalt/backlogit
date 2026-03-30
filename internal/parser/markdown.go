package parser

import "github.com/backlogit/backlogit/internal/models"

// ParseMarkdownFile reads a Markdown file and extracts the artifact and body.
//
// Worker: Implement file reading, frontmatter extraction, and artifact conversion.
func ParseMarkdownFile(path string) (*models.Artifact, string, error) {
	panic("not implemented: Worker: Implement Markdown file parsing")
}
