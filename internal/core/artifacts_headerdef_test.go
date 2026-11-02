package core_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// countArtifactFiles walks the workspace storage root and returns the number of
// on-disk artifact markdown files. It is used to assert that a fail-closed write
// persisted nothing.
func countArtifactFiles(t *testing.T, ws *core.Workspace) int {
	t.Helper()
	root := core.WorkspaceStorageRoot(ws.RootPath)
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".md" {
			count++
		}
		return nil
	})
	require.NoError(t, err)
	return count
}

// findArtifactFile walks the workspace storage root and returns the on-disk path
// of the artifact file named "<id>.md", or "" if it is not present.
func findArtifactFile(t *testing.T, ws *core.Workspace, id string) string {
	t.Helper()
	root := core.WorkspaceStorageRoot(ws.RootPath)
	want := id + ".md"
	found := ""
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == want {
			found = path
		}
		return nil
	})
	require.NoError(t, err)
	return found
}

// TestCreateArtifact_NilHeaderDefFailsClosed proves that CreateArtifact fails
// closed (ErrConfig, not ErrValidation) when the workspace header-def schema is
// absent, persisting no artifact — while the loaded-schema path still succeeds.
// Covers AC1, AC2, AC3, AC4 for the create write path.
func TestCreateArtifact_NilHeaderDefFailsClosed(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Loaded-HeaderDef regression guard: the normal (schema-present) create path
	// still succeeds and persists.
	loaded, err := core.CreateArtifact(ctx, ws, "Loaded feature", "feature")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.FileExists(t, findArtifactFile(t, ws, loaded.ID))

	// Snapshot the persisted-file count immediately before the nil-schema create.
	before := countArtifactFiles(t, ws)

	// Force the defensive branch: absent workspace schema.
	ws.HeaderDef = nil

	got, err := core.CreateArtifact(ctx, ws, "Nil feature", "feature")

	// AC1: fail closed with a non-nil error wrapping ErrConfig, not ErrValidation.
	require.Error(t, err)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, blerrors.ErrConfig),
		"nil-HeaderDef create must wrap blerrors.ErrConfig (system/config fault → MCP internal); got: %v", err)
	assert.False(t, errors.Is(err, blerrors.ErrValidation),
		"nil-HeaderDef create must NOT wrap blerrors.ErrValidation (would mis-map to validation_failed/422); got: %v", err)

	// AC2: no artifact file was persisted — the write failed closed before persist.
	after := countArtifactFiles(t, ws)
	assert.Equal(t, before, after, "nil-HeaderDef create must persist no artifact file")
}

// TestUpdateArtifact_NilHeaderDefFailsClosed proves that UpdateArtifact fails
// closed (ErrConfig, not ErrValidation) when the workspace header-def schema is
// absent, persisting no mutation — while the loaded-schema update still succeeds.
// Covers AC1, AC2, AC3, AC4 for the update write path.
func TestUpdateArtifact_NilHeaderDefFailsClosed(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Create the update target with the loaded HeaderDef.
	feat, err := core.CreateArtifact(ctx, ws, "Update target", "feature")
	require.NoError(t, err)
	require.NotNil(t, feat)

	// Loaded-HeaderDef regression guard: the normal update path still succeeds
	// and persists.
	_, err = core.UpdateArtifact(ctx, ws, feat.ID, map[string]any{"title": "Loaded rename"})
	require.NoError(t, err)

	path := findArtifactFile(t, ws, feat.ID)
	require.NotEmpty(t, path, "artifact file must exist after loaded update")
	baseline, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(baseline), "Loaded rename", "loaded update must persist the new title")

	// Force the defensive branch: absent workspace schema.
	ws.HeaderDef = nil

	got, err := core.UpdateArtifact(ctx, ws, feat.ID, map[string]any{"title": "Renamed"})

	// AC1: fail closed with a non-nil error wrapping ErrConfig, not ErrValidation.
	require.Error(t, err)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, blerrors.ErrConfig),
		"nil-HeaderDef update must wrap blerrors.ErrConfig (system/config fault → MCP internal); got: %v", err)
	assert.False(t, errors.Is(err, blerrors.ErrValidation),
		"nil-HeaderDef update must NOT wrap blerrors.ErrValidation (would mis-map to validation_failed/422); got: %v", err)

	// AC2: the on-disk artifact is unchanged — the mutation failed closed before persist.
	afterBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(baseline), string(afterBytes),
		"nil-HeaderDef update must not persist any mutation")
	assert.NotContains(t, string(afterBytes), "Renamed",
		"nil-HeaderDef update must not persist the attempted rename")
}

// TestU1_ShipmentWorkflowIDHeaderDefAndLegacyUpgrade pins the U1 contract: the
// shipment type's optional workflow_id field ships in the default header-def,
// legacy generated header-defs upgrade to include it, operator-customized
// header-defs are left alone, and the SQLite index projects it into
// items.workflow_id.
func TestU1_ShipmentWorkflowIDHeaderDefAndLegacyUpgrade(t *testing.T) {
	t.Run("default_header_def_includes_shipment_workflow_id_and_branch", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, config.WriteDefaults(dir))
		cfg, err := config.LoadHeaderDef(dir)
		require.NoError(t, err)

		fields := cfg.Types["shipment"].Fields
		requireOptionalStringField(t, fields, "branch")
		requireOptionalStringField(t, fields, "workflow_id")
	})

	t.Run("legacy_generated_header_def_upgrades_all_bitmask_variants", func(t *testing.T) {
		// Bits: 1 drops task.complexity and its provenance fields, 2 drops only
		// the provenance fields, 4 drops shipment status "blocked", and 8 drops
		// shipment.workflow_id. Every non-zero combination is a prior variant.
		for mask := 1; mask < 16; mask++ {
			t.Run(fmt.Sprintf("mask_%02d", mask), func(t *testing.T) {
				dir := t.TempDir()
				writeHeaderDef(t, dir, legacyVariant(t, mask))

				upgraded, err := config.LoadHeaderDef(dir)
				require.NoError(t, err)
				requireOptionalStringField(t, upgraded.Types["shipment"].Fields, "workflow_id")
			})
		}

		t.Run("operator_customized_header_def_is_untouched", func(t *testing.T) {
			dir := t.TempDir()
			customized := loadTargetHeaderDef(t)
			delete(customized.Types["shipment"].Fields, "workflow_id")
			customized.Types["shipment"].Fields["owner_team"] = &config.FieldDef{Type: "string", Optional: true}
			writeHeaderDef(t, dir, customized)

			loaded, err := config.LoadHeaderDef(dir)
			require.NoError(t, err)
			fields := loaded.Types["shipment"].Fields
			assert.NotContains(t, fields, "workflow_id", "operator-customized header-def.yaml must not gain workflow_id")
			assert.Contains(t, fields, "owner_team", "operator-customized header-def.yaml must keep its custom field")
		})
	})

	t.Run("sqlite_schema_extensions_project_workflow_id", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, config.WriteDefaults(dir))
		cfg, err := config.LoadHeaderDef(dir)
		require.NoError(t, err)

		conn, err := db.Open(filepath.Join(dir, "index.db"))
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, conn.Close()) })
		require.NoError(t, db.EnsureSchema(conn))

		stmts, err := db.GenerateSchemaExtensions(conn, cfg)
		require.NoError(t, err)
		assert.Contains(t, strings.Join(stmts, "\n"), `ALTER TABLE items ADD COLUMN "workflow_id" TEXT`,
			"default header-def must generate a workflow_id TEXT column on items")

		require.NoError(t, db.ApplySchemaExtensions(conn, cfg))
		require.NoError(t, db.ApplySchemaExtensions(conn, cfg), "schema extensions must be idempotent")
		require.True(t, itemsHasColumn(t, conn, "workflow_id"), "items table must have a workflow_id column")

		artifact := &models.Artifact{
			ID:           "211-S",
			Title:        "Workflow shipment",
			Status:       "queued",
			ArtifactType: "shipment",
			CustomFields: map[string]any{"workflow_id": "wf-211-s"},
		}
		require.NoError(t, db.UpsertItem(context.Background(), conn, artifact))

		var got sql.NullString
		require.NoError(t, conn.QueryRow(`SELECT workflow_id FROM items WHERE id = ?`, artifact.ID).Scan(&got))
		assert.Equal(t, "wf-211-s", got.String, "UpsertItem must project custom_fields.workflow_id into items.workflow_id")
	})
}

// requireOptionalStringField asserts that a shipment field is declared as an
// optional string in the header-def.
func requireOptionalStringField(t *testing.T, fields map[string]*config.FieldDef, name string) {
	t.Helper()
	field, ok := fields[name]
	require.True(t, ok, "shipment.%s field is not declared in header-def.yaml", name)
	require.NotNil(t, field, "shipment.%s field must not be null", name)
	assert.Equal(t, "string", field.Type, "shipment.%s must be type string", name)
	assert.True(t, field.Optional, "shipment.%s must be optional", name)
}

// loadTargetHeaderDef returns the shipped defaults with shipment.workflow_id
// declared, used as the base for constructing legacy and customized variants.
func loadTargetHeaderDef(t *testing.T) *config.HeaderDefConfig {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)
	cfg.Types["shipment"].Fields["workflow_id"] = &config.FieldDef{Type: "string", Optional: true}
	return cfg
}

// legacyVariant builds the prior generated header-def selected by the bitmask
// described in TestU1_ShipmentWorkflowIDHeaderDefAndLegacyUpgrade.
func legacyVariant(t *testing.T, mask int) *config.HeaderDefConfig {
	t.Helper()
	cfg := loadTargetHeaderDef(t)
	task := cfg.Types["task"].Fields
	if mask&1 != 0 {
		delete(task, "complexity")
	}
	if mask&1 != 0 || mask&2 != 0 {
		delete(task, "complexity_source")
		delete(task, "complexity_ruleset_version")
	}
	if mask&4 != 0 {
		status := cfg.Types["shipment"].Fields["status"]
		kept := make([]string, 0, len(status.Values))
		for _, v := range status.Values {
			if v != "blocked" {
				kept = append(kept, v)
			}
		}
		status.Values = kept
	}
	if mask&8 != 0 {
		delete(cfg.Types["shipment"].Fields, "workflow_id")
	}
	return cfg
}

// writeHeaderDef serializes cfg as header-def.yaml inside dir.
func writeHeaderDef(t *testing.T, dir string, cfg *config.HeaderDefConfig) {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "header-def.yaml"), data, 0o600))
}

// itemsHasColumn reports whether the items table declares the named column.
func itemsHasColumn(t *testing.T, conn *sql.DB, column string) bool {
	t.Helper()
	var name string
	err := conn.QueryRow(`SELECT name FROM pragma_table_info('items') WHERE name = ?`, column).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	require.NoError(t, err)
	return name == column
}
