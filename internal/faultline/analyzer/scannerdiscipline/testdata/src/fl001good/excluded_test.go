package fl001good

import (
	"bufio"
	"io"
)

func scannerViolationInTestFileIsExcluded(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
}
