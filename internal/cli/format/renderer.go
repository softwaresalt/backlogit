// Package format provides pluggable CLI output renderers for backlogit commands.
// Callers select a renderer by Format value and write structured row data via the
// Renderer interface. All renderers write to an io.Writer and return an error.
package format

import "io"

// Format specifies the output format selected by the --format flag.
type Format string

const (
	// FormatTable renders output as a tab-separated table with a header row.
	FormatTable Format = "table"
	// FormatJSON renders output as a compact JSON array where each element
	// contains only the columns requested.
	FormatJSON Format = "json"
	// FormatTile renders output as blank-line-separated property-value blocks.
	FormatTile Format = "tile"
)

// Column describes a single output column.
type Column struct {
	// Key is the map key used to look up the value from a row.
	Key string
	// Header is the column heading shown in table and tile output.
	Header string
}

// Renderer writes structured row data to an io.Writer in a specific format.
type Renderer interface {
	Render(w io.Writer, columns []Column, rows []map[string]any) error
}
