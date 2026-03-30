package models

// ParseFrontmatter extracts YAML frontmatter from Markdown content.
// Returns the parsed key-value pairs, the remaining body text, and any error.
//
// Worker: Implement YAML frontmatter extraction between --- delimiters.
func ParseFrontmatter(content string) (map[string]any, string, error) {
	panic("not implemented: Worker: Implement frontmatter parsing")
}

// ArtifactFromFrontmatter converts raw frontmatter to a typed Artifact struct.
//
// Worker: Implement type-safe conversion from map to Artifact.
func ArtifactFromFrontmatter(fm map[string]any, body string) (*Artifact, error) {
	panic("not implemented: Worker: Implement frontmatter to artifact conversion")
}

// SerializeFrontmatter produces a Markdown string with YAML frontmatter and body.
//
// Worker: Implement serialization with sorted YAML keys.
func SerializeFrontmatter(fields map[string]any, body string) string {
	panic("not implemented: Worker: Implement frontmatter serialization")
}
