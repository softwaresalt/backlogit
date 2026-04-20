package format

import (
	"fmt"
	"io"
)

const (
	ansiBold  = "\x1b[1m"
	ansiReset = "\x1b[0m"
)

// TileRenderer renders rows as blank-line-separated property-value blocks.
// Each block contains one "Header: value" line per column.
// Bold controls whether ANSI bold escape sequences are applied to the first
// property line of each block. Callers set Bold based on TTY detection;
// the renderer does not auto-detect terminal capabilities.
type TileRenderer struct {
	Bold bool
}

// NewTileRenderer returns a TileRenderer with the given bold setting.
func NewTileRenderer(bold bool) *TileRenderer {
	return &TileRenderer{Bold: bold}
}

// Render writes each row as a block of "Header: value" lines, separated by
// blank lines between blocks (but not after the final block).
// When Bold is true the first property line of each block is wrapped in ANSI
// bold escape sequences.
func (t *TileRenderer) Render(w io.Writer, columns []Column, rows []map[string]any) error {
	for blockIdx, row := range rows {
		if blockIdx > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return fmt.Errorf("tile render separator: %w", err)
			}
		}
		for lineIdx, c := range columns {
			val := ""
			if v, ok := row[c.Key]; ok {
				val = fmt.Sprintf("%v", v)
			}
			line := fmt.Sprintf("%s: %s", c.Header, val)
			if t.Bold && lineIdx == 0 {
				line = ansiBold + line + ansiReset
			}
			if _, err := fmt.Fprintln(w, line); err != nil {
				return fmt.Errorf("tile render line: %w", err)
			}
		}
	}
	return nil
}
