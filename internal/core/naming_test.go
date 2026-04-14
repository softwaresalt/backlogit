package core_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"

	_ "modernc.org/sqlite"
)

func TestResolveName_TaskFormat(t *testing.T) {
	// Arrange
	cfg := &config.ArtifactTypeConfig{
		Prefix:     "T",
		Suffix:     "-T",
		NameFormat: "{NNN}{suffix}-{title_slug}",
	}

	// Act
	name := core.ResolveName(cfg, "Implement JWT", 1, 60)

	// Assert
	assert.Equal(t, "001-T-implement-jwt", name)
}

func TestSlugify_Basic(t *testing.T) {
	// Act
	slug := core.Slugify("Hello World Test", 60)

	// Assert
	assert.Equal(t, "hello-world-test", slug)
}

func TestSlugify_Truncation(t *testing.T) {
	// Act
	slug := core.Slugify("A Very Long Title That Exceeds The Max Length", 10)

	// Assert
	assert.LessOrEqual(t, len(slug), 10)
}

func TestResolveFileName_DefaultsToArtifactID(t *testing.T) {
	cfg := &config.ArtifactTypeConfig{
		Prefix:     "T",
		Suffix:     "-T",
		NameFormat: "{NNN}{suffix}",
	}

	name := core.ResolveFileName(cfg, "001-T", "Implement JWT", 60)

	assert.Equal(t, "001-T", name)
}

func TestResolveFileName_UsesConfiguredFormat(t *testing.T) {
	cfg := &config.ArtifactTypeConfig{
		Prefix:         "R",
		Suffix:         "-R",
		NameFormat:     "{NNN}{suffix}",
		FileNameFormat: "{id}-{title_slug}",
	}

	name := core.ResolveFileName(cfg, "013.001-R", "Followup Review", 60)

	assert.Equal(t, "013.001-R-followup-review", name)
}

func TestResolveName_SuffixFormat(t *testing.T) {
	cfg := &config.ArtifactTypeConfig{
		Prefix:     "F",
		Suffix:     "-F",
		NameFormat: "{NNN}{suffix}",
	}

	name := core.ResolveName(cfg, "Artifact Identity", 1, 60)

	assert.Equal(t, "001-F", name)
}

func TestNextID_UsesHighestOrdinalInsteadOfCount(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
	})

	_, err = db.Exec(`
		CREATE TABLE items (
			id TEXT PRIMARY KEY,
			artifact_type TEXT
		);
		INSERT INTO items (id, artifact_type) VALUES
			('001-S', 'shipment'),
			('003-S', 'shipment');
	`)
	require.NoError(t, err)

	next, err := core.NextID(context.Background(), db, "shipment", &config.ArtifactTypeConfig{
		Prefix:     "S",
		Suffix:     "-S",
		NameFormat: "{NNN}{suffix}",
	})

	require.NoError(t, err)
	assert.Equal(t, 4, next)
}
