package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
)

func TestResolveName_TaskFormat(t *testing.T) {
	// Arrange
	cfg := &config.ArtifactTypeConfig{
		Prefix:     "T",
		NameFormat: "{prefix}{NNN}-{title_slug}",
	}

	// Act
	name := core.ResolveName(cfg, "Implement JWT", 1, 60)

	// Assert
	assert.Equal(t, "T001-implement-jwt", name)
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
		NameFormat: "{prefix}{NNN}",
	}

	name := core.ResolveFileName(cfg, "T001", "Implement JWT", 60)

	assert.Equal(t, "T001", name)
}

func TestResolveFileName_UsesConfiguredFormat(t *testing.T) {
	cfg := &config.ArtifactTypeConfig{
		Prefix:         "R",
		NameFormat:     "{prefix}{NNN}",
		FileNameFormat: "{id}-{title_slug}",
	}

	name := core.ResolveFileName(cfg, "F013.R001", "Followup Review", 60)

	assert.Equal(t, "F013.R001-followup-review", name)
}
