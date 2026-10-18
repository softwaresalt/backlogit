package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	backlogiterrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// Semantics-matrix behavior harness (171.002-T). These tests mutate no global
// seam and therefore may not run in parallel with seam-swapping tests; they
// intentionally do not call t.Parallel().

const (
	inWindowMissing = `{"agent":"ship","session_id":"s1","phase":"build"}`
	inWindowZero    = `{"schema_version":0,"agent":"stage","session_id":"s1","phase":"build","context":{"branch":"b","custom":"x"}}`
	validV1         = `{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","context":{"branch":"b"}}`
)

func TestCreateCheckpoint_SchemaMatrix_Default(t *testing.T) {
	tests := []struct {
		name string
		dump string
		want error
	}{
		{"missing in window", inWindowMissing, backlogiterrors.ErrCheckpointSchemaRejected},
		{"zero in window", inWindowZero, backlogiterrors.ErrCheckpointSchemaRejected},
		{"missing no identity", `{"phase":"build"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"consumer legacy shape", `{"consumer":"ship","phase":"build","shipment_id":"x"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"foreign top-level member", `{"agent":"ship","session_id":"s","phase":"p","findings":[]}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"future 2", `{"schema_version":2,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"negative", `{"schema_version":-1,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"string zero", `{"schema_version":"0","agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"string one", `{"schema_version":"1","agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"null", `{"schema_version":null,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"non-integral", `{"schema_version":1.5,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"overflow", `{"schema_version":99999999999999999999,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"duplicate", `{"schema_version":0,"schema_version":0,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"fold duplicate", `{"schema_version":1,"SCHEMA_VERSION":1,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"array", `[]`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"scalar", `5`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"null document", `null`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"malformed", `{"schema_version":1,`, backlogiterrors.ErrCheckpointMalformedInput},
		{"wrong-typed context", `{"agent":"ship","session_id":"s","phase":"p","context":"bad"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"invalid nested progress", `{"agent":"ship","session_id":"s","phase":"p","progress":{"bogus":1}}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"duplicate context members", `{"agent":"ship","session_id":"s","phase":"p","context":{"branch":"a","branch":"b"}}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			_, err := events.CreateCheckpoint(context.Background(), dir, tc.dump)
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "got %v, want %v", err, tc.want)
			if tc.want == backlogiterrors.ErrCheckpointSchemaUnsupported {
				assert.False(t, errors.Is(err, backlogiterrors.ErrCheckpointSchemaRejected))
			}
			assertNoCheckpointWritten(t, dir)
		})
	}
}

func TestCreateCheckpoint_SchemaMatrix_OptInFailClosed(t *testing.T) {
	tests := []struct {
		name string
		dump string
		want error
	}{
		{"consumer legacy shape", `{"consumer":"ship","phase":"build"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"missing session_id", `{"agent":"ship","phase":"build"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"future 2", `{"schema_version":2,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"string zero", `{"schema_version":"0","agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"duplicate", `{"schema_version":0,"schema_version":0,"agent":"ship","session_id":"s","phase":"p"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"array", `[1]`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"wrong-typed context", `{"agent":"ship","session_id":"s","phase":"p","context":"bad"}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"invalid nested progress", `{"agent":"ship","session_id":"s","phase":"p","progress":{"bogus":1}}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"duplicate context members", `{"agent":"ship","session_id":"s","phase":"p","context":{"branch":"a","branch":"b"}}`, backlogiterrors.ErrCheckpointSchemaUnsupported},
		{"malformed", `{"agent":`, backlogiterrors.ErrCheckpointMalformedInput},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			_, err := events.CreateCheckpoint(context.Background(), dir, tc.dump, events.WithAllowLegacyImport())
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.want), "got %v, want %v", err, tc.want)
			assertNoCheckpointWritten(t, dir)
		})
	}
}

// F1a (171.007-T): a present-but-null status/created_at/updated_at in a legacy
// dump is invalid (not defaultable); only an absent key is defaulted. The
// "dry-run" scenario is the default (no opt-in) path, which runs the same
// dry-run to decide the window; the "upgrade" scenario is the opt-in path.
func TestCreateCheckpoint_SchemaMatrix_PresentNullFieldsInvalid(t *testing.T) {
	rows := []struct {
		name string
		dump string
	}{
		{"null status", `{"agent":"ship","session_id":"s1","phase":"build","status":null}`},
		{"null created_at", `{"agent":"ship","session_id":"s1","phase":"build","created_at":null}`},
		{"null updated_at", `{"agent":"ship","session_id":"s1","phase":"build","updated_at":null}`},
	}
	scenarios := []struct {
		name string
		opts []events.CreateCheckpointOption
	}{
		{"upgrade", []events.CreateCheckpointOption{events.WithAllowLegacyImport()}},
		{"dry-run", nil},
	}
	for _, sc := range scenarios {
		for _, row := range rows {
			t.Run(sc.name+"/"+row.name, func(t *testing.T) {
				dir := t.TempDir()
				_, err := events.CreateCheckpoint(context.Background(), dir, row.dump, sc.opts...)
				require.Error(t, err)
				assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointSchemaUnsupported), "got %v", err)
				assert.False(t, errors.Is(err, backlogiterrors.ErrCheckpointSchemaRejected))
				assertNoCheckpointWritten(t, dir)
			})
		}
	}
}

func TestCreateCheckpoint_SchemaMatrix_CorpusFixturesFailClosed(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "legacy-corpus", "*.json"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			raw, err := os.ReadFile(f)
			require.NoError(t, err)
			for _, opts := range [][]events.CreateCheckpointOption{nil, {events.WithAllowLegacyImport()}} {
				dir := t.TempDir()
				_, err = events.CreateCheckpoint(context.Background(), dir, string(raw), opts...)
				require.Error(t, err)
				assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointSchemaUnsupported), "got %v", err)
				assertNoCheckpointWritten(t, dir)
			}
		})
	}
}

func TestCreateCheckpoint_SchemaMatrix_OptInUpgradesInWindow(t *testing.T) {
	for name, dump := range map[string]string{"missing": inWindowMissing, "zero": inWindowZero} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			res, err := events.CreateCheckpoint(context.Background(), dir, dump, events.WithAllowLegacyImport())
			require.NoError(t, err)

			raw, err := os.ReadFile(res.Path)
			require.NoError(t, err)
			var doc map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &doc))
			assert.JSONEq(t, `1`, string(doc["schema_version"]), "written file must carry canonical schema_version 1")
			assert.Contains(t, doc, "created_at")
			assert.Contains(t, doc, "updated_at")
			assert.JSONEq(t, `"active"`, string(doc["status"]))
			assert.NotEqual(t, dump, string(raw), "never written verbatim")

			cp, err := events.GetCheckpoint(context.Background(), dir, filepath.Base(res.Path))
			require.NoError(t, err)
			assert.Equal(t, 1, cp.SchemaVersion)
			assert.NotNil(t, res.ContextKeys)

			list, err := events.ListCheckpoints(context.Background(), dir, events.CheckpointFilter{})
			require.NoError(t, err)
			assert.Len(t, list, 1)
		})
	}
}

func TestCreateCheckpoint_SchemaMatrix_ValidV1Unchanged(t *testing.T) {
	for name, dump := range map[string]string{
		"plain":      validV1,
		"fold-key":   `{"SCHEMA_VERSION":1,"agent":"ship","session_id":"s1","phase":"build"}`,
		"with-optin": validV1,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			var opts []events.CreateCheckpointOption
			if name == "with-optin" {
				opts = append(opts, events.WithAllowLegacyImport())
			}
			res, err := events.CreateCheckpoint(context.Background(), dir, dump, opts...)
			require.NoError(t, err)
			_, err = events.GetCheckpoint(context.Background(), dir, filepath.Base(res.Path))
			require.NoError(t, err)
		})
	}
}

func TestCreateCheckpoint_SchemaMatrix_CanonicalGrowthPastLimit(t *testing.T) {
	// In-window input just under 64 KiB whose canonical form (timestamps,
	// status, schema_version, indentation) exceeds the limit.
	prefix := `{"agent":"ship","session_id":"s1","phase":"build","context":{"blob":"`
	suffix := `"}}`
	pad := 65536 - len(prefix) - len(suffix) - 8
	dump := prefix + strings.Repeat("a", pad) + suffix
	require.Less(t, len(dump), 65536)

	for _, opts := range [][]events.CreateCheckpointOption{nil, {events.WithAllowLegacyImport()}} {
		dir := t.TempDir()
		_, err := events.CreateCheckpoint(context.Background(), dir, dump, opts...)
		require.Error(t, err)
		assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpTooLarge), "got %v", err)
		assert.False(t, errors.Is(err, backlogiterrors.ErrCheckpointSchemaRejected))
		assert.False(t, errors.Is(err, backlogiterrors.ErrCheckpointSchemaUnsupported))
		assertNoCheckpointWritten(t, dir)
	}
}

func TestCreateCheckpoint_SchemaMatrix_PreClassificationGuards(t *testing.T) {
	escapedSecret := `{"agent":"ship","session_id":"s1","phase":"build","context":{"note":"\u0067hp_abcdefghijklmnopqrstuvwxyz0123456789"}}`
	tooLarge := `{"agent":"ship","session_id":"s1","phase":"build","context":{"blob":"` + strings.Repeat("a", 70000) + `"}}`
	for _, opts := range [][]events.CreateCheckpointOption{nil, {events.WithAllowLegacyImport()}} {
		dir := t.TempDir()
		_, err := events.CreateCheckpoint(context.Background(), dir, escapedSecret, opts...)
		require.Error(t, err)
		assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpSecretDetected), "got %v", err)
		assertNoCheckpointWritten(t, dir)

		_, err = events.CreateCheckpoint(context.Background(), dir, tooLarge, opts...)
		require.Error(t, err)
		assert.True(t, errors.Is(err, backlogiterrors.ErrCheckpointStateDumpTooLarge), "got %v", err)
		assertNoCheckpointWritten(t, dir)
	}
}

func TestCreateCheckpoint_SchemaMatrix_V1GuardsPreserved(t *testing.T) {
	dir := t.TempDir()
	_, err := events.CreateCheckpoint(context.Background(), dir,
		`{"schema_version":1,"agent":"ship","session_id":"s","phase":"p","context":{"branch":"a","branch":"b"}}`)
	var dup *backlogiterrors.CheckpointDuplicateContextKeyError
	require.True(t, errors.As(err, &dup))
	assertNoCheckpointWritten(t, dir)

	_, err = events.CreateCheckpoint(context.Background(), dir,
		`{"schema_version":1,"agent":"ship","session_id":"s","phase":"p","extra":1}`)
	require.True(t, errors.Is(err, backlogiterrors.ErrCheckpointUnknownField))
	assertNoCheckpointWritten(t, dir)
}
