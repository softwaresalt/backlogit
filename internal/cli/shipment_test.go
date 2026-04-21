package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// T003 / ST016: Verify NewShipmentCmd returns a valid command with subcommands.
func TestNewShipmentCmd_HasSubcommands(t *testing.T) {
	// Arrange & Act
	cmd := NewShipmentCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.Equal(t, "shipment", cmd.Use)

	names := make([]string, 0)
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "create")
	assert.Contains(t, names, "get")
	assert.Contains(t, names, "list")
	assert.Contains(t, names, "claim")
	assert.Contains(t, names, "ship")
	assert.Contains(t, names, "return-blocked")
}

// T003 / ST016: Verify shipment create has required flags.
func TestShipmentCreateCmd_HasFlags(t *testing.T) {
	// Arrange & Act
	cmd := newShipmentCreateCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("title"), "create must have --title flag")
}

// T003 / ST016: Verify shipment return-blocked has required flags.
func TestShipmentReturnBlockedCmd_HasFlags(t *testing.T) {
	// Arrange & Act
	cmd := newShipmentReturnBlockedCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("shipment"), "return-blocked must have --shipment flag")
	assert.NotNil(t, cmd.Flags().Lookup("item"), "return-blocked must have --item flag")
	assert.NotNil(t, cmd.Flags().Lookup("reason"), "return-blocked must have --reason flag")
}

// T003 / ST016: Verify shipment ship has commit-tracking flags.
func TestShipmentShipCmd_HasFlags(t *testing.T) {
	// Arrange & Act
	cmd := newShipmentShipCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("sha"), "ship must have --sha flag")
	assert.NotNil(t, cmd.Flags().Lookup("message"), "ship must have --message flag")
	assert.NotNil(t, cmd.Flags().Lookup("author"), "ship must have --author flag")
}

// T003 / ST016: Verify shipment commands have Short and Example populated.
func TestShipmentSubcommands_HaveDocumentation(t *testing.T) {
	// Arrange
	cmd := NewShipmentCmd()

	// Assert
	for _, sub := range cmd.Commands() {
		t.Run(sub.Name(), func(t *testing.T) {
			assert.NotEmpty(t, sub.Short, "%s must have .Short description", sub.Name())
			assert.NotEmpty(t, sub.Example, "%s must have .Example", sub.Name())
		})
	}
}

// T003 / ST016: Verify shipment list has --status filter flag.
func TestShipmentListCmd_HasStatusFilter(t *testing.T) {
	// Arrange & Act
	cmd := newShipmentListCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("status"), "list must have --status flag")
}

// T003 / ST016: Verify help output includes shipment subcommands.
func TestShipmentCmd_HelpOutput(t *testing.T) {
	// Arrange
	cmd := NewShipmentCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	// Act
	_ = cmd.Execute()

	// Assert
	output := buf.String()
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "ship")
	assert.Contains(t, output, "return-blocked")
}

func TestShipmentListCmd_RejectsInvalidFormat(t *testing.T) {
	root := setupShipmentCLIWorkspace(t)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "shipment", "list", "--format", "banana"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"banana"`)
}

func setupShipmentCLIWorkspace(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	return root
}
