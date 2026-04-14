package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/cli"
)

func TestFormatTreeView_RendersHierarchy(t *testing.T) {
	// Arrange
	items := []cli.ListItem{
		{ID: "T001", Title: "Parent", Status: "active", Type: "task", Depth: 0},
		{ID: "T002", Title: "Child A", Status: "done", Type: "task", ParentID: "T001", Depth: 1},
		{ID: "T003", Title: "Child B", Status: "queued", Type: "task", ParentID: "T001", Depth: 1},
	}

	// Act
	output := cli.FormatTreeView(items)

	// Assert
	assert.Contains(t, output, "T001")
	assert.Contains(t, output, "T002")
	assert.Contains(t, output, "T003")
}

func TestFormatGroupedView_GroupsByType(t *testing.T) {
	// Arrange
	items := []cli.ListItem{
		{ID: "T001", Title: "Task 1", Status: "queued", Type: "task"},
		{ID: "B001", Title: "Bug 1", Status: "active", Type: "bug"},
		{ID: "T002", Title: "Task 2", Status: "done", Type: "task"},
	}

	// Act
	output := cli.FormatGroupedView(items, "type")

	// Assert
	assert.Contains(t, output, "task")
	assert.Contains(t, output, "bug")
}
