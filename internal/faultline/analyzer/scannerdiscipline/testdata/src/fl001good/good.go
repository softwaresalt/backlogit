package fl001good

import (
	"bufio"
	"io"
)

func disciplined(r io.Reader) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	if err := s.Err(); err != nil {
		return err
	}
	return nil
}

func suppressed(r io.Reader) {
	s := bufio.NewScanner(r) // faultline:scanner-ok
	for s.Scan() {
		_ = s.Text()
	}
}

func escapesByReturn(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	return s
}

func escapesByCall(r io.Reader) {
	s := bufio.NewScanner(r)
	consume(s)
}

func consume(*bufio.Scanner) {}
