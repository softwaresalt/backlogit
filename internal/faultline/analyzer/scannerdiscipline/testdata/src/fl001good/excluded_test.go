package fl001good

import (
	"bufio"
	"io"
)

func scannerInTestFile(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
}
