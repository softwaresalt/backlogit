package fl002good

import (
	"errors"
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

func newError() error {
	return errors.New("load failed")
}
