package parser_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/parser"
)

func TestParseLegacy_Headings(t *testing.T) {
	// Arrange
	content := "# Backlog\n\n## In Progress\n\n- [ ] Task One\n- [x] Task Two\n\n## Done\n\n- [x] Task Three\n"

	// Act
	items, err := parser.ParseLegacy(content)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, items)
}

func TestParseLegacy_Checklists(t *testing.T) {
	// Arrange
	content := "- [ ] Open task\n- [x] Done task\n"

	// Act
	items, err := parser.ParseLegacy(content)

	// Assert
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 2)
}
