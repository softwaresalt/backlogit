package core

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestU1_ClaimMarkerWriterSourceShape(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "shipment_lifecycle.go", nil, 0)
	if err != nil {
		t.Fatalf("parse shipment_lifecycle.go: %v", err)
	}

	writers := claimMarkerWriterCandidates(file)
	if len(writers) == 0 {
		t.Fatalf("no receiverless unexported writer declares a claimMarker string parameter")
	}

	setArtifactStatus := findU1Function(file, "setArtifactStatus")
	if setArtifactStatus == nil {
		t.Fatalf("setArtifactStatus is not declared in shipment_lifecycle.go")
	}
	claimShipment := findU1Function(file, "ClaimShipment")
	if claimShipment == nil {
		t.Fatalf("ClaimShipment is not declared in shipment_lifecycle.go")
	}

	for _, writer := range writers {
		if hasU1WriterCall(setArtifactStatus, writer, isU1EmptyString) &&
			hasU1WriterCall(claimShipment, writer, isU1ShipmentID) {
			return
		}
	}

	t.Fatalf("setArtifactStatus and ClaimShipment must call the same claimMarker writer with \"\" and shipmentID")
}

type claimMarkerWriterCandidate struct {
	name       string
	paramIndex int
}

func claimMarkerWriterCandidates(file *ast.File) []claimMarkerWriterCandidate {
	writers := make([]claimMarkerWriterCandidate, 0)
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || ast.IsExported(function.Name.Name) {
			continue
		}
		if paramIndex, ok := claimMarkerStringParamIndex(function); ok {
			writers = append(writers, claimMarkerWriterCandidate{
				name:       function.Name.Name,
				paramIndex: paramIndex,
			})
		}
	}
	return writers
}

func claimMarkerStringParamIndex(function *ast.FuncDecl) (int, bool) {
	if function.Type.Params == nil {
		return 0, false
	}

	paramIndex := 0
	for _, field := range function.Type.Params.List {
		fieldWidth := len(field.Names)
		if fieldWidth == 0 {
			fieldWidth = 1
		}
		for nameIndex, name := range field.Names {
			typeName, isString := field.Type.(*ast.Ident)
			if name.Name == "claimMarker" && isString && typeName.Name == "string" {
				return paramIndex + nameIndex, true
			}
		}
		paramIndex += fieldWidth
	}
	return 0, false
}

func findU1Function(file *ast.File, name string) *ast.FuncDecl {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == name {
			return function
		}
	}
	return nil
}

func hasU1WriterCall(function *ast.FuncDecl, writer claimMarkerWriterCandidate, matches func(ast.Expr) bool) bool {
	if function.Body == nil {
		return false
	}

	found := false
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok || callee.Name != writer.name || len(call.Args) <= writer.paramIndex {
			return true
		}
		if matches(call.Args[writer.paramIndex]) {
			found = true
		}
		return true
	})
	return found
}

func isU1EmptyString(expression ast.Expr) bool {
	literal, ok := expression.(*ast.BasicLit)
	return ok && literal.Kind == token.STRING && literal.Value == `""`
}

func isU1ShipmentID(expression ast.Expr) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == "shipmentID"
}
