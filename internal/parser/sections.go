package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Sentinel errors returned by the section primitives so callers can distinguish
// a genuinely absent section (safe to append) from malformed markers (corruption
// that must be surfaced as an error, never silently repaired).
var (
	// ErrSectionNotFound indicates the named section's BEGIN marker is absent.
	ErrSectionNotFound = errors.New("not found in document")
	// ErrSectionMalformed indicates a BEGIN marker without a matching END marker.
	ErrSectionMalformed = errors.New("missing END tag")
)

// beginRE matches <!-- BEGIN:{name} --> tags and captures the section name.
var beginRE = regexp.MustCompile(`<!-- BEGIN:(\S+?) -->`)

// ParseSections extracts named sections from markdown content between
// <!-- BEGIN:{name} --> and <!-- END:{name} --> tags.
// Leading and trailing newlines are trimmed from each section's content, but
// internal whitespace (indentation, blank lines) is preserved.
// Returns an error if a BEGIN tag has no matching END tag.
func ParseSections(content string) (map[string]string, error) {
	sections := make(map[string]string)

	matches := beginRE.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		// match[0]:match[1] = full tag, match[2]:match[3] = captured name.
		name := content[match[2]:match[3]]
		beginTag := "<!-- BEGIN:" + name + " -->"
		endTag := "<!-- END:" + name + " -->"

		afterBegin := match[0] + len(beginTag)

		endRelIdx := strings.Index(content[afterBegin:], endTag)
		if endRelIdx == -1 {
			return nil, fmt.Errorf("section %q: %w", name, ErrSectionMalformed)
		}

		// Trim only leading/trailing newlines; preserve internal whitespace.
		extracted := content[afterBegin : afterBegin+endRelIdx]
		sections[name] = strings.Trim(extracted, "\n")
	}

	return sections, nil
}

// WriteSections replaces multiple section contents while preserving the rest of the document.
// For each entry in updates the content between the matching BEGIN/END tags is replaced with
// the provided value. Returns an error if a named section is absent from content.
func WriteSections(content string, updates map[string]string) (string, error) {
	result := content

	for name, value := range updates {
		beginTag := "<!-- BEGIN:" + name + " -->"
		endTag := "<!-- END:" + name + " -->"

		beginIdx := strings.Index(result, beginTag)
		if beginIdx == -1 {
			return "", fmt.Errorf("section %q: %w", name, ErrSectionNotFound)
		}

		afterBegin := beginIdx + len(beginTag)

		endRelIdx := strings.Index(result[afterBegin:], endTag)
		if endRelIdx == -1 {
			return "", fmt.Errorf("section %q: %w", name, ErrSectionMalformed)
		}

		endIdx := afterBegin + endRelIdx

		prefix := result[:beginIdx]
		suffix := result[endIdx+len(endTag):]
		result = prefix + beginTag + "\n" + value + "\n" + endTag + suffix
	}

	return result, nil
}

// WriteSection replaces a single section's content in the document.
// It is a convenience wrapper around WriteSections.
func WriteSection(content string, name string, value string) (string, error) {
	return WriteSections(content, map[string]string{name: value})
}

// ValidateSectionName rejects section names that would produce malformed
// BEGIN/END markers or be unparseable by ParseSections. Section names must be
// non-empty, contain no whitespace (beginRE requires a contiguous \S+ name),
// no "--" sequence (which is invalid inside an HTML comment and includes the
// "-->" comment terminator), and no newlines. Both the CLI and MCP
// section-write paths call this so a corrupt name can never be persisted via
// either surface.
func ValidateSectionName(name string) error {
	if name == "" {
		return fmt.Errorf("section name must not be empty")
	}
	if strings.Contains(name, "--") {
		return fmt.Errorf("section name %q must not contain \"--\"; it is invalid inside an HTML comment marker", name)
	}
	if strings.ContainsAny(name, "\n\r") {
		return fmt.Errorf("section name %q must not contain newlines", name)
	}
	if strings.ContainsAny(name, " \t") {
		return fmt.Errorf("section name %q must not contain whitespace; the section parser requires contiguous non-whitespace names", name)
	}
	return nil
}
