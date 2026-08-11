// Package jsonutil provides JSON marshaling helpers with sane defaults for
// human-readable output.
package jsonutil

import (
	"bytes"
	"encoding/json"
	"io"
)

// MarshalReadable serializes v to JSON without HTML-escaping special characters
// (<, >, &). Go's standard json.Marshal escapes these by default, making JSON
// files harder to read when they contain URLs, HTML, or comparison operators.
func MarshalReadable(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// json.Encoder always appends a trailing newline; trim it so the result
	// matches the byte layout produced by json.Marshal.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// MarshalReadableIndent serializes v to indented JSON without HTML-escaping.
// prefix and indent are applied as in json.MarshalIndent.
func MarshalReadableIndent(v any, prefix, indent string) ([]byte, error) {
	raw, err := MarshalReadable(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, prefix, indent); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NewEncoder returns a *json.Encoder that writes to w with HTML escaping
// disabled. Callers may still call SetIndent on the returned encoder.
func NewEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}
