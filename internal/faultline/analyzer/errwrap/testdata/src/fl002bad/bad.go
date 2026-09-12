package fl002bad

import (
	stderrors "errors"
	format "fmt"
)

func percentV(err error) error {
	return format.Errorf("load: %v", err) // want "FL002"
}

func percentS(err error) error {
	return format.Errorf("load: %s", err) // want "FL002"
}

func percentPlusV() error {
	return format.Errorf("load: %+v", source()) // want "FL002"
}

func aliasErrorf(err error) error {
	return format.Errorf("load: %v", err) // want "FL002"
}

func nestedErrorsNew() error {
	return format.Errorf("load: %v", identity(stderrors.New("failed"))) // want "FL002"
}

func nestedErrorf() error {
	return format.Errorf("load: %s", format.Errorf("failed")) // want "FL002"
}

func source() error {
	return format.Errorf("source")
}

func identity(err error) error {
	return err
}
