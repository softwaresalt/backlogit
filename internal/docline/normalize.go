package docline

import (
	"fmt"
	"time"
)

// NormalizeOptions controls the non-deterministic inputs to Normalize so the
// transform stays testable and idempotent.
type NormalizeOptions struct {
	// Now supplies the timestamp used to SEED ingested_at when it is absent.
	// Per the Q1 recommendation (seed-once at migration), an existing non-empty
	// ingested_at is preserved verbatim and Now is ignored.
	Now time.Time
}

// Normalize rewrites a markdown document's frontmatter to the canonical docline
// authoring profile while preserving the body bytes exactly. It is idempotent:
// Normalize(Normalize(x)) == Normalize(x).
func Normalize(relPath string, raw []byte, opts NormalizeOptions) ([]byte, error) {
	md, err := Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("docline.Normalize: decode %s: %w", relPath, err)
	}

	fm := md.Frontmatter
	if fm == nil {
		fm = map[string]any{}
	}

	// Build the docline namespace: start from any existing docline map, then
	// fold every non-contract top-level key into it (move, never drop).
	docline := map[string]any{}
	if existing, ok := fm["docline"].(map[string]any); ok {
		for k, v := range existing {
			docline[k] = v
		}
	}
	for k, v := range fm {
		if k == "docline" || isContractField(k) {
			continue
		}
		docline[k] = v
	}

	// Read the contract surface, then override the repo-derived fields.
	b := FromMap(fm)
	b.DocType = string(Classify(relPath))
	b.Source = DeriveSource(relPath)
	// Seed ingested_at once: preserve any existing non-empty value.
	if b.IngestedAt == "" {
		b.IngestedAt = opts.Now.UTC().Format(time.RFC3339)
	}
	b.Docline = docline

	out := &Markdown{
		HasFrontmatter: true,
		Frontmatter:    b.ToMap(),
		Body:           md.Body,
	}
	encoded, err := out.Encode()
	if err != nil {
		return nil, fmt.Errorf("docline.Normalize: encode %s: %w", relPath, err)
	}
	return encoded, nil
}
