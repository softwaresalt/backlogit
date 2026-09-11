package fl002good

import (
	. "errors"
	. "fmt"
)

func dotImportedErrorsNew() error {
	return Errorf("load: %s", New("failed"))
}
