package fl003good

import (
	"os"
	"unsafe"
)

type record struct{}

type recordError struct{}

func (*recordError) Error() string {
	return "record error"
}

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

func suppressedDedicated() error {
	// faultline:fail-open-ok
	if err := work(); err != nil {
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

func panics() error {
	if err := work(); err != nil {
		panic(err)
	}
	return nil
}

func errorResultIsNotLast() (error, bool) {
	if err := work(); err != nil {
		return nil, false
	}
	return nil, true
}

func shadowedNilCondition() error {
	nil := work()
	if err := work(); err != nil {
		return nil
	}
	return nil
}

func shadowedNilReturn() error {
	if err := work(); err != nil {
		nil := err
		return nil
	}
	return nil
}

func runtimePointerValue() (*record, error) {
	if err := work(); err != nil {
		return pointerValue(), nil
	}
	return &record{}, nil
}

func runtimeUnsafePointerValue() (unsafe.Pointer, error) {
	if err := work(); err != nil {
		value := &record{}
		return unsafe.Pointer(value), nil
	}
	return unsafe.Pointer(&record{}), nil
}

func typedNilConcreteError() error {
	if err := work(); err != nil {
		return (*recordError)(nil)
	}
	return nil
}

func typedNilComparisonIsNotNilIdentity() error {
	if err := work(); err != (*recordError)(nil) {
		return nil
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

func pointerValue() *record {
	return nil
}

func wrap(err error) error {
	return err
}
