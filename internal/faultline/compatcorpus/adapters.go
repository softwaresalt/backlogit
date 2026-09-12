package compatcorpus

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/softwaresalt/backlogit/internal/mdfront"

	"gopkg.in/yaml.v3"
)

const (
	minimumEventSchemaVersion = 1
	maximumScannerTokenSize   = 256
)

type frontmatterAdapter struct{}

func (frontmatterAdapter) Name() string {
	return "frontmatter"
}

func (frontmatterAdapter) Decode(ctx context.Context, input []byte) (DecodeResult, error) {
	if err := contextError(ctx); err != nil {
		return DecodeResult{}, err
	}

	normalized := bytes.ReplaceAll(input, []byte("\r\n"), []byte("\n"))
	yamlBlock, hasFrontmatter, err := splitFrontmatter(normalized)
	if err != nil {
		return DecodeResult{}, err
	}
	if hasFrontmatter {
		root, err := decodeYAMLNode(yamlBlock)
		if err != nil {
			return DecodeResult{}, err
		}
		if err := validateYAMLNode(root); err != nil {
			return DecodeResult{}, err
		}
	}

	if _, err := mdfront.Decode(normalized); err != nil {
		return DecodeResult{}, classifyYAMLError(err)
	}
	if !bytes.Equal(normalized, input) {
		return DecodeResult{Normalized: normalized}, nil
	}
	return DecodeResult{}, nil
}

type eventsJSONLAdapter struct{}

func (eventsJSONLAdapter) Name() string {
	return "events_jsonl"
}

func (eventsJSONLAdapter) Decode(ctx context.Context, input []byte) (DecodeResult, error) {
	if err := contextError(ctx); err != nil {
		return DecodeResult{}, err
	}
	if err := validateJSONKeys(input); err != nil {
		return DecodeResult{}, err
	}

	var envelope struct {
		SchemaVersion *int `json:"schema_version"`
	}
	if err := json.Unmarshal(input, &envelope); err != nil {
		return DecodeResult{}, classifyJSONError(err)
	}
	if envelope.SchemaVersion != nil && *envelope.SchemaVersion < minimumEventSchemaVersion {
		return DecodeResult{}, fmt.Errorf(
			"decode event schema version %d: %w",
			*envelope.SchemaVersion,
			ErrOldVersion,
		)
	}
	return DecodeResult{}, nil
}

type scannerAdapter struct{}

func (scannerAdapter) Name() string {
	return "scanner"
}

func (scannerAdapter) Decode(ctx context.Context, input []byte) (DecodeResult, error) {
	if err := contextError(ctx); err != nil {
		return DecodeResult{}, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(input))
	scanner.Buffer(make([]byte, 64), maximumScannerTokenSize)
	for scanner.Scan() {
		if err := contextError(ctx); err != nil {
			return DecodeResult{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return DecodeResult{}, fmt.Errorf("scan bounded token: %w", ErrTokenTooLong)
		}
		return DecodeResult{}, fmt.Errorf("scan compatibility input: %w", err)
	}
	return DecodeResult{}, nil
}

func newDefaultAdapters() map[string]ParserAdapter {
	return map[string]ParserAdapter{
		"events_jsonl": eventsJSONLAdapter{},
		"frontmatter":  frontmatterAdapter{},
		"scanner":      scannerAdapter{},
	}
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("decode compatibility input: %w", err)
	}
	return nil
}

func splitFrontmatter(input []byte) ([]byte, bool, error) {
	if !bytes.HasPrefix(input, []byte("---\n")) {
		return nil, false, nil
	}

	rest := input[len("---\n"):]
	for offset := 0; offset <= len(rest); {
		next := bytes.IndexByte(rest[offset:], '\n')
		lineEnd := len(rest)
		nextOffset := len(rest) + 1
		if next >= 0 {
			lineEnd = offset + next
			nextOffset = lineEnd + 1
		}
		if bytes.Equal(rest[offset:lineEnd], []byte("---")) {
			return rest[:offset], true, nil
		}
		offset = nextOffset
	}
	return nil, true, fmt.Errorf("split frontmatter: %w", ErrUnclosedFront)
}

func decodeYAMLNode(input []byte) (*yaml.Node, error) {
	if len(bytes.TrimSpace(input)) == 0 {
		return &yaml.Node{}, nil
	}

	decoder := yaml.NewDecoder(bytes.NewReader(input))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		return nil, classifyYAMLError(err)
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple YAML documents")
		}
		return nil, fmt.Errorf("decode frontmatter trailing content: %w", classifyYAMLError(err))
	}
	return &root, nil
}

func classifyYAMLError(err error) error {
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "unexpected end of stream") ||
		strings.Contains(message, "unexpected eof") {
		return fmt.Errorf(
			"decode truncated frontmatter: %w",
			errors.Join(ErrTruncated, err),
		)
	}
	return fmt.Errorf(
		"decode malformed frontmatter: %w",
		errors.Join(ErrMalformed, err),
	)
}

func validateYAMLNode(node *yaml.Node) error {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			if err := validateYAMLNode(child); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Kind != yaml.MappingNode {
		for _, child := range node.Content {
			if err := validateYAMLNode(child); err != nil {
				return err
			}
		}
		return nil
	}

	exactKeys := make(map[string]struct{}, len(node.Content)/2)
	foldedKeys := make(map[string]string, len(node.Content)/2)
	for index := 0; index+1 < len(node.Content); index += 2 {
		key := node.Content[index]
		value := node.Content[index+1]
		if key.Kind != yaml.ScalarNode {
			return fmt.Errorf("validate frontmatter mapping key: %w", ErrMalformed)
		}
		if _, exists := exactKeys[key.Value]; exists {
			return fmt.Errorf("validate frontmatter key %q: %w", key.Value, ErrDuplicateKey)
		}
		folded := strings.ToLower(key.Value)
		if prior, exists := foldedKeys[folded]; exists && prior != key.Value {
			return fmt.Errorf(
				"validate frontmatter keys %q and %q: %w",
				prior,
				key.Value,
				ErrCaseFoldCollision,
			)
		}
		exactKeys[key.Value] = struct{}{}
		foldedKeys[folded] = key.Value

		if isPathField(key.Value) && value.Kind == yaml.ScalarNode && invalidWindowsPath(value.Value) {
			return fmt.Errorf("validate frontmatter path %q: %w", value.Value, ErrInvalidPath)
		}
		if err := validateYAMLNode(value); err != nil {
			return err
		}
	}
	return nil
}

func isPathField(key string) bool {
	folded := strings.ToLower(key)
	return folded == "path" || strings.HasSuffix(folded, "_path")
}

func invalidWindowsPath(value string) bool {
	if strings.Contains(value, `\`) {
		return true
	}
	for _, component := range strings.Split(value, "/") {
		trimmed := strings.TrimRight(component, " .")
		base, _, _ := strings.Cut(trimmed, ".")
		switch strings.ToUpper(base) {
		case "CON", "PRN", "AUX", "NUL",
			"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
			"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9":
			return true
		}
	}
	return false
}

func validateJSONKeys(input []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return classifyJSONError(err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return classifyJSONError(err)
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			rawKey, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := rawKey.(string)
			if !ok {
				return fmt.Errorf("object key has type %T", rawKey)
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("decode duplicate JSON key %q: %w", key, ErrDuplicateKey)
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return fmt.Errorf("object ended with %v", end)
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return fmt.Errorf("array ended with %v", end)
		}
	default:
		return fmt.Errorf("unexpected closing delimiter %q", delim)
	}
	return nil
}

func classifyJSONError(err error) error {
	if errors.Is(err, ErrDuplicateKey) {
		return err
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		strings.Contains(strings.ToLower(err.Error()), "unexpected end") {
		return fmt.Errorf(
			"decode truncated event JSON: %w",
			errors.Join(ErrTruncated, err),
		)
	}
	return fmt.Errorf(
		"decode malformed event JSON: %w",
		errors.Join(ErrMalformed, err),
	)
}
