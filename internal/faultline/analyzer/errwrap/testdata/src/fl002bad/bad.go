package fl002bad

import "fmt"

func percentV(err error) error {
	return fmt.Errorf("load: %v", err) // want "FL002"
}

func percentS(err error) error {
	return fmt.Errorf("load: %s", err) // want "FL002"
}

func percentPlusV() error {
	return fmt.Errorf("load: %+v", source()) // want "FL002"
}

func source() error {
	return fmt.Errorf("source")
}
