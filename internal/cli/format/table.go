package format

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

// TableRenderer renders rows as a tab-separated table using tabwriter.
// The first row is a header containing column headings.
type TableRenderer struct{}

// NewTableRenderer returns a ready-to-use TableRenderer.
func NewTableRenderer() *TableRenderer {
	return &TableRenderer{}
}

// Render writes a header row followed by data rows to w using tabwriter padding.
// Column order follows the columns slice. Each row occupies exactly one output line.
func (t *TableRenderer) Render(w io.Writer, columns []Column, rows []map[string]any) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	// Header row.
	headers := make([]string, len(columns))
	for i, c := range columns {
		headers[i] = c.Header
	}
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return fmt.Errorf("table render header: %w", err)
	}

	// Data rows.
	for _, row := range rows {
		vals := make([]string, len(columns))
		for i, c := range columns {
			if v, ok := row[c.Key]; ok {
				vals[i] = fmt.Sprintf("%v", v)
			}
		}
		if _, err := fmt.Fprintln(tw, strings.Join(vals, "\t")); err != nil {
			return fmt.Errorf("table render row: %w", err)
		}
	}

	return tw.Flush()
}
