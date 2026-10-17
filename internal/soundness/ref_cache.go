package soundness

// refKey identifies one reference resolution within a run.
type refKey struct{ root, ref string }

// newCheckContext builds a fresh per-run context with an empty reference cache.
func newCheckContext(plan Plan, cfg Config) *checkContext {
	return &checkContext{
		plan:        plan,
		cfg:         cfg,
		byID:        map[string]*Unit{},
		refCache:    map[refKey]error{},
		refResolver: resolveReference,
	}
}

// resolveRef resolves ref against the configured root, memoizing both
// successes and failures for the lifetime of this context (one engine run).
func (c *checkContext) resolveRef(ref string) error {
	key := refKey{c.cfg.Root, ref}
	if err, ok := c.refCache[key]; ok {
		return err
	}
	if c.refCache == nil {
		c.refCache = map[refKey]error{}
	}
	resolver := c.refResolver
	if resolver == nil {
		resolver = resolveReference
	}
	c.refCalls++
	err := resolver(c.cfg.Root, ref)
	c.refCache[key] = err
	return err
}
