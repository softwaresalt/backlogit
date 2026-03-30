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
