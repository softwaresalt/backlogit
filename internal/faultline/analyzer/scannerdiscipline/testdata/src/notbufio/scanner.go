package notbufio

import "io"

// Scanner is a name-compatible scanner implementation outside package bufio.
type Scanner struct{}

// NewScanner returns a scanner with a bufio-compatible constructor name.
func NewScanner(io.Reader) *Scanner {
	return &Scanner{}
}

// Scan advances the scanner.
func (*Scanner) Scan() bool {
	return false
}

// Text returns the current token.
func (*Scanner) Text() string {
	return ""
}
