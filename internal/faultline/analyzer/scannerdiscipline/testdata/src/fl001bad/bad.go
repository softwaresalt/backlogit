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

func storedThenDiscardedErrResult(r io.Reader) {
	s := bufio.NewScanner(r) // want "FL001"
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	err := s.Err()
	_ = err
}

func deadAssignedErrResult(r io.Reader) {
	var err error
	s := bufio.NewScanner(r) // want "FL001"
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	err = s.Err()
	_ = err
}

func inexactSuppression(r io.Reader) {
	// faultline:scanner-ok because the caller checks the error
	s := bufio.NewScanner(r) // want "FL001"
	for s.Scan() {
		_ = s.Text()
	}
}

func replacementNewScannerHasOwnLifetime(first, second io.Reader) {
	s := bufio.NewScanner(first)
	s = bufio.NewScanner(second) // want "FL001"
	for s.Scan() {
		_ = s.Text()
	}
}

func assignmentAfterLoopDoesNotHideMissingErr(r io.Reader, replacement *bufio.Scanner) {
	s := bufio.NewScanner(r) // want "FL001"
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	s = replacement
}

func conditionalReplacementDoesNotEndLifetime(r io.Reader, replacement *bufio.Scanner, replace bool) {
	s := bufio.NewScanner(r) // want "FL001"
	if replace {
		s = replacement
	}
	for s.Scan() {
		_ = s.Text()
	}
}

func inLoopReplacementDoesNotEndLifetime(r io.Reader, replacement *bufio.Scanner) {
	s := bufio.NewScanner(r) // want "FL001"
	for s.Scan() {
		_ = s.Text()
		s = replacement
	}
}

func errOnInLoopReplacementDoesNotCheckOriginal(r io.Reader, replacement *bufio.Scanner) error {
	s := bufio.NewScanner(r) // want "FL001"
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
		s = replacement
	}
	return s.Err()
}

func selfReplacementDoesNotEndLifetime(r io.Reader) {
	s := bufio.NewScanner(r) // want "FL001"
	s = s
	for s.Scan() {
		_ = s.Text()
	}
}

func aliasReplacementDoesNotEndLifetime(r io.Reader) {
	s := bufio.NewScanner(r) // want "FL001"
	alias := s
	s = alias
	for s.Scan() {
		_ = s.Text()
	}
}
