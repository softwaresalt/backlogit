package fl002bad

import . "fmt"

func dotImportedErrorf(err error) error {
	return Errorf("load: %v", err) // want "FL002"
}
