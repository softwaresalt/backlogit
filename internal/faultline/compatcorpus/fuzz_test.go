package compatcorpus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	fuzzCanonicalSeedDir       = "testdata/fuzz/FuzzCompatibilityCorpusDecode"
	fuzzMaximumInputBytes      = 64 << 10
	fuzzMaximumStructuralDepth = 64
	fuzzMaximumSeedFileBytes   = (fuzzMaximumInputBytes * 4) + 64
)

var fuzzAdapterOrder = []string{"events_jsonl", "frontmatter", "scanner"}

var errFuzzInputRejected = errors.New("compatcorpus fuzz: input exceeds resource bounds")

type fuzzSeed struct {
	name  string
	input []byte
}

type fuzzAdapterOutcome struct {
	Adapter        string
	Normalized     []byte
	Classification string
}

type fuzzSeedReplay struct {
	Seed     string
	Outcomes []fuzzAdapterOutcome
}

// TestFuzzSeedCorpusCommitted verifies the canonical native seed corpus and
// deterministic adapter replay contract owned by 158.008-T.
func TestFuzzSeedCorpusCommitted(t *testing.T) {
	nativeSeeds, err := fuzzReadNativeSeeds()
	if err != nil {
		t.Errorf("read canonical fuzz seed directory %q: %v", fuzzCanonicalSeedDir, err)
	}
	if len(nativeSeeds) == 0 {
		t.Errorf("canonical fuzz seed directory %q is empty", fuzzCanonicalSeedDir)
	}

	corpusEntries := mustDefaultCorpus(t)
	if len(corpusEntries) == 0 {
		t.Fatal("DefaultCorpus returned no runtime fuzz seeds")
	}

	seeds := make([]fuzzSeed, 0, len(nativeSeeds)+len(corpusEntries))
	seeds = append(seeds, nativeSeeds...)
	for _, entry := range corpusEntries {
		seeds = append(seeds, fuzzSeed{
			name:  "corpus/" + entry.ID,
			input: append([]byte(nil), entry.Input...),
		})
	}

	adapters := DefaultAdapters()
	first := fuzzReplaySeeds(t, seeds, adapters)
	second := fuzzReplaySeeds(t, seeds, adapters)
	if err := fuzzCompareReplays(first, second); err != nil {
		t.Errorf("seed replay is nondeterministic: %v", err)
	}
}

// FuzzCompatibilityCorpusDecode is the bounded single-package fuzz harness for
// every strict compatibility-corpus adapter.
func FuzzCompatibilityCorpusDecode(f *testing.F) {
	entries, err := DefaultCorpus()
	if err != nil {
		f.Fatalf("DefaultCorpus: %v", err)
	}
	for _, entry := range entries {
		if err := fuzzValidateInput(entry.Input); err != nil {
			f.Fatalf("DefaultCorpus entry %q exceeds fuzz resource bounds: %v", entry.ID, err)
		}
		f.Add(entry.Input)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		outcomes, err := fuzzDecodeAllAdapters(context.Background(), input)
		// Oversized or over-depth values are deterministically skipped before
		// any per-adapter copy or parser call. These are fuzz-harness limits,
		// not changes to the production ParserAdapter contract.
		if errors.Is(err, errFuzzInputRejected) {
			return
		}
		if err != nil {
			t.Fatalf("decode fuzz input: %v", err)
		}
		adapterNames := fuzzOutcomeAdapterNames(outcomes)
		if !reflect.DeepEqual(adapterNames, fuzzAdapterOrder) {
			t.Fatalf("adapter replay order = %v, want %v", adapterNames, fuzzAdapterOrder)
		}
	})
}

func fuzzReplaySeeds(
	t *testing.T,
	seeds []fuzzSeed,
	adapters map[string]ParserAdapter,
) []fuzzSeedReplay {
	t.Helper()

	replays := make([]fuzzSeedReplay, 0, len(seeds))
	expectedAdapterNames := make([]string, 0, len(adapters))
	for name := range adapters {
		expectedAdapterNames = append(expectedAdapterNames, name)
	}
	sort.Strings(expectedAdapterNames)

	for _, seed := range seeds {
		outcomes, err := fuzzDecodeAllAdaptersWith(
			context.Background(),
			seed.input,
			adapters,
		)
		if err != nil {
			t.Errorf("%s adapter replay failed: %v", seed.name, err)
		}
		adapterNames := fuzzOutcomeAdapterNames(outcomes)
		if !reflect.DeepEqual(adapterNames, expectedAdapterNames) {
			t.Errorf(
				"%s adapter replay order = %v, want %v",
				seed.name,
				adapterNames,
				expectedAdapterNames,
			)
		}
		replays = append(replays, fuzzSeedReplay{
			Seed:     seed.name,
			Outcomes: outcomes,
		})
	}
	return replays
}

func fuzzCompareReplays(first, second []fuzzSeedReplay) error {
	if !reflect.DeepEqual(first, second) {
		return fmt.Errorf("canonical adapter outcomes differ:\nfirst: %#v\nsecond: %#v", first, second)
	}
	return nil
}

func fuzzDecodeAllAdapters(ctx context.Context, input []byte) ([]fuzzAdapterOutcome, error) {
	return fuzzDecodeAllAdaptersWith(ctx, input, DefaultAdapters())
}

func fuzzDecodeAllAdaptersWith(
	ctx context.Context,
	input []byte,
	adapters map[string]ParserAdapter,
) ([]fuzzAdapterOutcome, error) {
	if err := fuzzValidateInput(input); err != nil {
		return nil, err
	}

	adapterNames := make([]string, 0, len(adapters))
	for name := range adapters {
		adapterNames = append(adapterNames, name)
	}
	sort.Strings(adapterNames)

	outcomes := make([]fuzzAdapterOutcome, 0, len(adapterNames))
	var replayErrors []error
	for _, name := range adapterNames {
		adapter := adapters[name]
		if adapter == nil {
			outcomes = append(outcomes, fuzzAdapterOutcome{
				Adapter:        name,
				Classification: "invalid:nil-adapter",
			})
			replayErrors = append(replayErrors, fmt.Errorf("adapter %q is nil", name))
			continue
		}

		decoded, panicContext, panicked, decodeErr := decodeSafely(
			ctx,
			adapter,
			bytes.Clone(input),
		)
		outcomes = append(outcomes, fuzzAdapterOutcome{
			Adapter:        name,
			Normalized:     fuzzCanonicalBytes(decoded.Normalized),
			Classification: fuzzOutcomeClassification(decodeErr, panicContext, panicked),
		})
		if panicked {
			replayErrors = append(
				replayErrors,
				fmt.Errorf("adapter %q panicked: %s", name, panicContext),
			)
		}
	}
	return outcomes, errors.Join(replayErrors...)
}

func fuzzCanonicalBytes(input []byte) []byte {
	if len(input) == 0 {
		return nil
	}
	return bytes.Clone(input)
}

func fuzzOutcomeAdapterNames(outcomes []fuzzAdapterOutcome) []string {
	names := make([]string, 0, len(outcomes))
	for _, outcome := range outcomes {
		names = append(names, outcome.Adapter)
	}
	return names
}

func fuzzOutcomeClassification(err error, panicContext string, panicked bool) string {
	if panicked {
		return "panic:" + panicContext
	}
	if err == nil {
		return "ok"
	}

	knownErrors := []struct {
		err  error
		name string
	}{
		{ErrTruncated, "ErrTruncated"},
		{ErrMalformed, "ErrMalformed"},
		{ErrDuplicateKey, "ErrDuplicateKey"},
		{ErrCaseFoldCollision, "ErrCaseFoldCollision"},
		{ErrOldVersion, "ErrOldVersion"},
		{ErrTokenTooLong, "ErrTokenTooLong"},
		{ErrInvalidPath, "ErrInvalidPath"},
		{ErrUnclosedFront, "ErrUnclosedFront"},
		{context.Canceled, "context.Canceled"},
		{context.DeadlineExceeded, "context.DeadlineExceeded"},
	}
	for _, known := range knownErrors {
		if errors.Is(err, known.err) {
			return "error:" + known.name
		}
	}
	return "error:" + reflect.TypeOf(err).String()
}

func fuzzValidateInput(input []byte) error {
	if len(input) > fuzzMaximumInputBytes {
		return fmt.Errorf(
			"input has %d bytes, maximum is %d: %w",
			len(input),
			fuzzMaximumInputBytes,
			errFuzzInputRejected,
		)
	}

	depth := 0
	quote := byte(0)
	escaped := false
	for _, current := range input {
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case current == '\\' && quote == '"':
				escaped = true
			case current == quote:
				quote = 0
			}
			continue
		}

		switch current {
		case '"', '\'':
			quote = current
		case '{', '[':
			depth++
			if depth > fuzzMaximumStructuralDepth {
				return fmt.Errorf(
					"input structural depth exceeds %d: %w",
					fuzzMaximumStructuralDepth,
					errFuzzInputRejected,
				)
			}
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	if err := fuzzValidateYAMLDepth(input); err != nil {
		return err
	}
	return nil
}

func fuzzValidateYAMLDepth(input []byte) error {
	var content []byte
	switch {
	case bytes.HasPrefix(input, []byte("---\n")):
		content = input[len("---\n"):]
	case bytes.HasPrefix(input, []byte("---\r\n")):
		content = input[len("---\r\n"):]
	default:
		return nil
	}

	var indentation [fuzzMaximumStructuralDepth + 1]int
	level := 0
	haveLine := false
	for len(content) > 0 {
		line, rest, found := bytes.Cut(content, []byte("\n"))
		content = rest
		line = bytes.TrimSuffix(line, []byte("\r"))
		if bytes.Equal(line, []byte("---")) {
			return nil
		}

		indent := 0
		for indent < len(line) && line[indent] == ' ' {
			indent++
		}
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 || trimmed[0] == '#' {
			if !found {
				return nil
			}
			continue
		}

		if !haveLine {
			indentation[0] = indent
			haveLine = true
		} else if indent > indentation[level] {
			level++
			if level+1 > fuzzMaximumStructuralDepth {
				return fmt.Errorf(
					"input YAML depth exceeds %d: %w",
					fuzzMaximumStructuralDepth,
					errFuzzInputRejected,
				)
			}
			indentation[level] = indent
		} else {
			for level > 0 && indent < indentation[level] {
				level--
			}
			indentation[level] = indent
		}
		compactDepth := level + 1 + fuzzYAMLCompactSequenceDepth(line[indent:])
		if compactDepth > fuzzMaximumStructuralDepth {
			return fmt.Errorf(
				"input YAML depth exceeds %d: %w",
				fuzzMaximumStructuralDepth,
				errFuzzInputRejected,
			)
		}

		if !found {
			return nil
		}
	}
	return nil
}

func fuzzYAMLCompactSequenceDepth(line []byte) int {
	depth := 0
	for {
		line = bytes.TrimLeft(line, " \t")
		if len(line) < 2 || line[0] != '-' || (line[1] != ' ' && line[1] != '\t') {
			return depth
		}
		depth++
		line = line[2:]
	}
}

func fuzzReadNativeSeeds() ([]fuzzSeed, error) {
	entries, err := os.ReadDir(fuzzCanonicalSeedDir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	seeds := make([]fuzzSeed, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			return nil, fmt.Errorf("seed entry %q is a directory", entry.Name())
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("inspect seed %q metadata: %w", entry.Name(), err)
		}
		if info.Size() > int64(fuzzMaximumSeedFileBytes) {
			return nil, fmt.Errorf(
				"inspect seed %q metadata: native seed file has %d bytes, maximum is %d",
				entry.Name(),
				info.Size(),
				fuzzMaximumSeedFileBytes,
			)
		}

		seedPath := filepath.Join(fuzzCanonicalSeedDir, entry.Name())
		data, err := os.ReadFile(seedPath)
		if err != nil {
			return nil, fmt.Errorf("read seed %q: %w", entry.Name(), err)
		}
		if len(data) > fuzzMaximumSeedFileBytes {
			return nil, fmt.Errorf(
				"read seed %q: native seed file has %d bytes, maximum is %d",
				entry.Name(),
				len(data),
				fuzzMaximumSeedFileBytes,
			)
		}
		afterRead, err := os.Stat(seedPath)
		if err != nil {
			return nil, fmt.Errorf("reinspect seed %q metadata: %w", entry.Name(), err)
		}
		if afterRead.Size() != info.Size() || afterRead.Size() != int64(len(data)) {
			return nil, fmt.Errorf(
				"read seed %q: file size changed from %d to %d bytes while reading %d bytes",
				entry.Name(),
				info.Size(),
				afterRead.Size(),
				len(data),
			)
		}
		input, err := fuzzDecodeNativeSeed(data)
		if err != nil {
			return nil, fmt.Errorf("decode seed %q: %w", entry.Name(), err)
		}
		seeds = append(seeds, fuzzSeed{name: "native/" + entry.Name(), input: input})
	}
	sort.Slice(seeds, func(i, j int) bool {
		return seeds[i].name < seeds[j].name
	})
	return seeds, nil
}

func fuzzDecodeNativeSeed(data []byte) ([]byte, error) {
	if len(data) > fuzzMaximumSeedFileBytes {
		return nil, fmt.Errorf(
			"native seed file has %d bytes, maximum is %d",
			len(data),
			fuzzMaximumSeedFileBytes,
		)
	}

	normalized := strings.ReplaceAll(string(data), "\r\n", "\n")
	header, body, found := strings.Cut(normalized, "\n")
	if !found || header != "go test fuzz v1" {
		return nil, fmt.Errorf("missing Go fuzz corpus header")
	}
	body = strings.TrimSuffix(body, "\n")
	if body == "" || strings.Contains(body, "\n") {
		return nil, fmt.Errorf("native seed must contain exactly one encoded argument")
	}

	expression := strings.TrimSpace(body)
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseExprFrom(fileSet, "fuzz-seed", expression, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse seed expression: %w", err)
	}
	if fileSet.Position(parsed.End()).Offset != len(expression) {
		return nil, fmt.Errorf("seed expression has trailing data")
	}

	call, ok := parsed.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || call.Ellipsis.IsValid() {
		return nil, fmt.Errorf("seed expression %q is not []byte", expression)
	}
	arrayType, ok := call.Fun.(*ast.ArrayType)
	if !ok || arrayType.Len != nil {
		return nil, fmt.Errorf("seed expression %q is not []byte", expression)
	}
	element, ok := arrayType.Elt.(*ast.Ident)
	if !ok || element.Name != "byte" {
		return nil, fmt.Errorf("seed expression %q is not []byte", expression)
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return nil, fmt.Errorf("seed expression %q does not contain encoded bytes", expression)
	}

	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return nil, fmt.Errorf("unquote seed bytes: %w", err)
	}
	decoded := []byte(value)
	if err := fuzzValidateInput(decoded); err != nil {
		return nil, fmt.Errorf("validate seed input: %w", err)
	}
	return decoded, nil
}
