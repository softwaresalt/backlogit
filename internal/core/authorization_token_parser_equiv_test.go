package core_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func TestTokenJSONEquivDuplicateAllDepths(t *testing.T) {
	// Each pair names the same member after decoding: by case folding, by a
	// \uXXXX escape, or by both.
	pairs := []struct{ name, first, second string }{
		{"case-fold", `"nonce"`, `"NONCE"`},
		{"unicode-escape", `"nonce"`, `"no\u006ece"`},
		{"mixed case-fold and escape", `"NONCE"`, `"no\u006ece"`},
	}
	depths := []struct {
		name string
		path string
		data func(a, b string) string
	}{
		{"root", "token", func(a, b string) string {
			return `{` + a + `:"x",` + b + `:"y","header":{},"claims":{},"binding":{},"signature":""}`
		}},
		{"header", "token.header", func(a, b string) string {
			return `{"header":{` + a + `:"x",` + b + `:"y"},"claims":{},"binding":{},"signature":""}`
		}},
		{"claims", "token.claims", func(a, b string) string {
			return `{"header":{},"claims":{` + a + `:"x",` + b + `:"y"},"binding":{},"signature":""}`
		}},
		{"binding", "token.binding", func(a, b string) string {
			return `{"header":{},"claims":{},"binding":{` + a + `:"x",` + b + `:"y"},"signature":""}`
		}},
	}
	for _, pair := range pairs {
		for _, depth := range depths {
			t.Run(pair.name+"/"+depth.name, func(t *testing.T) {
				_, err := core.ParseAuthorizationToken([]byte(depth.data(pair.first, pair.second)))
				require.ErrorIs(t, err, core.ErrTokenDuplicateMember)
				var pe *core.TokenParseError
				require.ErrorAs(t, err, &pe)
				require.Equal(t, depth.path, pe.Path)
			})
		}
	}
}
