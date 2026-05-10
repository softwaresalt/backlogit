package telemetry_test

import (
	"reflect"
	"strings"
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
// sync with the SessionSummaryRecord struct. Validates field count, JSON tag
// names, and omitempty/Optional alignment.
func TestSessionSummaryFieldsDriftDetection(t *testing.T) {
	assertFieldsMatchStruct(t, "session_summary",
		reflect.TypeOf(telemetry.SessionSummaryRecord{}),
		telemetry.DescribeFactTables())
}

// TestToolCallFactFieldsDriftDetection ensures ToolCallFact and its schema reference match.
func TestToolCallFactFieldsDriftDetection(t *testing.T) {
	assertFieldsMatchStruct(t, "tool_call_fact",
		reflect.TypeOf(telemetry.ToolCallFact{}),
		telemetry.DescribeFactTables())
}

// TestSessionFactFieldsDriftDetection ensures SessionFact and its schema reference match.
func TestSessionFactFieldsDriftDetection(t *testing.T) {
	assertFieldsMatchStruct(t, "session_fact",
		reflect.TypeOf(telemetry.SessionFact{}),
		telemetry.DescribeFactTables())
}

// TestToolUsageFieldsDriftDetection ensures ToolUsageRecord and its schema reference match.
func TestToolUsageFieldsDriftDetection(t *testing.T) {
	assertFieldsMatchStruct(t, "tool_usage",
		reflect.TypeOf(telemetry.ToolUsageRecord{}),
		telemetry.DescribeFactTables())
}

// assertFieldsMatchStruct validates that a schema table's fields match the
// exported struct fields by count, JSON key, and Optional/omitempty alignment.
func assertFieldsMatchStruct(t *testing.T, tableName string, structType reflect.Type, tables []telemetry.FactTableSchema) {
	t.Helper()

	var table *telemetry.FactTableSchema
	for i := range tables {
		if tables[i].Name == tableName {
			table = &tables[i]
			break
		}
	}
	require.NotNilf(t, table, "table %q not found in DescribeFactTables()", tableName)

	structFields := exportedFieldCount(structType)
	assert.Equal(t, structFields, len(table.Fields),
		"%s has %d exported fields but schema_ref declares %d; update schema_ref.go",
		structType.Name(), structFields, len(table.Fields))

	// Build a map of struct field JSON keys to their omitempty status.
	structFieldMap := make(map[string]bool) // json_key -> has omitempty
	for i := 0; i < structType.NumField(); i++ {
		f := structType.Field(i)
		if !f.IsExported() {
			continue
		}
		jsonTag := f.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}
		parts := strings.Split(jsonTag, ",")
		key := parts[0]
		hasOmitempty := false
		for _, part := range parts[1:] {
			if part == "omitempty" {
				hasOmitempty = true
			}
		}
		structFieldMap[key] = hasOmitempty
	}

	// Validate each schema field matches the struct.
	for _, sf := range table.Fields {
		hasOmitempty, found := structFieldMap[sf.JSONKey]
		assert.Truef(t, found,
			"schema field %q (json_key=%q) has no matching struct field in %s",
			sf.Name, sf.JSONKey, structType.Name())
		if found {
			assert.Equalf(t, hasOmitempty, sf.Optional,
				"field %q: struct omitempty=%v but schema Optional=%v",
				sf.JSONKey, hasOmitempty, sf.Optional)
		}
	}
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
