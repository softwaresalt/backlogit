package telemetry_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/telemetry"
)

func TestDescribeFactTables_ReturnsAllTables(t *testing.T) {
	tables := telemetry.DescribeFactTables()
	require.Len(t, tables, 4)

	names := make(map[string]bool, len(tables))
	for _, tbl := range tables {
		names[tbl.Name] = true
	}
	assert.True(t, names["session_summary"])
	assert.True(t, names["tool_usage"])
	assert.True(t, names["tool_call_fact"])
	assert.True(t, names["session_fact"])
}

func TestDescribeTelemetrySQLTables_ReturnsAllTables(t *testing.T) {
	tables := telemetry.DescribeTelemetrySQLTables()
	require.Len(t, tables, 2)

	names := make(map[string]bool, len(tables))
	for _, tbl := range tables {
		names[tbl.Name] = true
	}
	assert.True(t, names["telemetry_sessions"])
	assert.True(t, names["telemetry_tool_usage"])
}

// TestSessionSummaryFieldsDriftDetection ensures the schema reference stays in
// sync with the SessionSummaryRecord struct. If a field is added or removed
// from the struct, this test fails as a reminder to update schema_ref.go.
func TestSessionSummaryFieldsDriftDetection(t *testing.T) {
	structFields := exportedFieldCount(reflect.TypeOf(telemetry.SessionSummaryRecord{}))
	tables := telemetry.DescribeFactTables()

	var schemaFields int
	for _, tbl := range tables {
		if tbl.Name == "session_summary" {
			schemaFields = len(tbl.Fields)
			break
		}
	}
	assert.Equal(t, structFields, schemaFields,
		"SessionSummaryRecord has %d exported fields but schema_ref declares %d; update schema_ref.go",
		structFields, schemaFields)
}

// TestToolCallFactFieldsDriftDetection ensures ToolCallFact and its schema reference match.
func TestToolCallFactFieldsDriftDetection(t *testing.T) {
	structFields := exportedFieldCount(reflect.TypeOf(telemetry.ToolCallFact{}))
	tables := telemetry.DescribeFactTables()

	var schemaFields int
	for _, tbl := range tables {
		if tbl.Name == "tool_call_fact" {
			schemaFields = len(tbl.Fields)
			break
		}
	}
	assert.Equal(t, structFields, schemaFields,
		"ToolCallFact has %d exported fields but schema_ref declares %d; update schema_ref.go",
		structFields, schemaFields)
}

// TestSessionFactFieldsDriftDetection ensures SessionFact and its schema reference match.
func TestSessionFactFieldsDriftDetection(t *testing.T) {
	structFields := exportedFieldCount(reflect.TypeOf(telemetry.SessionFact{}))
	tables := telemetry.DescribeFactTables()

	var schemaFields int
	for _, tbl := range tables {
		if tbl.Name == "session_fact" {
			schemaFields = len(tbl.Fields)
			break
		}
	}
	assert.Equal(t, structFields, schemaFields,
		"SessionFact has %d exported fields but schema_ref declares %d; update schema_ref.go",
		structFields, schemaFields)
}

// TestToolUsageFieldsDriftDetection ensures ToolUsageRecord and its schema reference match.
func TestToolUsageFieldsDriftDetection(t *testing.T) {
	structFields := exportedFieldCount(reflect.TypeOf(telemetry.ToolUsageRecord{}))
	tables := telemetry.DescribeFactTables()

	var schemaFields int
	for _, tbl := range tables {
		if tbl.Name == "tool_usage" {
			schemaFields = len(tbl.Fields)
			break
		}
	}
	assert.Equal(t, structFields, schemaFields,
		"ToolUsageRecord has %d exported fields but schema_ref declares %d; update schema_ref.go",
		structFields, schemaFields)
}

// exportedFieldCount returns the number of exported fields on a struct type.
func exportedFieldCount(t reflect.Type) int {
	count := 0
	for i := 0; i < t.NumField(); i++ {
		if t.Field(i).IsExported() {
			count++
		}
	}
	return count
}
