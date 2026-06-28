package docline

import (
	"github.com/softwaresalt/backlogit/internal/mdfront"
)

// Markdown is the decoded form of a markdown document. It is a TRUE alias for
// mdfront.Markdown: the single canonical body-preserving frontmatter codec now
// lives in internal/mdfront, and docline re-exports it so existing callers
// (cmd/gen-docs, the docline service) keep working unchanged.
//
// Because this is a real alias (not a defined type), the (*Markdown).Encode
// method is INHERITED from mdfront.Markdown — Go forbids declaring a method on a
// non-local alias target, so Encode is deliberately NOT re-declared here. Only
// the package-level Decode function is forwarded below.
type Markdown = mdfront.Markdown

// Decode splits raw markdown bytes into frontmatter and a preserved body by
// forwarding to mdfront.Decode. It is retained as a package-level function so
// the docline public codec surface (docline.Decode) is unchanged.
//
// See mdfront.Decode for the full contract: a leading "---" fence is recognized
// only with a matching first closing "---"; the body is preserved byte-for-byte;
// a fence-less document returns HasFrontmatter=false with a nil Frontmatter map
// and no error.
func Decode(raw []byte) (*Markdown, error) {
	return mdfront.Decode(raw)
}
