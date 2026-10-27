package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"unicode"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

type jsonScanFrame struct {
	object    bool
	expectKey bool
	seen      map[string]struct{}
}

// rejectDuplicateJSONMembers walks data once, token by token, and fails with
// blerrors.ErrValidation when any object at any depth (including objects
// inside arrays) repeats a member name, when the document is malformed, or
// when data follows the root value. Member names are compared by the
// unicode.SimpleFold orbit minimum of each rune, the same equivalence
// encoding/json uses to bind keys to struct fields, so case-only variants and
// Unicode aliases (U+017F, U+212A) are detected whether written literally or
// as \u escapes (the decoder unescapes before this comparison).
func rejectDuplicateJSONMembers(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var stack []jsonScanFrame
	rootDone := false
	for !rootDone {
		token, err := decoder.Token()
		if err != nil {
			return fmt.Errorf("%w: scan JSON members: %w", blerrors.ErrValidation, err)
		}
		if delim, ok := token.(json.Delim); ok {
			switch delim {
			case '{':
				stack = append(stack, jsonScanFrame{object: true, expectKey: true, seen: make(map[string]struct{})})
			case '[':
				stack = append(stack, jsonScanFrame{})
			default:
				stack = stack[:len(stack)-1]
				rootDone = finishJSONValue(stack)
			}
			continue
		}
		if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].expectKey {
			key, _ := token.(string)
			canonical := canonicalJSONMemberName(key)
			if _, dup := stack[n-1].seen[canonical]; dup {
				return fmt.Errorf("%w: duplicate JSON member name %q", blerrors.ErrValidation, key)
			}
			stack[n-1].seen[canonical] = struct{}{}
			stack[n-1].expectKey = false
			continue
		}
		rootDone = finishJSONValue(stack)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: unexpected data after the JSON root value", blerrors.ErrValidation)
	}
	return nil
}

// finishJSONValue marks a completed value in the enclosing frame and reports
// whether that value was the root.
func finishJSONValue(stack []jsonScanFrame) bool {
	if len(stack) == 0 {
		return true
	}
	if top := &stack[len(stack)-1]; top.object {
		top.expectKey = true
	}
	return false
}

func canonicalJSONMemberName(name string) string {
	runes := []rune(name)
	for i, r := range runes {
		smallest := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < smallest {
				smallest = f
			}
		}
		runes[i] = smallest
	}
	return string(runes)
}
