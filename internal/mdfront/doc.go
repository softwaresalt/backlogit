// Package mdfront is the single canonical body-preserving Markdown frontmatter
// codec for the repository. It splits a markdown document into an optional
// leading YAML frontmatter block and the verbatim body bytes that follow, and
// re-assembles the two with deterministic, sorted-key frontmatter while never
// touching a single body byte.
//
// # Ownership
//
// mdfront sits at the lowest layer and owns ONE concern: the byte-preserving
// RAW markdown/frontmatter codec. It deliberately does NOT know about backlog
// artifacts, file paths, or the filesystem. Two neighboring packages own
// adjacent concerns and must not be confused with this one:
//
//   - internal/models — artifact materialization and string serialization of
//     backlog work items (a higher-level, lossy-by-design contract that orders
//     and shapes known fields).
//   - internal/parser — higher-level file-to-artifact adapters that read files
//     from disk and produce typed artifacts.
//
// New low-level callers that need byte-exact frontmatter editing must depend on
// mdfront directly, NOT on internal/docline (which now re-exports this codec for
// backward compatibility) and NOT on a third re-inlined copy. There must be
// exactly one body-preserving codec in the tree, and it lives here.
//
// # Leaf package
//
// mdfront imports only the standard library plus gopkg.in/yaml.v3. It has no
// internal imports, so it can be consumed by both internal/docline and
// internal/core without reintroducing the import cycle that previously forced
// two divergent copies of this logic to exist.
//
// # Byte preservation
//
// Decode never normalizes the body: CRLF line endings, trailing whitespace, and
// horizontal rules (a body line that is exactly "---") are preserved
// byte-for-byte. Only the frontmatter block is re-marshaled (with sorted keys)
// on Encode. This guarantee underpins both the docline migration's
// body_bytes_changed == 0 invariant and the doctor archived_from repair, which
// must rewrite a single field without disturbing the surrounding document.
package mdfront
