package format

import (
	"encoding/json"
	"fmt"
	"io"
)

// JSONRenderer renders rows as a JSON array. Each element contains only the
// keys present in the supplied columns slice.
type JSONRenderer struct{}

// NewJSONRenderer returns a ready-to-use JSONRenderer.
func NewJSONRenderer() *JSONRenderer {
	return &JSONRenderer{}
}

// Render writes rows as a JSON array to w. Each array element is an object
// whose keys are the Column.Key values in the supplied columns slice.
// An empty rows slice produces "[]".
func (j *JSONRenderer) Render(w io.Writer, columns []Column, rows []map[string]any) error {
	// Build a slice of ordered maps restricted to the declared columns.
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		elem := make(map[string]any, len(columns))
		for _, c := range columns {
			elem[c.Key] = row[c.Key]
		}
		out = append(out, elem)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("json render: %w", err)
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}
