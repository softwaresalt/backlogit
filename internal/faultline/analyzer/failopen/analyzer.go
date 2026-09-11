// Package failopen declares the FL003 fail-open branch analyzer.
package failopen

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	diagnostic           = "FL003: error branch returns success (fail-open); safety-mode requires fail-closed"
	suppressionDirective = "// faultline:fail-open-ok"
)

// Analyzer declares the FL003 fail-open branch analyzer.
var Analyzer = &analysis.Analyzer{
	Name: "FL003failopen",
	Doc:  "reports error branches that return success instead of failing closed",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	errorType := types.Universe.Lookup("error").Type()

	for _, file := range pass.Files {
		parents := parentNodes(file)
		ast.Inspect(file, func(node ast.Node) bool {
			branch, ok := node.(*ast.IfStmt)
			if !ok {
				return true
			}

			signature, deferred := enclosingSignature(pass, parents, branch)
			if deferred ||
				signature == nil ||
				!hasTrailingErrorResult(signature, errorType) ||
				!isErrorNonNilCondition(pass, branch.Cond, errorType) ||
				isSuppressed(pass, file, branch) ||
				!returnsSuccess(pass, branch.Body, signature) {
				return true
			}

			pass.Reportf(branch.Pos(), diagnostic)
			return true
		})
	}

	return nil, nil
}

func enclosingSignature(
	pass *analysis.Pass,
	parents map[ast.Node]ast.Node,
	node ast.Node,
) (*types.Signature, bool) {
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		switch function := parent.(type) {
		case *ast.FuncDecl:
			object, _ := pass.TypesInfo.ObjectOf(function.Name).(*types.Func)
			if object == nil {
				return nil, false
			}
			signature, _ := object.Type().(*types.Signature)
			return signature, false
		case *ast.FuncLit:
			signature, _ := pass.TypesInfo.TypeOf(function).(*types.Signature)
			return signature, isDeferredClosure(parents, function)
		}
	}
	return nil, false
}

func isDeferredClosure(parents map[ast.Node]ast.Node, function *ast.FuncLit) bool {
	var expression ast.Expr = function
	parent := parents[function]
	for {
		parentheses, ok := parent.(*ast.ParenExpr)
		if !ok {
			break
		}
		expression = parentheses
		parent = parents[parentheses]
	}

	call, ok := parent.(*ast.CallExpr)
	if !ok || ast.Unparen(call.Fun) != ast.Unparen(expression) {
		return false
	}
	deferred, ok := parents[call].(*ast.DeferStmt)
	return ok && deferred.Call == call
}

func hasTrailingErrorResult(signature *types.Signature, errorType types.Type) bool {
	results := signature.Results()
	return results.Len() > 0 &&
		types.Identical(types.Unalias(results.At(results.Len()-1).Type()), errorType)
}

func isErrorNonNilCondition(pass *analysis.Pass, expression ast.Expr, errorType types.Type) bool {
	comparison, ok := ast.Unparen(expression).(*ast.BinaryExpr)
	if !ok || comparison.Op != token.NEQ {
		return false
	}

	var candidate ast.Expr
	switch {
	case isNil(pass, comparison.X):
		candidate = comparison.Y
	case isNil(pass, comparison.Y):
		candidate = comparison.X
	default:
		return false
	}

	identifier, ok := ast.Unparen(candidate).(*ast.Ident)
	if !ok {
		return false
	}
	object, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
	return ok && types.AssignableTo(object.Type(), errorType)
}

func returnsSuccess(pass *analysis.Pass, block *ast.BlockStmt, signature *types.Signature) bool {
	if block == nil || len(block.List) == 0 {
		return false
	}

	result, ok := block.List[len(block.List)-1].(*ast.ReturnStmt)
	if !ok || len(result.Results) != signature.Results().Len() {
		return false
	}
	if !isNilZero(
		pass,
		result.Results[len(result.Results)-1],
		signature.Results().At(signature.Results().Len()-1).Type(),
	) {
		return false
	}

	for index, expression := range result.Results[:len(result.Results)-1] {
		if !isZeroValue(pass, expression, signature.Results().At(index).Type()) {
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

func isNil(pass *analysis.Pass, expression ast.Expr) bool {
	typeAndValue, ok := pass.TypesInfo.Types[ast.Unparen(expression)]
	return ok && typeAndValue.IsNil()
}

func isNilZero(pass *analysis.Pass, expression ast.Expr, resultType types.Type) bool {
	expression = ast.Unparen(expression)
	if isNil(pass, expression) {
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
	if isNil(pass, argument) {
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

func isSuppressed(pass *analysis.Pass, file *ast.File, branch *ast.IfStmt) bool {
	startLine := pass.Fset.Position(branch.Pos()).Line
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.TrimSpace(comment.Text) != suppressionDirective {
				continue
			}

			line := pass.Fset.Position(comment.Pos()).Line
			if isInlineDirective(pass, branch, comment) {
				return true
			}
			if line == startLine-1 && isDedicatedComment(pass, file, comment) {
				return true
			}
		}
	}
	return false
}

func isInlineDirective(pass *analysis.Pass, branch *ast.IfStmt, comment *ast.Comment) bool {
	if comment.Pos() <= branch.Body.Lbrace ||
		pass.Fset.Position(comment.Pos()).Line != pass.Fset.Position(branch.Body.Lbrace).Line {
		return false
	}
	return len(branch.Body.List) == 0 || comment.Pos() < branch.Body.List[0].Pos()
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
