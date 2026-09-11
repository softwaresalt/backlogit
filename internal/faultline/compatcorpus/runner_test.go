package compatcorpus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
)

type corpusContractExpectation struct {
	adapter        string
	expect         Expectation
	wantErr        error
	wantNormalized []byte
}

var corpusContractExpectations = map[string]corpusContractExpectation{
	"json-malformed-01": {
		adapter: "events_jsonl",
		expect:  ExpectRejected,
		wantErr: ErrMalformed,
	},
	"json-truncated-01": {
		adapter: "events_jsonl",
		expect:  ExpectRejected,
		wantErr: ErrTruncated,
	},
	"json-dupkey-01": {
		adapter: "events_jsonl",
		expect:  ExpectRejected,
		wantErr: ErrDuplicateKey,
	},
	"json-oldver-01": {
		adapter: "events_jsonl",
		expect:  ExpectRejected,
		wantErr: ErrOldVersion,
	},
	"yaml-malformed-01": {
		adapter: "frontmatter",
		expect:  ExpectRejected,
		wantErr: ErrMalformed,
	},
	"yaml-truncated-01": {
		adapter: "frontmatter",
		expect:  ExpectRejected,
		wantErr: ErrUnclosedFront,
	},
	"yaml-dupkey-01": {
		adapter: "frontmatter",
		expect:  ExpectRejected,
		wantErr: ErrDuplicateKey,
	},
	"yaml-casefold-01": {
		adapter: "frontmatter",
		expect:  ExpectRejected,
		wantErr: ErrCaseFoldCollision,
	},
	"crlf-01": {
		adapter:        "frontmatter",
		expect:         ExpectNormalized,
		wantNormalized: []byte("---\nid: x\npath: notes/item.md\n---\nbody\n"),
	},
	"lf-01": {
		adapter: "frontmatter",
		expect:  ExpectAccepted,
	},
	"token-oversized-01": {
		adapter: "scanner",
		expect:  ExpectRejected,
		wantErr: ErrTokenTooLong,
	},
	"token-ok-01": {
		adapter: "scanner",
		expect:  ExpectAccepted,
	},
	"windows-semantics-01": {
		adapter: "frontmatter",
		expect:  ExpectRejected,
		wantErr: ErrInvalidPath,
	},
}

// TestCorpusRunner verifies the deterministic runner, strict adapters, corpus
// fixtures, panic containment, and versioned report contract owned by 158.009-T.
func TestCorpusRunner(t *testing.T) {
	t.Run("YAML trailing documents preserve classification sentinels", func(t *testing.T) {
		tests := []struct {
			name    string
			input   []byte
			wantErr error
		}{
			{
				name:    "multiple documents are malformed",
				input:   []byte("id: first\n---\nid: second\n"),
				wantErr: ErrMalformed,
			},
			{
				name:    "truncated trailing document is truncated",
				input:   []byte("id: first\n---\nname: \"unfinished"),
				wantErr: ErrTruncated,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				_, err := decodeYAMLNode(test.input)
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("decodeYAMLNode() error = %v, want errors.Is(_, %v)", err, test.wantErr)
				}
			})
		}
	})

	t.Run("default corpus declares every representative case", func(t *testing.T) {
		entries := mustDefaultCorpus(t)
		byID := make(map[string]Entry, len(entries))
		for _, entry := range entries {
			if _, exists := byID[entry.ID]; exists {
				t.Errorf("duplicate corpus entry ID %q", entry.ID)
			}
			byID[entry.ID] = entry
		}

		for id, want := range corpusContractExpectations {
			entry, ok := byID[id]
			if !ok {
				t.Errorf("required corpus entry %q is missing", id)
				continue
			}
			if entry.Adapter != want.adapter {
				t.Errorf("%s adapter = %q, want %q", id, entry.Adapter, want.adapter)
			}
			if entry.Expect != want.expect {
				t.Errorf("%s expectation = %q, want %q", id, entry.Expect, want.expect)
			}
			if want.wantErr != nil && !errors.Is(entry.WantErr, want.wantErr) {
				t.Errorf("%s WantErr = %v, want sentinel %v", id, entry.WantErr, want.wantErr)
			}
			if !bytes.Equal(entry.WantNormalized, want.wantNormalized) {
				t.Errorf(
					"%s normalized bytes = %q, want %q",
					id,
					entry.WantNormalized,
					want.wantNormalized,
				)
			}
		}
		assertCRLFFixtureBytes(t, entries)
	})

	t.Run("strict adapters produce declared outcomes", func(t *testing.T) {
		entries := mustDefaultCorpus(t)
		adapters := mustDefaultAdapters(t)

		for _, entry := range entries {
			entry := entry
			t.Run(entry.ID, func(t *testing.T) {
				adapter, ok := adapters[entry.Adapter]
				if !ok {
					t.Fatalf("adapter %q for corpus entry %q is not registered", entry.Adapter, entry.ID)
				}
				if adapter.Name() != entry.Adapter {
					t.Fatalf("adapter map key %q has Name() %q", entry.Adapter, adapter.Name())
				}

				got, err := decodeWithoutPanic(t, adapter, entry.Input)
				switch entry.Expect {
				case ExpectRejected:
					if err == nil {
						t.Fatalf("%s was accepted, want rejection", entry.ID)
					}
					if entry.WantErr != nil && !errors.Is(err, entry.WantErr) {
						t.Fatalf("%s error = %v, want sentinel %v", entry.ID, err, entry.WantErr)
					}
				case ExpectAccepted:
					if err != nil {
						t.Fatalf("%s error = %v, want acceptance", entry.ID, err)
					}
				case ExpectNormalized:
					if err != nil {
						t.Fatalf("%s error = %v, want normalized acceptance", entry.ID, err)
					}
					if !bytes.Equal(got.Normalized, entry.WantNormalized) {
						t.Fatalf(
							"%s normalized bytes = %q, want %q",
							entry.ID,
							got.Normalized,
							entry.WantNormalized,
						)
					}
				default:
					t.Fatalf("%s has unsupported expectation %q", entry.ID, entry.Expect)
				}
			})
		}
	})

	t.Run("runner emits stable sorted successful report", func(t *testing.T) {
		entries := mustDefaultCorpus(t)
		adapters := mustDefaultAdapters(t)
		first := runWithoutPanic(t, entries, adapters)
		second := runWithoutPanic(t, entries, adapters)

		if first.SchemaVersion != "compatcorpus.report/v1" {
			t.Errorf("schema version = %q, want compatcorpus.report/v1", first.SchemaVersion)
		}
		if first.Total != len(entries) || first.Passed != len(entries) || first.Failed != 0 {
			t.Errorf(
				"report counts = total:%d passed:%d failed:%d, want %d/%d/0",
				first.Total,
				first.Passed,
				first.Failed,
				len(entries),
				len(entries),
			)
		}
		if !first.Deterministic {
			t.Error("report Deterministic = false, want true")
		}
		if !sort.SliceIsSorted(first.Results, func(i, j int) bool {
			if first.Results[i].EntryID != first.Results[j].EntryID {
				return first.Results[i].EntryID < first.Results[j].EntryID
			}
			return first.Results[i].Adapter < first.Results[j].Adapter
		}) {
			t.Error("report results are not sorted by (EntryID, Adapter)")
		}

		firstJSON := reportJSONWithoutPanic(t, first)
		secondJSON := reportJSONWithoutPanic(t, second)
		if !bytes.Equal(firstJSON, secondJSON) {
			t.Errorf("identical runs emitted different JSON:\nfirst: %s\nsecond: %s", firstJSON, secondJSON)
		}
		var document map[string]json.RawMessage
		if err := json.Unmarshal(firstJSON, &document); err != nil {
			t.Fatalf("Report.JSON returned invalid JSON: %v", err)
		}
		for _, key := range []string{
			"schema_version",
			"total",
			"passed",
			"failed",
			"deterministic",
			"results",
		} {
			if _, ok := document[key]; !ok {
				t.Errorf("Report.JSON is missing %q", key)
			}
		}
		var schemaVersion string
		if err := json.Unmarshal(document["schema_version"], &schemaVersion); err != nil {
			t.Fatalf("decode report schema_version: %v", err)
		}
		if schemaVersion != "compatcorpus.report/v1" {
			t.Errorf("JSON schema_version = %q, want compatcorpus.report/v1", schemaVersion)
		}
	})

	t.Run("runner converts adapter panic to a failed result", func(t *testing.T) {
		tests := []struct {
			name       string
			adapter    ParserAdapter
			wantGotErr string
		}{
			{
				name:       "string",
				adapter:    panicAdapter{},
				wantGotErr: `adapter panic: string("adapter panic")`,
			},
			{
				name:       "error",
				adapter:    errorPanicAdapter{},
				wantGotErr: `adapter panic: *errors.errorString("decode exploded")`,
			},
			{
				name:       "scalar",
				adapter:    scalarPanicAdapter{},
				wantGotErr: "adapter panic: int16(-23)",
			},
			{
				name:       "nondeterministic pointer",
				adapter:    pointerPanicAdapter{payload: &panicPayload{value: 42}},
				wantGotErr: "adapter panic: *compatcorpus.panicPayload",
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				entry := Entry{
					ID:      "panic-01",
					Adapter: test.adapter.Name(),
					Input:   []byte("boom"),
					Expect:  ExpectAccepted,
				}
				adapters := map[string]ParserAdapter{test.adapter.Name(): test.adapter}
				first := runWithoutPanic(t, []Entry{entry}, adapters)
				second := runWithoutPanic(t, []Entry{entry}, adapters)

				if first.Total != 1 || first.Passed != 0 || first.Failed != 1 {
					t.Fatalf(
						"panic report counts = total:%d passed:%d failed:%d, want 1/0/1",
						first.Total,
						first.Passed,
						first.Failed,
					)
				}
				if len(first.Results) != 1 ||
					first.Results[0].Passed ||
					first.Results[0].Reason == "" {
					t.Fatalf("panic result = %+v, want one failed result with a reason", first.Results)
				}
				if first.Results[0].GotErr != test.wantGotErr {
					t.Errorf("panic GotErr = %q, want %q", first.Results[0].GotErr, test.wantGotErr)
				}

				firstJSON := reportJSONWithoutPanic(t, first)
				secondJSON := reportJSONWithoutPanic(t, second)
				if !bytes.Equal(firstJSON, secondJSON) {
					t.Errorf(
						"identical panic runs emitted different JSON:\nfirst: %s\nsecond: %s",
						firstJSON,
						secondJSON,
					)
				}
				if bytes.Contains(firstJSON, []byte("goroutine ")) ||
					bytes.Contains(firstJSON, []byte("0x")) {
					t.Errorf("panic report JSON contains nondeterministic runtime context: %s", firstJSON)
				}
			})
		}
	})
}

type panicAdapter struct{}

func (panicAdapter) Name() string {
	return "panic"
}

func (panicAdapter) Decode(context.Context, []byte) (DecodeResult, error) {
	panic("adapter panic")
}

type errorPanicAdapter struct{}

func (errorPanicAdapter) Name() string {
	return "error_panic"
}

func (errorPanicAdapter) Decode(context.Context, []byte) (DecodeResult, error) {
	panic(errors.New("decode exploded"))
}

type scalarPanicAdapter struct{}

func (scalarPanicAdapter) Name() string {
	return "scalar_panic"
}

func (scalarPanicAdapter) Decode(context.Context, []byte) (DecodeResult, error) {
	panic(int16(-23))
}

type panicPayload struct {
	value int
}

type pointerPanicAdapter struct {
	payload *panicPayload
}

func (pointerPanicAdapter) Name() string {
	return "pointer_panic"
}

func (a pointerPanicAdapter) Decode(context.Context, []byte) (DecodeResult, error) {
	panic(a.payload)
}

func mustDefaultCorpus(t *testing.T) (entries []Entry) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("158.009-T behavior assertion: DefaultCorpus must load fixtures without panic: %v", recovered)
		}
	}()

	entries, err := DefaultCorpus()
	if err != nil {
		t.Fatalf("DefaultCorpus: %v", err)
	}
	return entries
}

func mustDefaultAdapters(t *testing.T) (adapters map[string]ParserAdapter) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("158.009-T behavior assertion: DefaultAdapters must return strict adapters without panic: %v", recovered)
		}
	}()

	adapters = DefaultAdapters()
	for _, name := range []string{"events_jsonl", "frontmatter", "scanner"} {
		if adapters[name] == nil {
			t.Errorf("default adapter %q is not registered", name)
		}
	}
	return adapters
}

func runWithoutPanic(
	t *testing.T,
	entries []Entry,
	adapters map[string]ParserAdapter,
) (report Report) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("158.009-T behavior assertion: Run must return a report without panic: %v", recovered)
		}
	}()
	return Run(context.Background(), entries, adapters)
}

func decodeWithoutPanic(
	t *testing.T,
	adapter ParserAdapter,
	input []byte,
) (result DecodeResult, err error) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("adapter %q panicked: %v", adapter.Name(), recovered)
		}
	}()
	return adapter.Decode(context.Background(), input)
}

func reportJSONWithoutPanic(t *testing.T, report Report) (data []byte) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("158.009-T behavior assertion: Report.JSON must serialize without panic: %v", recovered)
		}
	}()

	data, err := report.JSON()
	if err != nil {
		t.Fatalf("Report.JSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Report.JSON returned an empty document")
	}
	return data
}

func assertCRLFFixtureBytes(t *testing.T, entries []Entry) {
	t.Helper()
	for _, entry := range entries {
		if entry.ID != "crlf-01" {
			continue
		}
		if !bytes.Contains(entry.Input, []byte("\r\n")) {
			t.Error("crlf-01 input does not preserve CRLF bytes")
		}
		if strings.Contains(string(entry.WantNormalized), "\r\n") {
			t.Error("crlf-01 normalized bytes still contain CRLF")
		}
		return
	}
	t.Error("crlf-01 is absent")
}
