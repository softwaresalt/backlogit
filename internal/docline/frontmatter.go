package docline

import (
	"fmt"
	"time"
)

// Default values for optional contract fields (mirrors the schema defaults in
// schemas/docline/base-frontmatter-v1.schema.json).
const (
	DefaultChunkStrategy = "h1-h2-h3"
	DefaultSchemaVersion = "1.0"
)

// BaseFrontmatter is the Go representation of the docline base-frontmatter v1
// contract surface. It models only the top-level contract fields; non-contract
// keys are folded under Docline by the normalizer (move, never drop).
type BaseFrontmatter struct {
	Title         string
	Source        string
	IngestedAt    string
	DocType       string
	Description   string
	ContentSHA256 string
	SourcePath    string
	ChunkStrategy string
	SchemaVersion string
	Docline       map[string]any
}

// FromMap builds a BaseFrontmatter from a decoded frontmatter map, reading only
// the top-level contract fields and applying defaults for chunk_strategy and
// schema_version. Non-contract keys are NOT read here; the normalizer folds
// them under the docline namespace before calling FromMap (move, never drop).
func FromMap(fm map[string]any) BaseFrontmatter {
	b := BaseFrontmatter{
		Title:         strFromMap(fm, "title"),
		Source:        strFromMap(fm, "source"),
		IngestedAt:    strFromMap(fm, "ingested_at"),
		DocType:       strFromMap(fm, "doc_type"),
		Description:   strFromMap(fm, "description"),
		ContentSHA256: strFromMap(fm, "content_sha256"),
		SourcePath:    strFromMap(fm, "source_path"),
		ChunkStrategy: strFromMap(fm, "chunk_strategy"),
		SchemaVersion: strFromMap(fm, "schema_version"),
	}
	if d, ok := fm["docline"].(map[string]any); ok && len(d) > 0 {
		b.Docline = d
	}
	if b.ChunkStrategy == "" {
		b.ChunkStrategy = DefaultChunkStrategy
	}
	if b.SchemaVersion == "" {
		b.SchemaVersion = DefaultSchemaVersion
	}
	return b
}

// ToMap renders the contract fields back to a frontmatter map for the codec.
// Required fields, description, chunk_strategy, and schema_version are always
// emitted. The pipeline-owned fields content_sha256 and source_path are emitted
// only when already populated (the repo never fabricates them), and the docline
// namespace is emitted only when non-empty.
func (b BaseFrontmatter) ToMap() map[string]any {
	out := map[string]any{
		"title":          b.Title,
		"source":         b.Source,
		"ingested_at":    b.IngestedAt,
		"doc_type":       b.DocType,
		"description":    b.Description,
		"chunk_strategy": orDefault(b.ChunkStrategy, DefaultChunkStrategy),
		"schema_version": orDefault(b.SchemaVersion, DefaultSchemaVersion),
	}
	if b.ContentSHA256 != "" {
		out["content_sha256"] = b.ContentSHA256
	}
	if b.SourcePath != "" {
		out["source_path"] = b.SourcePath
	}
	if len(b.Docline) > 0 {
		out["docline"] = b.Docline
	}
	return out
}

// strFromMap coerces a frontmatter value to a string. YAML timestamps decode to
// time.Time under an any target, so those are rendered back to RFC3339.
func strFromMap(fm map[string]any, key string) string {
	switch v := fm[key].(type) {
	case nil:
		return ""
	case string:
		return v
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// orDefault returns v when non-empty, otherwise def.
func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
