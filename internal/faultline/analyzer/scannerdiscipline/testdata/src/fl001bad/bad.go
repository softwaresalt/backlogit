package fl001bad

import (
	"bufio"
	"io"
)

func missingBoth(r io.Reader) {
	s := bufio.NewScanner(r) // want "FL001"
	for s.Scan() {
		_ = s.Text()
	}
}

func missingErr(r io.Reader) {
	s := bufio.NewScanner(r) // want "FL001"
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
}

func missingBuffer(r io.Reader) error {
	s := bufio.NewScanner(r) // want "FL001"
	for s.Scan() {
		_ = s.Text()
	}
	return s.Err()
}

func bufferAfterScanLoop(r io.Reader) error {
	s := bufio.NewScanner(r) // want "FL001"
	for s.Scan() {
		_ = s.Text()
	}
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	return s.Err()
}

func errBeforeScanLoop(r io.Reader) error {
	s := bufio.NewScanner(r) // want "FL001"
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	if err := s.Err(); err != nil {
		return err
	}
	for s.Scan() {
		_ = s.Text()
	}
	return nil
}

func localAliasIsNotEscape(r io.Reader) {
	s := bufio.NewScanner(r) // want "FL001"
	alias := s
	_ = alias
	for s.Scan() {
		_ = s.Text()
	}
}

func discardedErrResult(r io.Reader) {
	s := bufio.NewScanner(r) // want "FL001"
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	s.Err()
}

func blankErrResult(r io.Reader) {
	s := bufio.NewScanner(r) // want "FL001"
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	_ = s.Err()
}

func inexactSuppression(r io.Reader) {
	// faultline:scanner-ok because the caller checks the error
	s := bufio.NewScanner(r) // want "FL001"
	for s.Scan() {
		_ = s.Text()
	}
}
