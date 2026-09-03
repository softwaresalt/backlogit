package events_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemediationCommand_U4_S1_FieldAbsentFromCheckpointSummary(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "checkpoint_schema.go", nil, 0)
	require.NoError(t, err, "checkpoint_schema.go must parse successfully")

	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != "CheckpointSummary" {
			return true
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}
		for _, field := range structType.Fields.List {
			if field.Tag == nil {
				continue
			}
			if strings.Contains(field.Tag.Value, `"remediation_command`) {
				found = true
			}
		}
		return false
	})

	assert.False(t, found,
		"CheckpointSummary must NOT declare a field with json:\"remediation_command\" after U4 removal")
}
