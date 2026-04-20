package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTerminal_NonFileWriterReturnsFalse(t *testing.T) {
	assert.False(t, isTerminal(&bytes.Buffer{}), "bytes.Buffer should not be detected as a terminal")
}
