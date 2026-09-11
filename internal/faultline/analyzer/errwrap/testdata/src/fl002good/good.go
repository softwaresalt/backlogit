package fl002good

import (
	stderrors "errors"
	"fmt"
)

func wrapped(err error) error {
	return fmt.Errorf("load: %w", err)
}

func dynamicFormat(format string, err error) error {
	return fmt.Errorf(format, err)
}

func nonErrorPosition(value string) error {
	return fmt.Errorf("load: %s", value)
}

func suppressed(err error) error {
	// faultline:errwrap-ok
	return fmt.Errorf("load: %v", err)
}

func suppressedSameLine(err error) error {
	return fmt.Errorf("load: %v", err) // faultline:errwrap-ok
}

func suppressedMultiline(err error) error {
	// faultline:errwrap-ok
	return fmt.Errorf(
		"load: %v",
		err,
	)
}

func suppressedAfterRelatedComment(err error) error {
	// The loss of wrapping is intentional at this boundary.
	// faultline:errwrap-ok
	return fmt.Errorf("load: %v", err)
}

func suppressedNestedCall(err error) error {
	return nestedIdentity(
		// faultline:errwrap-ok
		fmt.Errorf("load: %v", err),
	)
}

func suppressedNestedCallSameLine(err error) error {
	return nestedIdentity(
		fmt.Errorf("load: %v", err), // faultline:errwrap-ok
	)
}

func newError() error {
	return stderrors.New("load failed")
}

func directErrorsNew() error {
	return fmt.Errorf("load: %v", stderrors.New("failed"))
}

func flagsAfterIndexAreNotFlags(err error) error {
	return fmt.Errorf("%[2]+v", "ignored", err)
}

func malformedOnly(err error) error {
	return fmt.Errorf("%[x]v", err)
}

func outOfRangeOnly(err error) error {
	return fmt.Errorf("%[2]v", err)
}

func danglingDirective(err error) error {
	return fmt.Errorf("load: %", err)
}

func nestedIdentity(err error) error {
	return err
}
