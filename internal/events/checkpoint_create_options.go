package events

// CreateCheckpointOption configures CreateCheckpoint. With no options the
// create is strict by default: only schema_version 1 dumps are accepted.
type CreateCheckpointOption func(*createCheckpointConfig)

// createCheckpointConfig holds the resolved create-time options.
type createCheckpointConfig struct {
	allowLegacyImport bool
}

// WithAllowLegacyImport opts in to the migration-only legacy import: an
// in-window schema-less (absent or 0 schema_version) dump is upgraded to a
// canonical CheckpointV1 instead of being rejected. It never applies to
// unsupported, future, wrong-typed, or ambiguous schema versions.
func WithAllowLegacyImport() CreateCheckpointOption {
	return func(c *createCheckpointConfig) { c.allowLegacyImport = true }
}
