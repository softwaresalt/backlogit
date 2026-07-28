package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteNotApplied_MatchesPredicateOnly(t *testing.T) {
	t.Parallel()
	cause := errors.New("rename failed before commit")
	err := fmt.Errorf("write temp: %w", fmt.Errorf("%w: %w", ErrWriteNotApplied, cause))

	assert.True(t, errors.Is(err, ErrWriteNotApplied), "must match the not-applied sentinel")
	assert.True(t, IsWriteNotApplied(err), "IsWriteNotApplied predicate must match")
	assert.False(t, errors.Is(err, ErrWriteIndeterminate), "must NOT match the indeterminate sentinel")
	assert.False(t, IsWriteIndeterminate(err), "IsWriteIndeterminate predicate must NOT match")
}

func TestWriteIndeterminate_MatchesPredicateOnly(t *testing.T) {
	t.Parallel()
	cause := errors.New("parent dir fsync failed after rename")
	err := fmt.Errorf("dir fsync: %w", fmt.Errorf("%w: %w", ErrWriteIndeterminate, cause))

	assert.True(t, errors.Is(err, ErrWriteIndeterminate), "must match the indeterminate sentinel")
	assert.True(t, IsWriteIndeterminate(err), "IsWriteIndeterminate predicate must match")
	assert.False(t, errors.Is(err, ErrWriteNotApplied), "must NOT match the not-applied sentinel")
	assert.False(t, IsWriteNotApplied(err), "IsWriteNotApplied predicate must NOT match")
}

func TestDurabilityErrors_PreserveCauseUnwrap(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		sentinel error
	}{
		{"not-applied", ErrWriteNotApplied},
		{"indeterminate", ErrWriteIndeterminate},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cause := errors.New("underlying syscall failure")
			err := fmt.Errorf("%w: %w", tc.sentinel, cause)
			require.True(t, errors.Is(err, tc.sentinel), "sentinel must match")
			assert.True(t, errors.Is(err, cause), "%w must preserve unwrap to the underlying cause")
		})
	}
}

func TestDurabilityPredicates_NilSafe(t *testing.T) {
	t.Parallel()
	assert.False(t, IsWriteNotApplied(nil))
	assert.False(t, IsWriteIndeterminate(nil))
}
