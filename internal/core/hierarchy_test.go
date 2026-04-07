package core_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"

	_ "modernc.org/sqlite"
)

func defaultQueueLayout() *core.QueueLayoutConfig {
	return &core.QueueLayoutConfig{
		RootDir:    "queue",
		NameFormat: "{NNN}",
		Levels: []core.HierarchyLevel{
			{Level: 1, Types: []string{"feature"}},
			{Level: 2, Types: []string{"task", "bug", "review"}},
			{Level: 3, Types: []string{"subtask"}},
		},
	}
}

func openHierarchyTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	_, err = db.Exec(`
		CREATE TABLE items (
			id TEXT PRIMARY KEY,
			parent_id TEXT,
			artifact_type TEXT
		);
	`)
	require.NoError(t, err)

	return db
}

func TestParseHierarchicalID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		want    []int
		wantErr bool
	}{
		{name: "single level", id: "001", want: []int{1}},
		{name: "two levels", id: "001.002", want: []int{1, 2}},
		{name: "three levels", id: "001.002.003", want: []int{1, 2, 3}},
		{name: "four levels", id: "001.001.001.001", want: []int{1, 1, 1, 1}},
		{name: "larger numbers", id: "042.007", want: []int{42, 7}},
		{name: "empty string", id: "", wantErr: true},
		{name: "non-numeric", id: "abc.def", wantErr: true},
		{name: "trailing dot", id: "001.", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := core.ParseHierarchicalID(tt.id)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLevelForType(t *testing.T) {
	layout := defaultQueueLayout()

	tests := []struct {
		name         string
		artifactType string
		wantLevel    int
		wantErr      bool
	}{
		{name: "feature is level 1", artifactType: "feature", wantLevel: 1},
		{name: "task is level 2", artifactType: "task", wantLevel: 2},
		{name: "bug is level 2", artifactType: "bug", wantLevel: 2},
		{name: "review is level 2", artifactType: "review", wantLevel: 2},
		{name: "subtask is level 3", artifactType: "subtask", wantLevel: 3},
		{name: "unknown type errors", artifactType: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			level, err := core.LevelForType(layout, tt.artifactType)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantLevel, level)
		})
	}
}

func TestFormatHierarchicalID(t *testing.T) {
	layout := defaultQueueLayout()

	tests := []struct {
		name    string
		segment int
		want    string
	}{
		{name: "single digit", segment: 1, want: "001"},
		{name: "double digit", segment: 42, want: "042"},
		{name: "triple digit", segment: 999, want: "999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := core.FormatHierarchicalID(tt.segment, layout)

			// Assert
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveHierarchicalPath(t *testing.T) {
	layout := defaultQueueLayout()

	tests := []struct {
		name         string
		parentID     string
		artifactType string
		wantContains string
		wantErr      bool
	}{
		{name: "top-level feature", parentID: "", artifactType: "feature", wantContains: "queue"},
		{name: "task under feature", parentID: "001", artifactType: "task", wantContains: "queue"},
		{name: "unmapped type errors", parentID: "001", artifactType: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			path, err := core.ResolveHierarchicalPath(layout, tt.parentID, tt.artifactType)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, path, tt.wantContains)
		})
	}
}

func TestNextTypedHierarchicalID_RootUsesSuffixFormat(t *testing.T) {
	db := openHierarchyTestDB(t)

	typeCfg := &config.ArtifactTypeConfig{
		Prefix:     "F",
		Suffix:     "-F",
		NameFormat: "{NNN}{suffix}",
	}

	got, err := core.NextTypedHierarchicalID(
		context.Background(),
		db,
		"",
		"feature",
		typeCfg,
		defaultQueueLayout(),
	)

	require.NoError(t, err)
	assert.Equal(t, "001-F", got)
}

func TestNextTypedHierarchicalID_ChildUsesSuffixOrdinal(t *testing.T) {
	db := openHierarchyTestDB(t)

	_, err := db.Exec(`
		INSERT INTO items (id, parent_id, artifact_type) VALUES
			('001-F', NULL, 'feature'),
			('001.001-T', '001-F', 'task'),
			('001.002-T', '001-F', 'task');
	`)
	require.NoError(t, err)

	typeCfg := &config.ArtifactTypeConfig{
		Prefix:     "T",
		Suffix:     "-T",
		NameFormat: "{NNN}{suffix}",
	}

	got, err := core.NextTypedHierarchicalID(
		context.Background(),
		db,
		"001-F",
		"task",
		typeCfg,
		defaultQueueLayout(),
	)

	require.NoError(t, err)
	assert.Equal(t, "001.003-T", got)
}

func TestNextHierarchicalID_ChildOrdinalPastNine(t *testing.T) {
	db := openHierarchyTestDB(t)

	_, err := db.Exec(`
		INSERT INTO items (id, parent_id, artifact_type) VALUES
			('001-F', NULL, 'feature'),
			('001.009-T', '001-F', 'task'),
			('001.010-T', '001-F', 'task');
	`)
	require.NoError(t, err)

	got, err := core.NextHierarchicalID(db, "001-F", defaultQueueLayout())

	require.NoError(t, err)
	assert.Equal(t, "001-F.011", got)
}

func TestNextTypedHierarchicalID_LegacyParentErrors(t *testing.T) {
	db := openHierarchyTestDB(t)

	typeCfg := &config.ArtifactTypeConfig{
		Prefix:     "T",
		Suffix:     "-T",
		NameFormat: "{NNN}{suffix}",
	}

	_, err := core.NextTypedHierarchicalID(
		context.Background(),
		db,
		"F016",
		"task",
		typeCfg,
		defaultQueueLayout(),
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `parse hierarchical parent "F016"`)
}

func TestParseHierarchicalID_TypedSegments(t *testing.T) {
	got, err := core.ParseHierarchicalID("001-F.002-T.003-ST")

	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, got)
}
