package core_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func TestMutationEnvelope_AllStepsSucceed(t *testing.T) {
	ctx := context.Background()
	applied := make([]string, 0, 2)

	err := core.MutationEnvelope(ctx, []core.MutationStep{
		{
			Name: "step-1",
			Apply: func(context.Context) error {
				applied = append(applied, "step-1")
				return nil
			},
		},
		{
			Name: "step-2",
			Apply: func(context.Context) error {
				applied = append(applied, "step-2")
				return nil
			},
		},
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"step-1", "step-2"}, applied)
}

func TestMutationEnvelope_FirstStepNotAppliedFailure(t *testing.T) {
	ctx := context.Background()

	err := core.MutationEnvelope(ctx, []core.MutationStep{
		{
			Name: "step-1",
			Apply: func(context.Context) error {
				return errors.Join(blerrors.ErrWriteNotApplied, errors.New("write failed"))
			},
			Compensate: func(context.Context) error {
				t.Fatal("compensate should not be called when nothing applied")
				return nil
			},
		},
	})

	require.Error(t, err)
	var partialErr *blerrors.MutationPartialError
	require.ErrorAs(t, err, &partialErr)
	assert.Empty(t, partialErr.Completed)
	assert.Equal(t, "step-1", partialErr.FailedStep)
	assert.Equal(t, "compensated", partialErr.CompensationState)
	assert.Equal(t, "not-applied", partialErr.Class)
	assert.ErrorIs(t, partialErr, blerrors.ErrWriteNotApplied)
}

func TestMutationEnvelope_SecondStepNotAppliedCompensatesFirst(t *testing.T) {
	ctx := context.Background()
	compensated := false

	err := core.MutationEnvelope(ctx, []core.MutationStep{
		{
			Name:  "step-1",
			Apply: func(context.Context) error { return nil },
			Compensate: func(context.Context) error {
				compensated = true
				return nil
			},
		},
		{
			Name: "step-2",
			Apply: func(context.Context) error {
				return errors.Join(blerrors.ErrWriteNotApplied, errors.New("write failed"))
			},
		},
	})

	require.Error(t, err)
	var partialErr *blerrors.MutationPartialError
	require.ErrorAs(t, err, &partialErr)
	assert.True(t, compensated)
	assert.Equal(t, []string{"step-1"}, partialErr.Completed)
	assert.Equal(t, "step-2", partialErr.FailedStep)
	assert.Equal(t, "compensated", partialErr.CompensationState)
	assert.Equal(t, "not-applied", partialErr.Class)
}

func TestMutationEnvelope_IndeterminateFirstStepDoesNotCompensate(t *testing.T) {
	ctx := context.Background()
	present := false
	compensated := false

	err := core.MutationEnvelope(ctx, []core.MutationStep{
		{
			Name: "step-1",
			Apply: func(context.Context) error {
				present = true
				return errors.Join(blerrors.ErrWriteIndeterminate, errors.New("fsync failed"))
			},
			Compensate: func(context.Context) error {
				compensated = true
				present = false
				return nil
			},
		},
	})

	require.Error(t, err)
	var partialErr *blerrors.MutationPartialError
	require.ErrorAs(t, err, &partialErr)
	assert.True(t, present, "indeterminate write should remain present")
	assert.False(t, compensated, "indeterminate path must not compensate")
	assert.Empty(t, partialErr.Completed)
	assert.Equal(t, "step-1", partialErr.FailedStep)
	assert.Equal(t, "not-compensated", partialErr.CompensationState)
	assert.Equal(t, "indeterminate", partialErr.Class)
	assert.ErrorIs(t, partialErr, blerrors.ErrWriteIndeterminate)
}

func TestMutationEnvelope_IndeterminateSecondStepContinuesRemainingSteps(t *testing.T) {
	ctx := context.Background()
	var ran []string
	compensated := false

	err := core.MutationEnvelope(ctx, []core.MutationStep{
		{
			Name: "step-1",
			Apply: func(context.Context) error {
				ran = append(ran, "step-1")
				return nil
			},
			Compensate: func(context.Context) error {
				compensated = true
				return nil
			},
		},
		{
			Name: "step-2",
			Apply: func(context.Context) error {
				ran = append(ran, "step-2")
				return errors.Join(blerrors.ErrWriteIndeterminate, errors.New("fsync failed"))
			},
		},
		{
			Name: "step-3",
			Apply: func(context.Context) error {
				ran = append(ran, "step-3")
				return nil
			},
		},
	})

	require.Error(t, err)
	var partialErr *blerrors.MutationPartialError
	require.ErrorAs(t, err, &partialErr)
	assert.False(t, compensated)
	assert.Equal(t, []string{"step-1", "step-2", "step-3"}, ran)
	assert.Equal(t, []string{"step-1"}, partialErr.Completed)
	assert.Equal(t, "step-2", partialErr.FailedStep)
	assert.Equal(t, "not-compensated", partialErr.CompensationState)
	assert.Equal(t, "indeterminate", partialErr.Class)
}

func TestMutationEnvelope_DoubleFault(t *testing.T) {
	ctx := context.Background()

	err := core.MutationEnvelope(ctx, []core.MutationStep{
		{
			Name:  "step-1",
			Apply: func(context.Context) error { return nil },
			Compensate: func(context.Context) error {
				return errors.New("compensation failed")
			},
		},
		{
			Name: "step-2",
			Apply: func(context.Context) error {
				return errors.New("apply failed")
			},
		},
	})

	require.Error(t, err)
	var partialErr *blerrors.MutationPartialError
	require.ErrorAs(t, err, &partialErr)
	assert.Equal(t, []string{"step-1"}, partialErr.Completed)
	assert.Equal(t, "step-2", partialErr.FailedStep)
	assert.Equal(t, "unknown", partialErr.CompensationState)
	assert.Equal(t, "double-fault", partialErr.Class)
	assert.Contains(t, partialErr.Error(), "mutation partial")
}

func TestMutationEnvelope_RerunAfterFailureReturnsSameClassification(t *testing.T) {
	ctx := context.Background()

	steps := []core.MutationStep{
		{
			Name:       "step-1",
			Apply:      func(context.Context) error { return nil },
			Compensate: func(context.Context) error { return nil },
		},
		{
			Name: "step-2",
			Apply: func(context.Context) error {
				return errors.Join(blerrors.ErrWriteNotApplied, errors.New("write failed"))
			},
		},
	}

	firstErr := core.MutationEnvelope(ctx, steps)
	secondErr := core.MutationEnvelope(ctx, steps)

	require.Error(t, firstErr)
	require.Error(t, secondErr)

	var firstPartial *blerrors.MutationPartialError
	var secondPartial *blerrors.MutationPartialError
	require.ErrorAs(t, firstErr, &firstPartial)
	require.ErrorAs(t, secondErr, &secondPartial)
	assert.Equal(t, firstPartial.Class, secondPartial.Class)
	assert.Equal(t, firstPartial.Completed, secondPartial.Completed)
	assert.Equal(t, firstPartial.FailedStep, secondPartial.FailedStep)
}
