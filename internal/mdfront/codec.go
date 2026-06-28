package mdfront

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
)

// Markdown is the decoded form of a markdown document: an optional YAML
// frontmatter map plus the verbatim body bytes that follow it.
//
// The codec operates on raw bytes and is intentionally decoupled from the
// backlog-artifact write path (internal/parser, internal/models). It never
// normalizes the body: CRLF, trailing whitespace, and horizontal rules in the
// body are preserved byte-for-byte. Only the frontmatter block is rewritten
// deterministically (sorted keys) on Encode.
type Markdown struct {
	// HasFrontmatter is true when the source had a leading --- fenced block.
	HasFrontmatter bool
	// Frontmatter is the parsed YAML map. Nested maps (the docline namespace
	// and any sub-trees) are preserved as decoded.
	Frontmatter map[string]any
	// Body is the exact body bytes that follow the closing frontmatter fence
	// (or the whole document when there is no frontmatter).
	Body []byte
}

// Decode splits raw markdown bytes into frontmatter and a preserved body.
//
// A leading frontmatter block is recognized only when the document begins with
// a line that is exactly "---" (LF or CRLF terminated) and a later line that is
// exactly "---" closes it. The closing fence is the FIRST such line, and the
// match requires no leading whitespace, so a "---" inside an indented YAML
// block scalar is never mistaken for the fence. When no opening or closing
// fence is found the whole input is returned as the body with
// HasFrontmatter=false and a nil Frontmatter map.
func Decode(raw []byte) (*Markdown, error) {
	openLen := openingFenceLen(raw)
	if openLen == 0 {
		return &Markdown{HasFrontmatter: false, Body: raw}, nil
	}

	rest := raw[openLen:]
	yamlBlock, body, ok := splitAtClosingFence(rest)
	if !ok {
		// No closing fence: this is not a frontmatter block.
		return &Markdown{HasFrontmatter: false, Body: raw}, nil
	}

	fm := map[string]any{}
	// Normalize CRLF in the FRONTMATTER block only; the body is never touched.
	normalized := bytes.ReplaceAll(yamlBlock, []byte("\r\n"), []byte("\n"))
	if len(bytes.TrimSpace(normalized)) > 0 {
		if err := yaml.Unmarshal(normalized, &fm); err != nil {
			return nil, fmt.Errorf("mdfront.Decode: parse frontmatter: %w", err)
		}
	}
	if fm == nil {
		fm = map[string]any{}
	}

	return &Markdown{HasFrontmatter: true, Frontmatter: fm, Body: body}, nil
}

// Encode assembles the document with deterministic sorted-key frontmatter and
// the preserved body bytes. When the frontmatter map is empty the body is
// returned unchanged (no fence is emitted).
func (m *Markdown) Encode() ([]byte, error) {
	if len(m.Frontmatter) == 0 {
		return m.Body, nil
	}

	data, err := yaml.Marshal(m.Frontmatter)
	if err != nil {
		return nil, fmt.Errorf("mdfront.Encode: marshal frontmatter: %w", err)
	}

	var buf bytes.Buffer
	buf.Grow(len(data) + len(m.Body) + 8)
	buf.WriteString("---\n")
	buf.Write(data) // yaml.Marshal output is LF-terminated
	buf.WriteString("---\n")
	buf.Write(m.Body)
	return buf.Bytes(), nil
}

// openingFenceLen returns the byte length of the opening fence line (including
// its terminator) when raw begins with a line that is exactly "---", or 0.
func openingFenceLen(raw []byte) int {
	switch {
	case bytes.HasPrefix(raw, []byte("---\n")):
		return 4
	case bytes.HasPrefix(raw, []byte("---\r\n")):
		return 5
	default:
		return 0
	}
}

// splitAtClosingFence scans rest line-by-line and splits at the first line whose
// content is exactly "---" (a trailing CR is tolerated for CRLF files; leading
// whitespace is not, so indented block-scalar fences are skipped). It returns
// the frontmatter YAML bytes, the body bytes after the fence line, and whether
// a closing fence was found.
func splitAtClosingFence(rest []byte) (yamlBlock, body []byte, ok bool) {
	for i := 0; i < len(rest); {
		nl := bytes.IndexByte(rest[i:], '\n')
		var lineEnd, nextStart int
		if nl == -1 {
			lineEnd, nextStart = len(rest), len(rest)
		} else {
			lineEnd, nextStart = i+nl, i+nl+1
		}

		line := rest[i:lineEnd]
		if n := len(line); n > 0 && line[n-1] == '\r' {
			line = line[:n-1]
		}

		if string(line) == "---" {
			return rest[:i], rest[nextStart:], true
		}
		i = nextStart
	}
	return nil, nil, false
}
