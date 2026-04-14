package errors_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerr "github.com/softwaresalt/backlogit/internal/errors"
)

func TestSentinelErrors(t *testing.T) {
	sentinels := []error{
		blerr.ErrConfig,
		blerr.ErrValidation,
		blerr.ErrQuery,
		blerr.ErrRehydration,
		blerr.ErrMigration,
		blerr.ErrMCP,
	}
	for _, sentinel := range sentinels {
		assert.Error(t, sentinel)
		assert.Contains(t, sentinel.Error(), "backlogit:")
	}
}

func TestConfigError_Is(t *testing.T) {
	// Arrange
	configErr := blerr.NewConfigError("field", "bad value", nil)

	// Act & Assert
	assert.True(t, errors.Is(configErr, blerr.ErrConfig))
	assert.False(t, errors.Is(configErr, blerr.ErrValidation))
}

func TestConfigError_As(t *testing.T) {
	// Arrange
	configErr := blerr.NewConfigError("database", "connection failed", fmt.Errorf("dial timeout"))
	wrapped := fmt.Errorf("loading config: %w", configErr)

	// Act
	var target *blerr.ConfigError
	found := errors.As(wrapped, &target)

	// Assert
	require.True(t, found)
	assert.Equal(t, "database", target.Field)
	assert.Equal(t, "connection failed", target.Message)
}

func TestValidationError_Is(t *testing.T) {
	// Arrange
	valErr := blerr.NewValidationError("status", "invalid", "oneof", nil)

	// Act & Assert
	assert.True(t, errors.Is(valErr, blerr.ErrValidation))
	assert.False(t, errors.Is(valErr, blerr.ErrConfig))
}

func TestQueryError_Is(t *testing.T) {
	// Arrange
	queryErr := blerr.NewQueryError("DROP TABLE", nil)

	// Act & Assert
	assert.True(t, errors.Is(queryErr, blerr.ErrQuery))
	assert.False(t, errors.Is(queryErr, blerr.ErrConfig))
}

func TestConfigError_Unwrap(t *testing.T) {
	// Arrange
	inner := fmt.Errorf("inner error")
	configErr := blerr.NewConfigError("field", "msg", inner)

	// Act
	unwrapped := configErr.Unwrap()

	// Assert
	assert.Equal(t, inner, unwrapped)
}
