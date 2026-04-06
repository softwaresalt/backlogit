package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T002 / ST011: Verify shipment sentinel errors are distinct and support errors.Is.
func TestShipmentSentinelErrors_ErrorsIs(t *testing.T) {
	tests := []struct {
		name     string
		sentinel error
		msg      string
	}{
		{"ErrShipmentNotFound", ErrShipmentNotFound, "backlogit: shipment not found"},
		{"ErrItemAlreadyAssigned", ErrItemAlreadyAssigned, "backlogit: item already assigned to a shipment"},
		{"ErrShipmentConflict", ErrShipmentConflict, "backlogit: shipment status conflict"},
		{"ErrCannotReturnItem", ErrCannotReturnItem, "backlogit: cannot return item from shipment"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			wrapped := fmt.Errorf("context: %w", tt.sentinel)

			// Act & Assert
			require.True(t, errors.Is(wrapped, tt.sentinel),
				"wrapped error must match %s via errors.Is", tt.name)
			assert.Equal(t, tt.msg, tt.sentinel.Error())
		})
	}
}

// T002 / ST011: Verify shipment sentinels are not aliases of each other.
func TestShipmentSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{
		ErrShipmentNotFound,
		ErrItemAlreadyAssigned,
		ErrShipmentConflict,
		ErrCannotReturnItem,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j {
				assert.False(t, errors.Is(a, b),
					"%v and %v must not match each other", a, b)
			}
		}
	}
}
