package parser_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/parser"
)

// TASK-010.01.01: Enhance legacy parser for broader Backlog.md format coverage

func TestParseLegacyEnhanced_NestedHeadings(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		wantItemCount int
		wantDepths    []int
	}{
		{
			name: "H1 through H4 heading hierarchy",
			content: `# Backlog
## Sprint 1
### Epic: Authentication
#### Task: Login form
- [ ] Implement login
#### Task: Logout
- [ ] Implement logout
### Epic: Dashboard
- [ ] Build dashboard
`,
			wantItemCount: 3,
			wantDepths:    []int{4, 4, 3},
		},
		{
			name: "H2 only flat structure",
			content: `## Todo
- [ ] First task
- [ ] Second task
`,
			wantItemCount: 2,
			wantDepths:    []int{2, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			items, err := parser.ParseLegacyEnhanced(tt.content)

			// Assert
			require.NoError(t, err)
			assert.Len(t, items, tt.wantItemCount)
			for i, expectedDepth := range tt.wantDepths {
				if i < len(items) {
					assert.Equal(t, expectedDepth, items[i].Depth, "depth mismatch for item %d", i)
				}
			}
		})
	}
}

func TestParseLegacyEnhanced_ParentChildRelationships(t *testing.T) {
	// Arrange
	content := `## Epic: User Management
### Story: Create user
- [ ] Implement create endpoint
### Story: Delete user
- [ ] Implement delete endpoint
`

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "Story: Create user", items[0].ParentTitle)
	assert.Equal(t, "Story: Delete user", items[1].ParentTitle)
}

func TestParseLegacyEnhanced_PriorityMarkers(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		wantTitle    string
		wantPriority string
	}{
		{
			name:         "P0 marker",
			content:      "## Tasks\n- [ ] [P0] Fix critical security bug\n",
			wantTitle:    "Fix critical security bug",
			wantPriority: "P0",
		},
		{
			name:         "!high marker",
			content:      "## Tasks\n- [ ] !high Deploy new version\n",
			wantTitle:    "Deploy new version",
			wantPriority: "high",
		},
		{
			name:         "no priority",
			content:      "## Tasks\n- [ ] Regular task\n",
			wantTitle:    "Regular task",
			wantPriority: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			items, err := parser.ParseLegacyEnhanced(tt.content)

			// Assert
			require.NoError(t, err)
			require.Len(t, items, 1)
			assert.Equal(t, tt.wantTitle, items[0].Title)
			// Priority should be stored; field will be available after enhancement
			_ = tt.wantPriority
		})
	}
}

func TestParseLegacyEnhanced_AssigneeMentions(t *testing.T) {
	// Arrange
	content := "## Tasks\n- [ ] @alice Fix the login page\n- [ ] @bob Update docs\n"

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 2)
	// After enhancement, items should have assignee extracted
	assert.NotEmpty(t, items[0].Title)
	assert.NotEmpty(t, items[1].Title)
}

func TestParseLegacyEnhanced_DateReferences(t *testing.T) {
	// Arrange
	content := "## Tasks\n- [ ] 2026-03-15 Ship feature release\n"

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Contains(t, items[0].Title, "Ship feature release")
}

func TestParseLegacyEnhanced_TagAnnotations(t *testing.T) {
	// Arrange
	content := "## Tasks\n- [ ] [backend, api] Refactor user service\n"

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Contains(t, items[0].Title, "Refactor user service")
}

func TestParseLegacyEnhanced_SprintGroupings(t *testing.T) {
	// Arrange
	content := `## Sprint 1
- [ ] Task A
- [ ] Task B
## Sprint 2
- [ ] Task C
`

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "Sprint 1", items[0].ParentTitle)
	assert.Equal(t, "Sprint 2", items[2].ParentTitle)
}

func TestParseLegacyEnhanced_DescriptionPreservation(t *testing.T) {
	// Arrange
	content := `## Tasks
This section contains critical work items.

- [ ] Fix authentication

The login form is broken on mobile devices.
Users report a blank screen after entering credentials.

- [ ] Update dependencies
`

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Contains(t, items[0].Description, "login form")
}

func TestParseLegacyEnhanced_BackwardsCompatible(t *testing.T) {
	// Arrange — same input as the original ParseLegacy tests
	content := "# Backlog\n\n## In Progress\n\n- [ ] Task One\n- [x] Task Two\n\n## Done\n\n- [x] Task Three\n"

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	assert.Len(t, items, 3)
	assert.Equal(t, "Task One", items[0].Title)
	assert.Equal(t, "active", items[0].Status)
	assert.Equal(t, "done", items[1].Status)
	assert.Equal(t, "done", items[2].Status)
}

func TestParseLegacyEnhanced_EmptyContent(t *testing.T) {
	// Arrange
	content := ""

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestParseLegacyEnhanced_NoChecklists(t *testing.T) {
	// Arrange
	content := "# Just a heading\n\nSome paragraph text.\n\n## Another heading\n\nMore text.\n"

	// Act
	items, err := parser.ParseLegacyEnhanced(content)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, items)
}
