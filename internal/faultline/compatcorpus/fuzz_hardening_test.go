package compatcorpus

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type fuzzTestAdapter struct {
	name   string
	decode func(context.Context, []byte) (DecodeResult, error)
}

func (a fuzzTestAdapter) Name() string {
	return a.name
}

func (a fuzzTestAdapter) Decode(ctx context.Context, input []byte) (DecodeResult, error) {
	return a.decode(ctx, input)
}

func TestFuzzDecodeNativeSeedRequiresExactlyOneArgument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		seed    string
		want    []byte
		wantErr bool
	}{
		{
			name: "one byte slice argument",
			seed: "go test fuzz v1\n[]byte(\"seed\")\n",
			want: []byte("seed"),
		},
		{
			name:    "second encoded expression",
			seed:    "go test fuzz v1\n[]byte(\"seed\")\n[]byte(\"trailing\")\n",
			wantErr: true,
		},
		{
			name:    "trailing data",
			seed:    "go test fuzz v1\n[]byte(\"seed\") trailing\n",
			wantErr: true,
		},
		{
			name:    "trailing comment data",
			seed:    "go test fuzz v1\n[]byte(\"seed\") // trailing\n",
			wantErr: true,
		},
		{
			name:    "trailing blank expression line",
			seed:    "go test fuzz v1\n[]byte(\"seed\")\n\n",
			wantErr: true,
		},
		{
			name:    "multiple conversion arguments",
			seed:    "go test fuzz v1\n[]byte(\"seed\", \"trailing\")\n",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := fuzzDecodeNativeSeed([]byte(test.seed))
			if test.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestFuzzReadNativeSeedsRejectsOversizedFileFromMetadata(t *testing.T) {
	root := t.TempDir()
	seedDir := filepath.Join(root, fuzzCanonicalSeedDir)
	require.NoError(t, os.MkdirAll(seedDir, 0o700))
	seedPath := filepath.Join(seedDir, "oversized")
	seed := bytes.Repeat([]byte("x"), fuzzMaximumSeedFileBytes+1)
	require.NoError(t, os.WriteFile(seedPath, seed, 0o600))
	t.Chdir(root)

	_, err := fuzzReadNativeSeeds()
	require.ErrorContains(t, err, "metadata")
	require.ErrorContains(t, err, "maximum")
}

func TestFuzzDecodeAllAdaptersRejectsInputsOutsideResourceBounds(t *testing.T) {
	t.Parallel()

	const expectedMaximumInputBytes = 64 << 10

	// Resource-rejected cases deterministically return errFuzzInputRejected.
	// The fuzz callback converts only that sentinel into an early skip; this
	// keeps the limits out of the production adapter contract.
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:  "byte boundary accepted",
			input: bytes.Repeat([]byte("a\n"), expectedMaximumInputBytes/2),
		},
		{
			name:    "one byte over boundary rejected",
			input:   bytes.Repeat([]byte("a"), expectedMaximumInputBytes+1),
			wantErr: true,
		},
		{
			name:  "structural depth boundary accepted",
			input: append(bytes.Repeat([]byte("["), 64), bytes.Repeat([]byte("]"), 64)...),
		},
		{
			name:    "structural depth over boundary rejected",
			input:   append(bytes.Repeat([]byte("["), 65), bytes.Repeat([]byte("]"), 65)...),
			wantErr: true,
		},
		{
			name:  "YAML indentation depth boundary accepted",
			input: deeplyNestedFuzzYAML(64),
		},
		{
			name:    "YAML indentation depth over boundary rejected",
			input:   deeplyNestedFuzzYAML(65),
			wantErr: true,
		},
		{
			name:    "compact YAML sequence depth over boundary rejected",
			input:   compactNestedFuzzYAML(64),
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := fuzzDecodeAllAdapters(context.Background(), test.input)
			if test.wantErr {
				require.ErrorIs(t, err, errFuzzInputRejected)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestFuzzResourceRejectionOccursBeforeAdapterCopies(t *testing.T) {
	t.Parallel()

	calls := 0
	adapter := fuzzTestAdapter{
		name: "tracking",
		decode: func(context.Context, []byte) (DecodeResult, error) {
			calls++
			return DecodeResult{}, nil
		},
	}

	_, err := fuzzDecodeAllAdaptersWith(
		context.Background(),
		bytes.Repeat([]byte("x"), fuzzMaximumInputBytes+1),
		map[string]ParserAdapter{adapter.Name(): adapter},
	)
	require.ErrorIs(t, err, errFuzzInputRejected)
	require.Zero(t, calls, "resource-rejected input must not reach or be copied for an adapter")
}

func TestFuzzCanonicalAdapterOutcomes(t *testing.T) {
	t.Parallel()

	adapters := map[string]ParserAdapter{
		"error": fuzzTestAdapter{
			name: "error",
			decode: func(context.Context, []byte) (DecodeResult, error) {
				return DecodeResult{Normalized: []byte("error bytes")}, fmt.Errorf(
					"test decode: %w",
					ErrMalformed,
				)
			},
		},
		"normalized": fuzzTestAdapter{
			name: "normalized",
			decode: func(context.Context, []byte) (DecodeResult, error) {
				return DecodeResult{Normalized: []byte("canonical bytes")}, nil
			},
		},
		"panic": fuzzTestAdapter{
			name: "panic",
			decode: func(context.Context, []byte) (DecodeResult, error) {
				panic("stable panic")
			},
		},
	}

	outcomes, err := fuzzDecodeAllAdaptersWith(context.Background(), []byte("input"), adapters)
	require.Error(t, err)
	require.Equal(t, []fuzzAdapterOutcome{
		{
			Adapter:        "error",
			Normalized:     []byte("error bytes"),
			Classification: "error:ErrMalformed",
		},
		{
			Adapter:        "normalized",
			Normalized:     []byte("canonical bytes"),
			Classification: "ok",
		},
		{
			Adapter:        "panic",
			Classification: `panic:string("stable panic")`,
		},
	}, outcomes)
}

func TestFuzzReplayGateDetectsAlternatingAdapter(t *testing.T) {
	t.Parallel()

	call := 0
	adapter := fuzzTestAdapter{
		name: "alternating",
		decode: func(context.Context, []byte) (DecodeResult, error) {
			call++
			if call%2 == 0 {
				return DecodeResult{}, fmt.Errorf("alternating decode: %w", ErrMalformed)
			}
			return DecodeResult{Normalized: []byte("odd")}, nil
		},
	}
	adapters := map[string]ParserAdapter{adapter.Name(): adapter}
	seeds := []fuzzSeed{{name: "seed", input: []byte("seed")}}

	first := fuzzReplaySeeds(t, seeds, adapters)
	second := fuzzReplaySeeds(t, seeds, adapters)

	err := fuzzCompareReplays(first, second)
	require.Error(t, err)
}

func deeplyNestedFuzzYAML(depth int) []byte {
	var output bytes.Buffer
	output.WriteString("---\n")
	for level := 0; level < depth; level++ {
		output.Write(bytes.Repeat([]byte(" "), level))
		output.WriteString("key:\n")
	}
	output.WriteString("---\n")
	return output.Bytes()
}

func compactNestedFuzzYAML(depth int) []byte {
	var output bytes.Buffer
	output.WriteString("---\n")
	for range depth {
		output.WriteString("- ")
	}
	output.WriteString("value\n---\n")
	return output.Bytes()
}
