package fl003bad

import "unsafe"

type record struct{}

func nilError() error {
	if err := work(); err != nil { // want "FL003"
		return nil
	}
	return nil
}

func zeroValuesAndNil() (string, error) {
	if err := work(); err != nil { // want "FL003"
		return "", nil
	}
	return "ok", nil
}

func reverseNilComparison() error {
	if err := work(); nil != err { // want "FL003"
		return nil
	}
	return nil
}

func typedPointerZero() (*record, error) {
	if err := work(); err != nil { // want "FL003"
		return (*record)(nil), nil
	}
	return &record{}, nil
}

func typedUnsafePointerZero() (unsafe.Pointer, error) {
	if err := work(); err != nil { // want "FL003"
		return unsafe.Pointer(nil), nil
	}
	return unsafe.Pointer(&record{}), nil
}

func untypedUnsafePointerZero() (unsafe.Pointer, error) {
	if err := work(); err != nil { // want "FL003"
		return nil, nil
	}
	return unsafe.Pointer(&record{}), nil
}

func typedNilError() error {
	if err := work(); err != nil { // want "FL003"
		return error(nil)
	}
	return nil
}

func nestedAssignedClosure() error {
	check := func() error {
		if err := work(); err != nil { // want "FL003"
			return nil
		}
		return nil
	}
	return check()
}

func nestedImmediateClosure() error {
	return func() error {
		if err := work(); nil != err { // want "FL003"
			return nil
		}
		return nil
	}()
}

func nonImmediateSuppression() error {
	// faultline:fail-open-ok

	if err := work(); err != nil { // want "FL003"
		return nil
	}
	return nil
}

func previousStatementDoesNotOwnSuppression() error {
	_ = "reviewed"                 // faultline:fail-open-ok
	if err := work(); err != nil { // want "FL003"
		return nil
	}
	return nil
}

func statementCommentDoesNotSuppressOuter() error {
	if err := work(); err != nil { // want "FL003"
		func() {
			// faultline:fail-open-ok
		}()
		return nil
	}
	return nil
}

func inlineNestedStatementCommentDoesNotSuppressOuter() error {
	if err := work(); // want "FL003"
	err != nil {
		func() { // faultline:fail-open-ok
		}()
		return nil
	}
	return nil
}

func closureHeaderCommentDoesNotSuppressOuter() error {
	if err := func() error { // want "FL003"
		// faultline:fail-open-ok
		return work()
	}(); err != nil {
		return nil
	}
	return nil
}

func closureStatementCommentDoesNotSuppressOuter() error {
	if err := func() error { // want "FL003"
		return work() // faultline:fail-open-ok
	}(); err != nil {
		return nil
	}
	return nil
}

func work() error {
	return nil
}
