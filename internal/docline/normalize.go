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
		// Two-%w wrap (Go >= 1.20; go.mod declares go 1.24.0): the discriminator
		// ErrFrontmatterDecode is wrapped alongside the original decode cause so
		// classifyDecodeFailure can select on errors.Is(err, ErrFrontmatterDecode)
		// while the underlying YAML error is still recoverable for diagnostics.
		// This mirrors the two-%w pattern already used in decodeDoc.
		return nil, fmt.Errorf("docline.Normalize: decode %s: %w: %w", relPath, ErrFrontmatterDecode, err)
	}

	fm := md.Frontmatter
	if fm == nil {
		fm = map[string]any{}
	}

	// Build the docline namespace: start from any existing docline map, then
	// fold every non-contract top-level key into it (move, never drop).
	// Collisions and a non-map existing docline value are preserved without
	// dropping data.
	docline := map[string]any{}
	switch existing := fm["docline"].(type) {
	case map[string]any:
		for k, v := range existing {
			docline[k] = v
		}
	case nil:
		// No existing docline namespace.
	default:
		// A non-map docline value (malformed/legacy input) is preserved, never
		// silently dropped.
		foldUnderDocline(docline, "docline", existing)
	}
	for k, v := range fm {
		if k == "docline" || isContractField(k) {
			continue
		}
		foldUnderDocline(docline, k, v)
	}

	// Read the contract surface, then override the repo-derived fields.
	b := FromMap(fm)
	b.DocType = string(Classify(relPath))
	// Source defaults to the repo-relative POSIX path, but a pre-existing
	// full-URI source (a known online source) is preserved verbatim and never
	// rewritten — Q2 sign-off, task 065.002-T.
	if !hasURIScheme(b.Source) {
		b.Source = DeriveSource(relPath)
	}
	// Seed ingested_at once: preserve any existing non-empty value. Seeding from
	// a zero Now would write a nonsense 0001-01-01T00:00:00Z timestamp, so fail
	// fast and require callers to supply a real clock when a seed is needed.
	if b.IngestedAt == "" {
		if opts.Now.IsZero() {
			return nil, fmt.Errorf("docline.Normalize: %s: %w", relPath, ErrMissingSeedTime)
		}
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

// foldUnderDocline stores val under key in the docline namespace without ever
// overwriting (dropping) an existing value. On collision the incoming value is
// preserved under a deterministic suffixed key so the fold remains lossless and
// idempotent across repeated normalization runs.
func foldUnderDocline(ns map[string]any, key string, val any) {
	if _, exists := ns[key]; !exists {
		ns[key] = val
		return
	}
	for i := 1; ; i++ {
		alt := fmt.Sprintf("%s_top%d", key, i)
		if _, exists := ns[alt]; !exists {
			ns[alt] = val
			return
		}
	}
}
