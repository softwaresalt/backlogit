package format

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

// JSON-RPC 2.0 standard error codes.
const (
	// ErrCodeParseError is returned when the server received invalid JSON.
	ErrCodeParseError = -32700
	// ErrCodeInvalidRequest is returned when the JSON sent is not a valid Request object.
	ErrCodeInvalidRequest = -32600
	// ErrCodeMethodNotFound is returned when the method does not exist or is not available.
	ErrCodeMethodNotFound = -32601
	// ErrCodeInvalidParams is returned when invalid method parameter(s) were supplied.
	ErrCodeInvalidParams = -32602
	// ErrCodeInternalError is returned for an internal JSON-RPC error.
	ErrCodeInternalError = -32603
	// ErrCodeServerError is the base value for server-defined errors (-32000 to -32099).
	ErrCodeServerError = -32000
)

const jsonrpcVersion = "2.0"

// JSONRPCResponse is a JSON-RPC 2.0 response object. Exactly one of Result or
// Error will be non-nil in a well-formed response.
type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError is the error object embedded in a JSON-RPC 2.0 error response.
// The optional Data field carries structured diagnostic information; it is
// omitted from the JSON output when nil (never serialised as null).
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// WrapResult serialises a successful JSON-RPC 2.0 response. id is typically
// the CLI command path (e.g. "backlogit list"). result may be any
// JSON-serialisable value, including nil (rendered as null).
func WrapResult(id string, result any) ([]byte, error) {
	resp := JSONRPCResponse{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Result:  result,
	}
	// Ensure "result" key is always present even when nil.
	type resultAlways struct {
		JSONRPC string `json:"jsonrpc"`
		ID      string `json:"id"`
		Result  any    `json:"result"`
	}
	b, err := json.Marshal(resultAlways{
		JSONRPC: resp.JSONRPC,
		ID:      resp.ID,
		Result:  resp.Result,
	})
	if err != nil {
		return nil, fmt.Errorf("jsonrpc wrap result: %w", err)
	}
	return b, nil
}

// WrapError serialises a JSON-RPC 2.0 error response. id is the CLI command
// path. code should be one of the ErrCode* constants or an application-defined
// value in the -32000 range.
func WrapError(id string, code int, msg string) ([]byte, error) {
	resp := JSONRPCResponse{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: msg},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("jsonrpc wrap error: %w", err)
	}
	return b, nil
}

// WrapErrorData serialises a JSON-RPC 2.0 error response with an optional
// structured data payload. When data is nil (or a typed nil — a nil pointer,
// map, slice, channel, function, or interface wrapped in a non-nil interface
// value), the "data" key is absent from the output, making the result
// byte-identical to WrapError. A non-nil data value is embedded verbatim.
//
// Typed-nil guard: a plain data == nil check does not catch typed-nil values
// such as (*SomeStruct)(nil), which the Go runtime represents as a non-nil
// interface holding a nil concrete pointer. reflect.Value.IsNil() is used to
// normalise all such cases to a true nil before serialisation.
func WrapErrorData(id string, code int, msg string, data any) ([]byte, error) {
	// Typed-nil guard: normalise typed-nil pointers / maps / slices / etc.
	// so they are never marshalled as "data":null.
	if data != nil {
		v := reflect.ValueOf(data)
		k := v.Kind()
		if (k == reflect.Pointer || k == reflect.Map || k == reflect.Slice ||
			k == reflect.Chan || k == reflect.Func || k == reflect.Interface) && v.IsNil() {
			data = nil
		}
	}
	resp := JSONRPCResponse{
		JSONRPC: jsonrpcVersion,
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: msg, Data: data},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("jsonrpc wrap error data: %w", err)
	}
	return b, nil
}

// JSONRPCRenderer wraps row data in a JSON-RPC 2.0 success envelope. The id
// field is typically the CLI command path and is set at construction time.
// It implements the Renderer interface.
type JSONRPCRenderer struct {
	id string
}

// NewJSONRPCRenderer returns a JSONRPCRenderer for the given command id.
func NewJSONRPCRenderer(id string) *JSONRPCRenderer {
	return &JSONRPCRenderer{id: id}
}

// Render writes a JSON-RPC 2.0 success response to w. The result payload is a
// JSON array of objects whose keys are limited to the declared columns.
func (r *JSONRPCRenderer) Render(w io.Writer, columns []Column, rows []map[string]any) error {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		elem := make(map[string]any, len(columns))
		for _, c := range columns {
			elem[c.Key] = row[c.Key]
		}
		out = append(out, elem)
	}
	b, err := WrapResult(r.id, out)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}
