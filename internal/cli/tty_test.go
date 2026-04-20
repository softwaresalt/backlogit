package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsTerminal_NonFileWriterReturnsFalse(t *testing.T) {
	assert.False(t, isTerminal(&bytes.Buffer{}), "bytes.Buffer should not be detected as a terminal")
}

func TestIsTerminal_PipeReturnsFalse(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()
	defer w.Close()
	assert.False(t, isTerminal(r), "pipe read-end should not be detected as a terminal")
	assert.False(t, isTerminal(w), "pipe write-end should not be detected as a terminal")
}
