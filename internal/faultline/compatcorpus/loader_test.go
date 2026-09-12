package compatcorpus

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestLoadCorpusValidationFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest string
		addInput bool
		wantErr  error
	}{
		{
			name:     "missing input file",
			manifest: `[{"id":"entry","category":"lf","adapter":"scanner","expect":"accepted"}]`,
			wantErr:  fs.ErrNotExist,
		},
		{
			name:     "unknown adapter",
			manifest: `[{"id":"entry","category":"lf","adapter":"unknown","expect":"accepted"}]`,
			addInput: true,
			wantErr:  fs.ErrInvalid,
		},
		{
			name:     "unknown sentinel",
			manifest: `[{"id":"entry","category":"lf","adapter":"scanner","expect":"rejected","want_err":"ErrUnknown"}]`,
			addInput: true,
			wantErr:  fs.ErrInvalid,
		},
		{
			name:     "malformed manifest",
			manifest: `[{"id":`,
			wantErr:  nil,
		},
		{
			name:     "invalid id",
			manifest: `[{"id":"bad/id","category":"lf","adapter":"scanner","expect":"accepted"}]`,
			addInput: true,
			wantErr:  fs.ErrInvalid,
		},
		{
			name:     "unknown category",
			manifest: `[{"id":"entry","category":"unknown","adapter":"scanner","expect":"accepted"}]`,
			addInput: true,
			wantErr:  fs.ErrInvalid,
		},
		{
			name:     "unknown expectation",
			manifest: `[{"id":"entry","category":"lf","adapter":"scanner","expect":"unknown"}]`,
			addInput: true,
			wantErr:  fs.ErrInvalid,
		},
		{
			name:     "accepted entry declares rejection field",
			manifest: `[{"id":"entry","category":"lf","adapter":"scanner","expect":"accepted","want_err":"ErrMalformed"}]`,
			addInput: true,
			wantErr:  fs.ErrInvalid,
		},
		{
			name:     "rejected entry declares normalized field",
			manifest: `[{"id":"entry","category":"lf","adapter":"scanner","expect":"rejected","want_normalized_file":"lf/entry.normalized"}]`,
			addInput: true,
			wantErr:  fs.ErrInvalid,
		},
		{
			name:     "normalized entry omits normalized file",
			manifest: `[{"id":"entry","category":"lf","adapter":"scanner","expect":"normalized"}]`,
			addInput: true,
			wantErr:  fs.ErrInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			corpusFS := fstest.MapFS{
				"manifest.json": &fstest.MapFile{Data: []byte(test.manifest)},
			}
			if test.addInput {
				corpusFS["lf/entry.input"] = &fstest.MapFile{Data: []byte("input")}
			}

			_, err := LoadCorpus(corpusFS)
			if err == nil {
				t.Fatal("LoadCorpus() error = nil, want validation failure")
			}
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Errorf("LoadCorpus() error = %v, want errors.Is(_, %v)", err, test.wantErr)
			}
		})
	}
}
