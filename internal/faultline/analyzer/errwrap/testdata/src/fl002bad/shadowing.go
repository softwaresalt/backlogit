package fl002bad

import "fmt"

type errorFactory struct{}

func (errorFactory) New(string) error {
	return source()
}

func shadowedErrorsNew() error {
	errors := errorFactory{}
	return fmt.Errorf("load: %v", errors.New("failed")) // want "FL002"
}

func shadowedDotImportName() error {
	New := func(string) error { return source() }
	return fmt.Errorf("load: %v", New("failed")) // want "FL002"
}
