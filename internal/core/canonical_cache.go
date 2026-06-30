package core

import "fmt"

// scanCanonicalArtifactsFn is the package-level seam through which both the
// per-call CreateArtifact uniqueness guard and NewCanonicalCache reach the
// canonical scanner. Production code always binds it to scanCanonicalArtifacts;
// tests swap it to count how many full filesystem scans a flow performs.
var scanCanonicalArtifactsFn = scanCanonicalArtifacts

// CanonicalCache memoizes the canonical-uniqueness scan for a single bulk-create
// batch (070.001-T). scanCanonicalArtifacts walks and parses every queue/archive
// .md file, so calling it once per CreateArtifact turns a bulk import or priority
// harvest into an O(files * creates) -> O(N^2) operation on a large backlog.
//
// A bulk caller builds one cache before its loop with NewCanonicalCache, passes
// it to every create via WithCanonicalCache, and the per-create filesystem scan
// is skipped. Each successful create records its freshly minted ID back into the
// cache so later creates in the same batch still detect ID collisions without a
// re-scan. Single interactive creates pass no cache and keep scanning per call.
//
// A CanonicalCache is scoped to one sequential batch and is NOT safe for
// concurrent use.
type CanonicalCache struct {
	refs map[string][]artifactRef
}

// NewCanonicalCache performs the one-time canonical scan for a batch and returns
// a cache seeded with the current on-disk artifact set.
func NewCanonicalCache(ws *Workspace) (*CanonicalCache, error) {
	refs, err := scanCanonicalArtifactsFn(ws)
	if err != nil {
		return nil, fmt.Errorf("build canonical cache: %w", err)
	}
	if refs == nil {
		refs = make(map[string][]artifactRef)
	}
	return &CanonicalCache{refs: refs}, nil
}

// lookup returns the recorded refs for an ID (empty when the ID is unseen).
func (c *CanonicalCache) lookup(id string) []artifactRef {
	if c == nil {
		return nil
	}
	return c.refs[id]
}

// record registers a freshly created ID/path so later creates in the same batch
// observe it as taken without re-scanning the filesystem.
func (c *CanonicalCache) record(id, path string) {
	if c == nil || id == "" {
		return
	}
	c.refs[id] = append(c.refs[id], artifactRef{path: path, id: id})
}

// WithCanonicalCache shares a pre-built canonical scan across a bulk-create batch
// so the uniqueness guard scans the filesystem once for the batch instead of once
// per create. Omit it for single interactive creates.
func WithCanonicalCache(c *CanonicalCache) Option {
	return func(o *createOptions) { o.canonicalCache = c }
}
