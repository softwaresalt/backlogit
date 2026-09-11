// Package compatcorpus defines compatibility-corpus declarations for parser
// fault-line testing.
package compatcorpus

import (
	"context"
	"errors"
	"io/fs"
)

// Category identifies a compatibility-corpus fault category.
type Category string

const (
	// CatMalformedJSON identifies malformed JSON input.
	CatMalformedJSON Category = "malformed_json"
	// CatTruncatedJSON identifies truncated JSON input.
	CatTruncatedJSON Category = "truncated_json"
	// CatMalformedYAML identifies malformed YAML input.
	CatMalformedYAML Category = "malformed_yaml"
	// CatTruncatedYAML identifies truncated YAML input.
	CatTruncatedYAML Category = "truncated_yaml"
	// CatDuplicateKey identifies input containing duplicate keys.
	CatDuplicateKey Category = "duplicate_key"
	// CatCaseFoldedKey identifies keys that collide after case folding.
	CatCaseFoldedKey Category = "case_folded_key"
	// CatCRLF identifies input using CRLF line endings.
	CatCRLF Category = "crlf"
	// CatLF identifies input using LF line endings.
	CatLF Category = "lf"
	// CatOversizedToken identifies input containing an oversized token.
	CatOversizedToken Category = "oversized_token"
	// CatOldIndexVersion identifies an unsupported old index version.
	CatOldIndexVersion Category = "old_index_version"
	// CatWindowsSemantics identifies input with invalid Windows path semantics.
	CatWindowsSemantics Category = "windows_semantics"
)

// Expectation identifies the expected result of decoding a corpus entry.
type Expectation string

const (
	// ExpectRejected requires decoding to return an error.
	ExpectRejected Expectation = "rejected"
	// ExpectAccepted requires decoding to succeed.
	ExpectAccepted Expectation = "accepted"
	// ExpectNormalized requires decoding to produce canonical normalized bytes.
	ExpectNormalized Expectation = "normalized"
)

var (
	// ErrTruncated identifies truncated input.
	ErrTruncated = errors.New("compatcorpus: truncated input")
	// ErrMalformed identifies malformed input.
	ErrMalformed = errors.New("compatcorpus: malformed input")
	// ErrDuplicateKey identifies duplicate keys.
	ErrDuplicateKey = errors.New("compatcorpus: duplicate key")
	// ErrCaseFoldCollision identifies keys that collide after case folding.
	ErrCaseFoldCollision = errors.New("compatcorpus: case-folded key collision")
	// ErrOldVersion identifies an unsupported old index version.
	ErrOldVersion = errors.New("compatcorpus: unsupported/old index version")
	// ErrTokenTooLong identifies a token that exceeds the configured bound.
	ErrTokenTooLong = errors.New("compatcorpus: token exceeds bound")
	// ErrInvalidPath identifies an invalid or reserved path.
	ErrInvalidPath = errors.New("compatcorpus: invalid/reserved path")
	// ErrUnclosedFront identifies an unclosed frontmatter fence.
	ErrUnclosedFront = errors.New("compatcorpus: unclosed frontmatter fence")
)

// Entry describes one compatibility-corpus input and its expected outcome.
type Entry struct {
	ID             string
	Category       Category
	Adapter        string
	Input          []byte
	Expect         Expectation
	WantErr        error
	WantNormalized []byte
}

// DecodeResult contains canonical bytes produced while decoding an entry.
type DecodeResult struct {
	Normalized []byte
}

// ParserAdapter decodes compatibility-corpus input.
type ParserAdapter interface {
	Name() string
	Decode(ctx context.Context, input []byte) (DecodeResult, error)
}

// Result records the outcome of running one corpus entry.
type Result struct {
	EntryID string
	Adapter string
	Passed  bool
	GotErr  string
	Reason  string
}

// Report summarizes compatibility-corpus results.
type Report struct {
	SchemaVersion string
	Total         int
	Passed        int
	Failed        int
	Deterministic bool
	Results       []Result
}

// Run declares the compatibility-corpus runner.
func Run(context.Context, []Entry, map[string]ParserAdapter) Report {
	panic("not implemented")
}

// DefaultCorpus declares access to the default compatibility corpus.
func DefaultCorpus() ([]Entry, error) {
	panic("not implemented")
}

// DefaultAdapters declares access to the default parser adapters.
func DefaultAdapters() map[string]ParserAdapter {
	panic("not implemented")
}

// LoadCorpus declares compatibility-corpus loading from a file system.
func LoadCorpus(fs.FS) ([]Entry, error) {
	panic("not implemented")
}

// JSON declares stable machine-readable report serialization.
func (Report) JSON() ([]byte, error) {
	panic("not implemented")
}
