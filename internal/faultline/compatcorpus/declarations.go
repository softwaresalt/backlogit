// Package compatcorpus defines compatibility-corpus declarations for parser
// fault-line testing.
package compatcorpus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"strconv"
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

// Run executes entries against their named adapters and returns a stable report.
func Run(ctx context.Context, entries []Entry, adapters map[string]ParserAdapter) Report {
	if ctx == nil {
		ctx = context.Background()
	}

	report := Report{
		SchemaVersion: "compatcorpus.report/v1",
		Total:         len(entries),
		Deterministic: true,
		Results:       make([]Result, 0, len(entries)),
	}
	for _, entry := range entries {
		result := runEntry(ctx, entry, adapters[entry.Adapter])
		report.Results = append(report.Results, result)
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}
	sort.SliceStable(report.Results, func(i, j int) bool {
		if report.Results[i].EntryID != report.Results[j].EntryID {
			return report.Results[i].EntryID < report.Results[j].EntryID
		}
		return report.Results[i].Adapter < report.Results[j].Adapter
	})
	return report
}

// DefaultCorpus loads the embedded compatibility corpus.
func DefaultCorpus() ([]Entry, error) {
	entries, err := loadDefaultCorpus()
	if err != nil {
		return nil, fmt.Errorf("load default compatibility corpus: %w", err)
	}
	return entries, nil
}

// DefaultAdapters returns the strict adapters used by the compatibility corpus.
func DefaultAdapters() map[string]ParserAdapter {
	return newDefaultAdapters()
}

// LoadCorpus loads a compatibility corpus from a manifest-rooted file system.
func LoadCorpus(fsys fs.FS) ([]Entry, error) {
	entries, err := loadCorpus(fsys)
	if err != nil {
		return nil, fmt.Errorf("load compatibility corpus: %w", err)
	}
	return entries, nil
}

// JSON returns stable machine-readable report serialization.
func (r Report) JSON() ([]byte, error) {
	results := append([]Result(nil), r.Results...)
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].EntryID != results[j].EntryID {
			return results[i].EntryID < results[j].EntryID
		}
		return results[i].Adapter < results[j].Adapter
	})
	if results == nil {
		results = make([]Result, 0)
	}

	document := reportDocument{
		SchemaVersion: r.SchemaVersion,
		Total:         r.Total,
		Passed:        r.Passed,
		Failed:        r.Failed,
		Deterministic: r.Deterministic,
		Results:       make([]resultDocument, 0, len(results)),
	}
	for _, result := range results {
		document.Results = append(document.Results, resultDocument(result))
	}

	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("marshal compatibility report: %w", err)
	}
	return data, nil
}

type reportDocument struct {
	SchemaVersion string           `json:"schema_version"`
	Total         int              `json:"total"`
	Passed        int              `json:"passed"`
	Failed        int              `json:"failed"`
	Deterministic bool             `json:"deterministic"`
	Results       []resultDocument `json:"results"`
}

type resultDocument struct {
	EntryID string `json:"entry_id"`
	Adapter string `json:"adapter"`
	Passed  bool   `json:"passed"`
	GotErr  string `json:"got_err"`
	Reason  string `json:"reason"`
}

func runEntry(ctx context.Context, entry Entry, adapter ParserAdapter) Result {
	result := Result{
		EntryID: entry.ID,
		Adapter: entry.Adapter,
	}
	if adapter == nil {
		result.Reason = "named adapter is not registered"
		return result
	}

	decoded, panicContext, panicked, err := decodeSafely(ctx, adapter, bytes.Clone(entry.Input))
	if panicked {
		result.GotErr = "adapter panic: " + panicContext
		result.Reason = "adapter panicked while decoding"
		return result
	}
	if err != nil {
		result.GotErr = err.Error()
	}

	switch entry.Expect {
	case ExpectRejected:
		switch {
		case err == nil:
			result.Reason = "input was accepted; rejection required"
		case isDecodeAbort(err) && entry.WantErr == nil:
			result.Reason = "decode was canceled; explicit matching rejection error required"
		case entry.WantErr != nil && !errors.Is(err, entry.WantErr):
			result.Reason = "decoder returned a different error"
		default:
			result.Passed = true
		}
	case ExpectAccepted:
		if err != nil {
			result.Reason = "decoder rejected input; acceptance required"
		} else {
			result.Passed = true
		}
	case ExpectNormalized:
		switch {
		case err != nil:
			result.Reason = "decoder rejected input; normalized acceptance required"
		case !bytes.Equal(decoded.Normalized, entry.WantNormalized):
			result.Reason = "decoder returned different normalized bytes"
		default:
			result.Passed = true
		}
	default:
		result.Reason = "unsupported expectation"
	}
	return result
}

func isDecodeAbort(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func decodeSafely(
	ctx context.Context,
	adapter ParserAdapter,
	input []byte,
) (decoded DecodeResult, panicContext string, panicked bool, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			decoded = DecodeResult{}
			err = nil
			panicContext = describePanic(recovered)
			panicked = true
		}
	}()
	decoded, err = adapter.Decode(ctx, input)
	return decoded, "", false, err
}

func describePanic(recovered any) string {
	recoveredType := reflect.TypeOf(recovered)
	if recoveredType == nil {
		return "<nil>"
	}

	if recoveredErr, ok := recovered.(error); ok {
		message, formatted := panicSafeErrorString(recoveredErr)
		if !formatted {
			return recoveredType.String() + "(<Error() panicked>)"
		}
		return recoveredType.String() + "(" + strconv.Quote(message) + ")"
	}

	recoveredValue := reflect.ValueOf(recovered)
	var value string
	switch recoveredValue.Kind() {
	case reflect.String:
		value = strconv.Quote(recoveredValue.String())
	case reflect.Bool:
		value = strconv.FormatBool(recoveredValue.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value = strconv.FormatInt(recoveredValue.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value = strconv.FormatUint(recoveredValue.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		value = strconv.FormatFloat(
			recoveredValue.Float(),
			'g',
			-1,
			recoveredType.Bits(),
		)
	case reflect.Complex64, reflect.Complex128:
		value = strconv.FormatComplex(
			recoveredValue.Complex(),
			'g',
			-1,
			recoveredType.Bits(),
		)
	default:
		return recoveredType.String()
	}
	return recoveredType.String() + "(" + value + ")"
}

func panicSafeErrorString(err error) (message string, formatted bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			message = ""
			formatted = false
		}
	}()
	return err.Error(), true
}
