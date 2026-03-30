package parser

// ParseSections extracts named sections from markdown content between
// <!-- BEGIN:{name} --> and <!-- END:{name} --> tags.
func ParseSections(content string) (map[string]string, error) {
	panic("not implemented: Worker: Scan content for <!-- BEGIN:{name} --> and <!-- END:{name} --> delimiters. Extract the text between each pair into a map keyed by section name. Return error for missing END tags. Handle edge cases: empty sections, leading/trailing whitespace preservation, nested HTML comments.")
}

// WriteSections replaces multiple section contents while preserving the rest of the document.
func WriteSections(content string, updates map[string]string) (string, error) {
	panic("not implemented: Worker: For each section name in updates, locate the BEGIN/END delimiters in content and replace the content between them with the new value. Preserve everything outside section delimiters. Return error if a section name in updates does not exist in content.")
}

// WriteSection replaces a single section's content in the document.
func WriteSection(content string, name string, value string) (string, error) {
	panic("not implemented: Worker: Delegate to WriteSections with a single-entry map. This is a convenience wrapper.")
}
