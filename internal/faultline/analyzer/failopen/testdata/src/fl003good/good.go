package fl003good

import "os"

func failClosed() error {
	if err := work(); err != nil {
		return err
	}
	return nil
}

func wrapped() error {
	if err := work(); err != nil {
		return wrap(err)
	}
	return nil
}

func suppressed() error {
	if err := work(); err != nil { // faultline:fail-open-ok
		return nil
	}
	return nil
}

func noErrorResult() *int {
	if err := work(); err != nil {
		return nil
	}
	value := 1
	return &value
}

func exits() error {
	if err := work(); err != nil {
		os.Exit(1)
	}
	return nil
}

func deferredCleanup() {
	defer func() error {
		if err := work(); err != nil {
			return nil
		}
		return nil
	}()
}

func work() error {
	return nil
}

func wrap(err error) error {
	return err
}
