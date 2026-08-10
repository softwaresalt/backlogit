package core

import (
	"context"
	"errors"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// MutationStep describes one ordered mutation and its compensation.
type MutationStep struct {
	Name       string
	Apply      func(context.Context) error
	Compensate func(context.Context) error
}

// MutationEnvelope executes ordered mutation steps and classifies partial
// failures into machine-readable outcomes.
func MutationEnvelope(ctx context.Context, steps []MutationStep) error {
	appliedIndexes := make([]int, 0, len(steps))
	appliedNames := make([]string, 0, len(steps))

	for i, step := range steps {
		var stepErr error
		if step.Apply != nil {
			stepErr = step.Apply(ctx)
		}
		if stepErr == nil {
			appliedIndexes = append(appliedIndexes, i)
			appliedNames = append(appliedNames, step.Name)
			continue
		}

		if blerrors.IsWriteIndeterminate(stepErr) {
			allErrs := []error{stepErr}
			for j := i + 1; j < len(steps); j++ {
				if steps[j].Apply == nil {
					continue
				}
				if contErr := steps[j].Apply(ctx); contErr != nil {
					allErrs = append(allErrs, contErr)
				}
			}
			return &blerrors.MutationPartialError{
				Completed:         appliedNames,
				FailedStep:        step.Name,
				CompensationState: "not-compensated",
				Class:             "indeterminate",
				Cause:             errors.Join(allErrs...),
			}
		}

		var compensationErr error
		for j := len(appliedIndexes) - 1; j >= 0; j-- {
			appliedStep := steps[appliedIndexes[j]]
			if appliedStep.Compensate == nil {
				continue
			}
			if err := appliedStep.Compensate(ctx); err != nil {
				compensationErr = errors.Join(compensationErr, err)
			}
		}
		if compensationErr != nil {
			return &blerrors.MutationPartialError{
				Completed:         appliedNames,
				FailedStep:        step.Name,
				CompensationState: "unknown",
				Class:             "double-fault",
				Cause:             errors.Join(stepErr, compensationErr),
			}
		}
		return &blerrors.MutationPartialError{
			Completed:         appliedNames,
			FailedStep:        step.Name,
			CompensationState: "compensated",
			Class:             "not-applied",
			Cause:             stepErr,
		}
	}

	return nil
}
