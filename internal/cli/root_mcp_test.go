package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenMCPServer_AllowsUninitializedWorkspace(t *testing.T) {
	// Arrange
	root := t.TempDir()

	// Act
	server, err := openMCPServer(context.Background(), root)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, server)
	assert.Contains(t, server.ListTools(), "backlogit_list_items")
}
