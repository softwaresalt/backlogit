package fl002bad

import "fmt"

func unrelatedPriorComment(err error) error {
	// faultline:errwrap-ok

	return fmt.Errorf("load: %v", err) // want "FL002"
}

func nondedicatedPriorComment(err error) error {
	_ = "reviewed"                     // faultline:errwrap-ok
	return fmt.Errorf("load: %v", err) // want "FL002"
}

func nestedComment(err error) error {
	return fmt.Errorf(
		"load: %v",
		// faultline:errwrap-ok
		err, // want "FL002"
	)
}

func outerStatementCommentDoesNotSuppressNestedCall(err error) error {
	// faultline:errwrap-ok
	return identity(fmt.Errorf("load: %v", err)) // want "FL002"
}
