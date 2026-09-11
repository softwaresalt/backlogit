// Package auditsuccess declares the FL004 success-after-audit-warning analyzer.
package auditsuccess

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	diagnostic           = "FL004: success returned in same block after an audit warning without failing closed"
	suppressionDirective = "// faultline:warn-nonfatal"
)

// Analyzer declares the FL004 success-after-audit-warning analyzer.
var Analyzer = &analysis.Analyzer{
	Name: "FL004auditsuccess",
	Doc:  "reports success returns in the same block after declared audit-warning calls",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	errorType := types.Universe.Lookup("error").Type()

	for _, file := range pass.Files {
		parents := parentNodes(file)
		ast.Inspect(file, func(node ast.Node) bool {
			block, ok := node.(*ast.BlockStmt)
			if !ok {
				return true
			}

			signature := enclosingSignature(pass, parents, block)
			if signature == nil || !hasTrailingErrorResult(signature, errorType) {
				return true
			}
			analyzeBlock(pass, file, block, signature)
			return true
		})
	}

	return nil, nil
}

func analyzeBlock(
	pass *analysis.Pass,
	file *ast.File,
	block *ast.BlockStmt,
	signature *types.Signature,
) {
	warnings := make([]*ast.CallExpr, 0, 1)
	for _, statement := range block.List {
		if result, ok := statement.(*ast.ReturnStmt); ok && isSuccessReturn(pass, result, signature) {
			for _, warning := range warnings {
				pass.Reportf(warning.Pos(), diagnostic)
			}
			warnings = warnings[:0]
			continue
		}

		if containsErrorReturn(pass, statement, signature) {
			warnings = warnings[:0]
		}

		call := directCall(statement)
		if call == nil ||
			!isAuditWarning(pass, call) ||
			isSuppressed(pass, file, block, statement) {
			continue
		}
		warnings = append(warnings, call)
	}
}

func directCall(statement ast.Stmt) *ast.CallExpr {
	expression, ok := statement.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, _ := ast.Unparen(expression.X).(*ast.CallExpr)
	return call
}

func isAuditWarning(pass *analysis.Pass, call *ast.CallExpr) bool {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}

	for _, sink := range auditWarningSinks {
		if selector.Sel.Name != sink.selector {
			continue
		}
		switch sink.kind {
		case packageFunctionSink:
			if matchesPackageFunction(pass, selector, sink.packagePath) {
				return true
			}
		case loggerMethodSink:
			if matchesReceiver(pass, selector, sink.receiverPackage, sink.receiverName) {
				return true
			}
		case selectorNameSink:
			return true
		}
	}
	return false
}

func matchesPackageFunction(pass *analysis.Pass, selector *ast.SelectorExpr, packagePath string) bool {
	if pass.TypesInfo.Selections[selector] != nil {
		return false
	}
	function, ok := pass.TypesInfo.ObjectOf(selector.Sel).(*types.Func)
	return ok && function.Pkg() != nil && function.Pkg().Path() == packagePath
}

func matchesReceiver(
	pass *analysis.Pass,
	selector *ast.SelectorExpr,
	packagePath string,
	typeName string,
) bool {
	selection := pass.TypesInfo.Selections[selector]
	if selection == nil {
		return false
	}

	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	signature, ok := method.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return false
	}

	receiver := types.Unalias(signature.Recv().Type())
	if pointer, pointerReceiver := receiver.(*types.Pointer); pointerReceiver {
		receiver = types.Unalias(pointer.Elem())
	}
	named, ok := receiver.(*types.Named)
	return ok &&
		named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == packagePath &&
		named.Obj().Name() == typeName
}

func enclosingSignature(
	pass *analysis.Pass,
	parents map[ast.Node]ast.Node,
	node ast.Node,
) *types.Signature {
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		switch function := parent.(type) {
		case *ast.FuncDecl:
			object, _ := pass.TypesInfo.ObjectOf(function.Name).(*types.Func)
			if object == nil {
				return nil
			}
			signature, _ := object.Type().(*types.Signature)
			return signature
		case *ast.FuncLit:
			signature, _ := pass.TypesInfo.TypeOf(function).(*types.Signature)
			return signature
		}
	}
	return nil
}

func hasTrailingErrorResult(signature *types.Signature, errorType types.Type) bool {
	results := signature.Results()
	return results.Len() > 0 &&
		types.Identical(types.Unalias(results.At(results.Len()-1).Type()), errorType)
}

func containsErrorReturn(
	pass *analysis.Pass,
	statement ast.Stmt,
	signature *types.Signature,
) bool {
	found := false
	ast.Inspect(statement, func(node ast.Node) bool {
		if node == nil || found {
			return false
		}
		if _, nestedFunction := node.(*ast.FuncLit); nestedFunction {
			return false
		}
		result, ok := node.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		if !isSuccessReturn(pass, result, signature) {
			found = true
		}
		return false
	})
	return found
}

func isSuccessReturn(
	pass *analysis.Pass,
	result *ast.ReturnStmt,
	signature *types.Signature,
) bool {
	results := signature.Results()
	if len(result.Results) != results.Len() {
		return false
	}
	if !isNilZero(
		pass,
		result.Results[len(result.Results)-1],
		results.At(results.Len()-1).Type(),
	) {
		return false
	}

	for index, expression := range result.Results[:len(result.Results)-1] {
		if !isZeroValue(pass, expression, results.At(index).Type()) {
			return false
		}
	}
	return true
}

func isZeroValue(pass *analysis.Pass, expression ast.Expr, resultType types.Type) bool {
	expression = ast.Unparen(expression)
	if isNilZero(pass, expression, resultType) {
		return true
	}
	if isInterface(resultType) {
		return false
	}

	if value := pass.TypesInfo.Types[expression].Value; value != nil {
		switch value.Kind() {
		case constant.Bool:
			return !constant.BoolVal(value)
		case constant.String:
			return constant.StringVal(value) == ""
		case constant.Int, constant.Float:
			return constant.Sign(value) == 0
		case constant.Complex:
			return constant.Sign(constant.Real(value)) == 0 &&
				constant.Sign(constant.Imag(value)) == 0
		}
	}

	literal, ok := expression.(*ast.CompositeLit)
	if !ok || len(literal.Elts) != 0 {
		return false
	}
	literalType := pass.TypesInfo.TypeOf(literal)
	if literalType == nil || !types.AssignableTo(literalType, resultType) {
		return false
	}
	switch resultType.Underlying().(type) {
	case *types.Array, *types.Struct:
		return true
	default:
		return false
	}
}

func isNilZero(pass *analysis.Pass, expression ast.Expr, resultType types.Type) bool {
	expression = ast.Unparen(expression)
	if typeAndValue, ok := pass.TypesInfo.Types[expression]; ok && typeAndValue.IsNil() {
		return isNilable(resultType)
	}

	expressionType, typedNil := typedNilType(pass, expression)
	if !typedNil ||
		!types.AssignableTo(expressionType, resultType) ||
		(isInterface(resultType) && !isInterface(expressionType)) {
		return false
	}
	return true
}

func typedNilType(pass *analysis.Pass, expression ast.Expr) (types.Type, bool) {
	call, ok := ast.Unparen(expression).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return nil, false
	}

	conversion, ok := pass.TypesInfo.Types[ast.Unparen(call.Fun)]
	if !ok || !conversion.IsType() {
		return nil, false
	}

	convertedType := pass.TypesInfo.TypeOf(call)
	if convertedType == nil || !isNilable(convertedType) {
		return nil, false
	}

	argument := ast.Unparen(call.Args[0])
	if typeAndValue, exists := pass.TypesInfo.Types[argument]; exists && typeAndValue.IsNil() {
		return convertedType, true
	}

	argumentType, typedNil := typedNilType(pass, argument)
	if !typedNil || (isInterface(convertedType) && !isInterface(argumentType)) {
		return nil, false
	}
	return convertedType, true
}

func isNilable(valueType types.Type) bool {
	switch underlying := valueType.Underlying().(type) {
	case *types.Chan, *types.Interface, *types.Map, *types.Pointer, *types.Signature, *types.Slice:
		return true
	case *types.Basic:
		return underlying.Kind() == types.UnsafePointer
	default:
		return false
	}
}

func isInterface(valueType types.Type) bool {
	_, ok := valueType.Underlying().(*types.Interface)
	return ok
}

func isSuppressed(
	pass *analysis.Pass,
	file *ast.File,
	block *ast.BlockStmt,
	statement ast.Stmt,
) bool {
	startLine := pass.Fset.Position(statement.Pos()).Line
	endLine := pass.Fset.Position(statement.End()).Line
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.TrimSpace(comment.Text) != suppressionDirective {
				continue
			}

			line := pass.Fset.Position(comment.Pos()).Line
			if line == endLine &&
				comment.Pos() >= statement.End() &&
				trailingStatement(pass, block, comment) == statement {
				return true
			}
			if line == startLine-1 &&
				isDedicatedComment(pass, file, comment) &&
				followingStatement(pass, block, comment) == statement {
				return true
			}
		}
	}
	return false
}

func trailingStatement(
	pass *analysis.Pass,
	block *ast.BlockStmt,
	comment *ast.Comment,
) ast.Stmt {
	commentLine := pass.Fset.Position(comment.Pos()).Line
	var owner ast.Stmt
	for _, candidate := range block.List {
		if candidate.End() > comment.Pos() ||
			pass.Fset.Position(candidate.End()).Line != commentLine {
			continue
		}
		if owner == nil || candidate.End() > owner.End() {
			owner = candidate
		}
	}
	return owner
}

func followingStatement(
	pass *analysis.Pass,
	block *ast.BlockStmt,
	comment *ast.Comment,
) ast.Stmt {
	followingLine := pass.Fset.Position(comment.Pos()).Line + 1
	var owner ast.Stmt
	for _, candidate := range block.List {
		if candidate.Pos() < comment.End() ||
			pass.Fset.Position(candidate.Pos()).Line != followingLine {
			continue
		}
		if owner == nil || candidate.Pos() < owner.Pos() {
			owner = candidate
		}
	}
	return owner
}

func isDedicatedComment(pass *analysis.Pass, file *ast.File, comment *ast.Comment) bool {
	line := pass.Fset.Position(comment.Pos()).Line
	dedicated := true
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil || !dedicated {
			return dedicated
		}
		switch node.(type) {
		case *ast.Comment, *ast.CommentGroup:
			return false
		}
		if node.Pos() >= comment.Pos() || node.End() > comment.Pos() {
			return true
		}
		if pass.Fset.Position(node.End()).Line == line {
			dedicated = false
			return false
		}
		return true
	})
	return dedicated
}

func parentNodes(root ast.Node) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}
