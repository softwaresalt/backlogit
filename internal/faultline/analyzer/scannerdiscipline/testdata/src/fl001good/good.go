package fl001good

import (
	"bufio"
	"io"

	"notbufio"
)

var externalScanner *bufio.Scanner

type scannerHolder struct {
	scanner *bufio.Scanner
}

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

func suppressedOnPrecedingLine(r io.Reader) {
	// faultline:scanner-ok
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
}

func escapesByReturn(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
	return s
}

func escapesByCall(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
	consume(s)
}

func escapesByNamedResult(r io.Reader) (s *bufio.Scanner) {
	s = bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
	return
}

func escapesByMapKey(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
	_ = map[*bufio.Scanner]struct{}{s: {}}
}

func escapesByMapValue(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
	_ = map[string]*bufio.Scanner{"scanner": s}
}

func escapesByFieldAssignment(r io.Reader, holder *scannerHolder) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
	holder.scanner = s
}

func escapesByIndexAssignment(r io.Reader, scanners []*bufio.Scanner) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
	scanners[0] = s
}

func escapesByExternalAssignment(r io.Reader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
	externalScanner = s
}

func assignedErrResult(r io.Reader) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	err := s.Err()
	_ = err
}

func returnedErrResult(r io.Reader) error {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	return s.Err()
}

func switchedErrResult(r io.Reader) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for s.Scan() {
		_ = s.Text()
	}
	switch err := s.Err(); {
	case err != nil:
		return
	}
}

// methodExpressionScanIsExcluded records FL001's deliberate v.Scan() boundary.
func methodExpressionScanIsExcluded(r io.Reader) {
	s := bufio.NewScanner(r)
	scan := (*bufio.Scanner).Scan
	for scan(s) {
		_ = s.Text()
	}
}

func nameCompatibleNonBufioConstructorIsExcluded(r io.Reader) {
	s := notbufio.NewScanner(r)
	for s.Scan() {
		_ = s.Text()
	}
}

func consume(*bufio.Scanner) {}
