package parser

import (
	"fmt"
	"os"

	"github.com/backlogit/backlogit/internal/models"
)

// ParseMarkdownFile reads a Markdown file and extracts the artifact and body.
func ParseMarkdownFile(path string) (*models.Artifact, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read file %s: %w", path, err)
	}

	fm, body, err := models.ParseFrontmatter(string(data))
	if err != nil {
		return nil, "", fmt.Errorf("parse frontmatter: %w", err)
	}

	artifact, err := models.ArtifactFromFrontmatter(fm, body)
	if err != nil {
		return nil, "", fmt.Errorf("artifact from frontmatter: %w", err)
	}

	return artifact, body, nil
}
