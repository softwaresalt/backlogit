package evidencedag

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxJSONDepth = 16

// Load decodes, bounds-checks and validates a DAG declaration. The document
// may be JSON (when it starts with '{') or YAML. Decoding is strict: unknown
// fields, duplicate keys, multiple documents and trailing data are refused,
// and the schema version must match SchemaVersion exactly.
func Load(data []byte) (*Declaration, error) {
	if len(data) > MaxDeclarationBytes {
		return nil, fmt.Errorf("%w: declaration exceeds %d bytes", ErrInvalidDeclaration, MaxDeclarationBytes)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: declaration is empty", ErrInvalidDeclaration)
	}
	var d Declaration
	var err error
	if trimmed[0] == '{' {
		err = decodeJSON(trimmed, &d)
	} else {
		err = decodeYAML(trimmed, &d)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalidDeclaration, truncate(err.Error()))
	}
	if d.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("%w: got %d, want %d", ErrUnsupportedVersion, d.SchemaVersion, SchemaVersion)
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

const (
	maxYAMLDepth = 32
	maxYAMLNodes = 50000
)

// rejectYAMLIndirection refuses anchors, aliases and merge keys (which let a
// small document expand or smuggle keys past duplicate checks) and bounds
// depth and node count.
func rejectYAMLIndirection(n *yaml.Node, depth int, count *int) error {
	*count++
	if *count > maxYAMLNodes {
		return errors.New("YAML document has too many nodes")
	}
	if depth > maxYAMLDepth {
		return errors.New("YAML nesting too deep")
	}
	if n.Kind == yaml.AliasNode || n.Alias != nil {
		return errors.New("YAML aliases are not allowed")
	}
	if n.Anchor != "" {
		return errors.New("YAML anchors are not allowed")
	}
	if n.Kind == yaml.MappingNode {
		seen := map[string]struct{}{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Tag == "!!merge" || (k.Kind == yaml.ScalarNode && k.Value == "<<") {
				return errors.New("YAML merge keys are not allowed")
			}
			if k.Kind == yaml.ScalarNode {
				folded := strings.ToLower(k.Value)
				if _, dup := seen[folded]; dup {
					return fmt.Errorf("duplicate key %q", k.Value)
				}
				seen[folded] = struct{}{}
			}
		}
	}
	for _, c := range n.Content {
		if err := rejectYAMLIndirection(c, depth+1, count); err != nil {
			return err
		}
	}
	return nil
}

func decodeYAML(data []byte, d *Declaration) error {
	var root yaml.Node
	pre := yaml.NewDecoder(bytes.NewReader(data))
	if err := pre.Decode(&root); err != nil {
		return err
	}
	count := 0
	if err := rejectYAMLIndirection(&root, 0, &count); err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(d); err != nil {
		return err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("multiple YAML documents or trailing data")
	}
	return nil
}

func decodeJSON(data []byte, d *Declaration) error {
	if err := rejectDuplicateKeys(data); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(d); err != nil {
		return err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return errors.New("trailing data after JSON document")
	}
	return nil
}

// rejectDuplicateKeys walks the JSON token stream and refuses repeated object
// keys (encoding/json would silently keep the last one) and excessive depth.
// Keys are compared case-insensitively, because encoding/json matches struct
// fields that way and a differently-cased duplicate would otherwise be a
// parser differential.
func rejectDuplicateKeys(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	type frame struct {
		keys   map[string]struct{}
		isObj  bool
		expect bool // next token in an object is a key
	}
	var stack []frame
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.isObj && top.expect {
				if k, ok := tok.(string); ok {
					folded := strings.ToLower(k)
					if _, dup := top.keys[folded]; dup {
						return fmt.Errorf("duplicate key %q", k)
					}
					top.keys[folded] = struct{}{}
					top.expect = false
					continue
				}
			}
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{', '[':
				if len(stack) >= maxJSONDepth {
					return errors.New("JSON nesting too deep")
				}
				stack = append(stack, frame{keys: map[string]struct{}{}, isObj: v == '{', expect: v == '{'})
			default:
				stack = stack[:len(stack)-1]
				if len(stack) > 0 && stack[len(stack)-1].isObj {
					stack[len(stack)-1].expect = true
				}
			}
			continue
		}
		if len(stack) > 0 && stack[len(stack)-1].isObj {
			stack[len(stack)-1].expect = true
		}
	}
}

func joinSorted(parts []string) string { return strings.Join(parts, "; ") }
