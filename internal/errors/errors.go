package errors

import "errors"

// Sentinel errors for the backlogit error hierarchy.
var (
	ErrConfig      = errors.New("backlogit: configuration error")
	ErrValidation  = errors.New("backlogit: validation error")
	ErrQuery       = errors.New("backlogit: query error")
	ErrRehydration = errors.New("backlogit: rehydration error")
	ErrMigration   = errors.New("backlogit: migration error")
	ErrMCP         = errors.New("backlogit: mcp error")
	ErrNotFound    = errors.New("backlogit: not found")

	// Shipment sentinel errors (F015 / T002 / ST011).
	ErrShipmentNotFound    = errors.New("backlogit: shipment not found")
	ErrItemAlreadyAssigned = errors.New("backlogit: item already assigned to a shipment")
	ErrShipmentConflict    = errors.New("backlogit: shipment status conflict")
	ErrCannotReturnItem    = errors.New("backlogit: cannot return item from shipment")

	// Cascade sentinel errors (F018 / T004).
	ErrChildrenNotTerminal = errors.New("backlogit: parent cannot transition while non-terminal children exist")

	// Link sentinel errors (F018 / T001, 026-F).
	ErrInvalidLinkType = errors.New("backlogit: invalid link type")
	ErrLinkNotFound    = errors.New("backlogit: link not found")

	// Telemetry sentinel errors (021-F).
	ErrTelemetrySourceMissing = errors.New("backlogit: telemetry source missing")
	ErrTelemetryParseFailed   = errors.New("backlogit: telemetry parse failed")

	// Checkpoint sentinel errors (045-F).
	ErrCheckpointNotFound = errors.New("backlogit: checkpoint not found")
	ErrCheckpointInvalid  = errors.New("backlogit: checkpoint validation failed")
	ErrCheckpointCorrupt  = errors.New("backlogit: checkpoint file corrupt or unparseable")

	// Hook event sentinel errors (027-F).
	ErrHookEvent = errors.New("backlogit: hook event error")

	// Lifecycle hook sentinel errors (033-F).
	ErrHook                    = errors.New("backlogit: hook error")
	ErrInvalidStatusTransition = errors.New("backlogit: invalid status transition")
	ErrWebhookDispatch         = errors.New("backlogit: webhook dispatch error")

	// Root-ID conflict integrity sentinel errors (066-F).
	// ErrIDCollision indicates a freshly resolved artifact ID already exists as a
	// canonical file on the filesystem (queue, archive, or a routed directory),
	// so creation must fail loud rather than silently reuse the ID.
	ErrIDCollision = errors.New("backlogit: artifact ID already exists on the canonical filesystem")
	// ErrArchiveDestinationOccupied indicates the archive destination is already
	// occupied by a different item that shares the filename, so archiving must
	// refuse rather than overwrite the existing archived item.
	ErrArchiveDestinationOccupied = errors.New("backlogit: archive destination already occupied by a different item")
)

// ConfigError wraps a configuration failure with field context.
type ConfigError struct {
	Field   string
	Message string
	Err     error
}

// Error returns the formatted error message.
func (e *ConfigError) Error() string {
	return "config: " + e.Field + ": " + e.Message
}

// Unwrap returns the underlying error.
func (e *ConfigError) Unwrap() error {
	return e.Err
}

// Is reports whether the target matches ErrConfig.
func (e *ConfigError) Is(target error) bool {
	return target == ErrConfig
}

// NewConfigError creates a new ConfigError.
func NewConfigError(field, message string, err error) *ConfigError {
	return &ConfigError{Field: field, Message: message, Err: err}
}

// ValidationError wraps a validation failure with field context.
type ValidationError struct {
	Field      string
	Value      any
	Constraint string
	Err        error
}

// Error returns the formatted error message.
func (e *ValidationError) Error() string {
	return "validation: " + e.Field + ": " + e.Constraint
}

// Unwrap returns the underlying error.
func (e *ValidationError) Unwrap() error {
	return e.Err
}

// Is reports whether the target matches ErrValidation.
func (e *ValidationError) Is(target error) bool {
	return target == ErrValidation
}

// NewValidationError creates a new ValidationError.
func NewValidationError(field string, value any, constraint string, err error) *ValidationError {
	return &ValidationError{Field: field, Value: value, Constraint: constraint, Err: err}
}

// QueryError wraps a query failure with SQL context.
type QueryError struct {
	SQL string
	Err error
}

// Error returns the formatted error message.
func (e *QueryError) Error() string {
	return "query: " + e.SQL
}

// Unwrap returns the underlying error.
func (e *QueryError) Unwrap() error {
	return e.Err
}

// Is reports whether the target matches ErrQuery.
func (e *QueryError) Is(target error) bool {
	return target == ErrQuery
}

// NewQueryError creates a new QueryError.
func NewQueryError(sql string, err error) *QueryError {
	return &QueryError{SQL: sql, Err: err}
}
