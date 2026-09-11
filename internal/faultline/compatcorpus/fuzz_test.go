package compatcorpus

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const fuzzCanonicalSeedDir = "testdata/fuzz/FuzzCompatibilityCorpusDecode"

var fuzzAdapterOrder = []string{"events_jsonl", "frontmatter", "scanner"}

type fuzzSeed struct {
	name  string
	input []byte
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

	first := fuzzReplaySeeds(t, seeds)
	second := fuzzReplaySeeds(t, seeds)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("seed replay order is nondeterministic:\nfirst: %v\nsecond: %v", first, second)
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
		f.Add(entry.Input)
	}

	f.Fuzz(func(t *testing.T, input []byte) {
		adapterNames, err := fuzzDecodeAllAdapters(context.Background(), input)
		if err != nil {
			t.Fatalf("decode fuzz input: %v", err)
		}
		if !reflect.DeepEqual(adapterNames, fuzzAdapterOrder) {
			t.Fatalf("adapter replay order = %v, want %v", adapterNames, fuzzAdapterOrder)
		}
	})
}

func fuzzReplaySeeds(t *testing.T, seeds []fuzzSeed) []string {
	t.Helper()

	trace := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		adapterNames, err := fuzzDecodeAllAdapters(context.Background(), seed.input)
		if err != nil {
			t.Errorf("%s adapter replay failed: %v", seed.name, err)
		}
		if !reflect.DeepEqual(adapterNames, fuzzAdapterOrder) {
			t.Errorf("%s adapter replay order = %v, want %v", seed.name, adapterNames, fuzzAdapterOrder)
		}
		trace = append(trace, seed.name+":"+strings.Join(adapterNames, ","))
	}
	return trace
}

func fuzzDecodeAllAdapters(ctx context.Context, input []byte) ([]string, error) {
	adapters := DefaultAdapters()
	adapterNames := make([]string, 0, len(adapters))
	for name := range adapters {
		adapterNames = append(adapterNames, name)
	}
	sort.Strings(adapterNames)

	var replayErrors []error
	for _, name := range adapterNames {
		adapter := adapters[name]
		if adapter == nil {
			replayErrors = append(replayErrors, fmt.Errorf("adapter %q is nil", name))
			continue
		}

		_, panicContext, panicked, decodeErr := decodeSafely(ctx, adapter, bytes.Clone(input))
		_ = decodeErr // Decode errors are valid fuzz outcomes; only panics violate the property.
		if panicked {
			replayErrors = append(
				replayErrors,
				fmt.Errorf("adapter %q panicked: %s", name, panicContext),
			)
		}
	}
	return adapterNames, errors.Join(replayErrors...)
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
		data, err := os.ReadFile(filepath.Join(fuzzCanonicalSeedDir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read seed %q: %w", entry.Name(), err)
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
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) < 2 || lines[0] != "go test fuzz v1" {
		return nil, fmt.Errorf("missing Go fuzz corpus header")
	}

	expression := strings.TrimSpace(lines[1])
	if !strings.HasPrefix(expression, "[]byte(") || !strings.HasSuffix(expression, ")") {
		return nil, fmt.Errorf("seed expression %q is not []byte", expression)
	}
	literal := strings.TrimSuffix(strings.TrimPrefix(expression, "[]byte("), ")")
	value, err := strconv.Unquote(literal)
	if err != nil {
		return nil, fmt.Errorf("unquote seed bytes: %w", err)
	}
	return []byte(value), nil
}
