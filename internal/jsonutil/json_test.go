package jsonutil_test

import (
	"strings"
	"testing"

	"github.com/softwaresalt/backlogit/internal/jsonutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarshalReadable_NoHTMLEscape(t *testing.T) {
	type payload struct {
		URL     string `json:"url"`
		Expr    string `json:"expr"`
		Ampersand string `json:"ampersand"`
	}

	v := payload{
		URL:       "https://example.com/a?x=1&y=2",
		Expr:      "a > b && b < c",
		Ampersand: "cats & dogs",
	}

	got, err := jsonutil.MarshalReadable(v)
	require.NoError(t, err)
	s := string(got)

	assert.Contains(t, s, `>`, "greater-than must not be Unicode-escaped")
	assert.Contains(t, s, `<`, "less-than must not be Unicode-escaped")
	assert.Contains(t, s, `&`, "ampersand must not be Unicode-escaped")

	assert.NotContains(t, s, `\u003e`, "\\u003e escape must not appear")
	assert.NotContains(t, s, `\u003c`, "\\u003c escape must not appear")
	assert.NotContains(t, s, `\u0026`, "\\u0026 escape must not appear")
}

func TestMarshalReadable_ValidJSON(t *testing.T) {
	v := map[string]string{"key": "val<ue>"}
	got, err := jsonutil.MarshalReadable(v)
	require.NoError(t, err)

	// Must be valid JSON (no trailing newline from Encoder).
	assert.False(t, strings.HasSuffix(string(got), "\n"), "trailing newline must be stripped")
}

func TestMarshalReadableIndent_NoHTMLEscape(t *testing.T) {
	v := map[string]string{"link": "a&b>c<d"}
	got, err := jsonutil.MarshalReadableIndent(v, "", "  ")
	require.NoError(t, err)
	s := string(got)

	assert.Contains(t, s, `&`)
	assert.Contains(t, s, `>`)
	assert.Contains(t, s, `<`)
	assert.NotContains(t, s, `\u0026`)
	assert.NotContains(t, s, `\u003e`)
	assert.NotContains(t, s, `\u003c`)
}
