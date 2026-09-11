// Package scannerdiscipline declares the FL001 scanner-discipline analyzer.
package scannerdiscipline

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const diagnostic = "FL001: bufio.Scanner used without Buffer() bound and/or Err() check"

// Analyzer declares the FL001 scanner-discipline analyzer.
var Analyzer *analysis.Analyzer = &analysis.Analyzer{
	Name: "FL001scannerdiscipline",
	Doc:  "reports scanner loops that do not check scanner errors",
	Run:  run,
}

type scannerCreation struct {
	assignment *ast.AssignStmt
	call       *ast.CallExpr
	object     *types.Var
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if strings.HasSuffix(filepath.Base(pass.Fset.Position(file.Pos()).Filename), "_test.go") {
			continue
		}

		ast.Inspect(file, func(node ast.Node) bool {
			switch function := node.(type) {
			case *ast.FuncDecl:
				analyzeFunction(pass, file, function.Type, function.Body)
			case *ast.FuncLit:
				analyzeFunction(pass, file, function.Type, function.Body)
			}
			return true
		})
	}

	return nil, nil
}

func analyzeFunction(pass *analysis.Pass, file *ast.File, functionType *ast.FuncType, body *ast.BlockStmt) {
	if body == nil {
		return
	}

	creations := scannerCreations(pass, functionType, body)
	for _, creation := range creations {
		upperBound := nextDirectAssignment(
			pass,
			body,
			creation.object,
			creation.assignment.End(),
		)

		if hasSuppression(pass, file, creation.assignment) ||
			scannerEscapes(
				pass,
				functionType,
				body,
				creation.object,
				creation.assignment.End(),
				upperBound,
			) {
			continue
		}

		firstLoop, lastLoopEnd := scanLoopBounds(pass, body, creation.object, creation.assignment.End(), upperBound)
		if firstLoop == 0 {
			continue
		}

		hasBuffer := hasMethodCall(
			pass,
			body,
			creation.object,
			"Buffer",
			creation.assignment.End(),
			firstLoop,
		)
		hasErr := hasMeaningfulErrUse(pass, body, creation.object, lastLoopEnd, upperBound)
		if !hasBuffer || !hasErr {
			pass.Reportf(creation.call.Pos(), diagnostic)
		}
	}
}

func nextDirectAssignment(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	object *types.Var,
	lowerBound token.Pos,
) token.Pos {
	upperBound := body.End()
	inspectFunctionBody(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || assignment.Pos() <= lowerBound || assignment.Pos() >= upperBound {
			return true
		}
		for _, left := range assignment.Lhs {
			identifier, ok := ast.Unparen(left).(*ast.Ident)
			if ok && pass.TypesInfo.ObjectOf(identifier) == object {
				upperBound = assignment.Pos()
				return false
			}
		}
		return true
	})
	return upperBound
}

func scannerCreations(
	pass *analysis.Pass,
	functionType *ast.FuncType,
	body *ast.BlockStmt,
) []scannerCreation {
	var creations []scannerCreation
	inspectFunctionBody(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != len(assignment.Rhs) {
			return true
		}

		for index, right := range assignment.Rhs {
			call, ok := ast.Unparen(right).(*ast.CallExpr)
			if !ok || !isBufioNewScanner(pass, call) {
				continue
			}

			identifier, ok := ast.Unparen(assignment.Lhs[index]).(*ast.Ident)
			if !ok {
				continue
			}
			object, _ := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
			if object == nil ||
				object.Pkg() != pass.Pkg ||
				object.Parent() == pass.Pkg.Scope() ||
				object.Pos() < functionType.Pos() ||
				object.Pos() > body.End() {
				continue
			}

			creations = append(creations, scannerCreation{
				assignment: assignment,
				call:       call,
				object:     object,
			})
		}
		return true
	})
	return creations
}

func isBufioNewScanner(pass *analysis.Pass, call *ast.CallExpr) bool {
	var object types.Object
	switch function := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		object = pass.TypesInfo.ObjectOf(function)
	case *ast.SelectorExpr:
		object = pass.TypesInfo.ObjectOf(function.Sel)
	}

	newScanner, ok := object.(*types.Func)
	if !ok || newScanner.Pkg() == nil ||
		newScanner.Pkg().Path() != "bufio" ||
		newScanner.Name() != "NewScanner" {
		return false
	}

	pointer, ok := pass.TypesInfo.TypeOf(call).(*types.Pointer)
	if !ok {
		return false
	}
	scanner, ok := pointer.Elem().(*types.Named)
	return ok &&
		scanner.Obj().Pkg() != nil &&
		scanner.Obj().Pkg().Path() == "bufio" &&
		scanner.Obj().Name() == "Scanner"
}

func hasSuppression(pass *analysis.Pass, file *ast.File, assignment *ast.AssignStmt) bool {
	startLine := pass.Fset.Position(assignment.Pos()).Line
	endLine := pass.Fset.Position(assignment.End()).Line
	for _, group := range file.Comments {
		for _, comment := range group.List {
			line := pass.Fset.Position(comment.Pos()).Line
			if (line == startLine-1 || line >= startLine && line <= endLine) &&
				strings.TrimSpace(comment.Text) == "// faultline:scanner-ok" {
				return true
			}
		}
	}
	return false
}

func scanLoopBounds(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	object *types.Var,
	lowerBound,
	upperBound token.Pos,
) (token.Pos, token.Pos) {
	var firstLoop token.Pos
	var lastLoopEnd token.Pos
	inspectFunctionBody(body, func(node ast.Node) bool {
		loop, ok := node.(*ast.ForStmt)
		if !ok || loop.Pos() <= lowerBound || loop.End() >= upperBound {
			return true
		}
		if !containsMethodCall(pass, loop, object, "Scan") {
			return true
		}
		if firstLoop == 0 || loop.Pos() < firstLoop {
			firstLoop = loop.Pos()
		}
		if loop.End() > lastLoopEnd {
			lastLoopEnd = loop.End()
		}
		return true
	})
	return firstLoop, lastLoopEnd
}

func hasMethodCall(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	object *types.Var,
	name string,
	lowerBound,
	upperBound token.Pos,
) bool {
	found := false
	inspectFunctionBody(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok &&
			call.Pos() > lowerBound &&
			call.End() < upperBound &&
			isMethodCallOn(pass, call, object, name) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func hasMeaningfulErrUse(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	object *types.Var,
	lowerBound,
	upperBound token.Pos,
) bool {
	parents := parentNodes(body)
	found := false
	inspectFunctionBody(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok &&
			call.Pos() > lowerBound &&
			call.End() < upperBound &&
			isMethodCallOn(pass, call, object, "Err") &&
			meaningfullyConsumesErr(pass, body, call, parents, upperBound) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func meaningfullyConsumesErr(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	call *ast.CallExpr,
	parents map[ast.Node]ast.Node,
	upperBound token.Pos,
) bool {
	for node := parents[call]; node != nil; node = parents[node] {
		switch parent := node.(type) {
		case *ast.ExprStmt:
			return false
		case *ast.AssignStmt:
			object := assignedCallObject(pass, parent.Lhs, parent.Rhs, call)
			return errorObjectIsChecked(pass, body, parents, object, call.End(), upperBound)
		case *ast.ValueSpec:
			names := make([]ast.Expr, 0, len(parent.Names))
			for _, name := range parent.Names {
				names = append(names, name)
			}
			object := assignedCallObject(pass, names, parent.Values, call)
			return errorObjectIsChecked(pass, body, parents, object, call.End(), upperBound)
		case *ast.ReturnStmt:
			return true
		case *ast.IfStmt:
			return containsPosition(parent.Cond, call)
		case *ast.ForStmt:
			return containsPosition(parent.Cond, call)
		case *ast.SwitchStmt:
			if parent.Tag != nil && containsPosition(parent.Tag, call) {
				return true
			}
		case *ast.CaseClause:
			for _, expression := range parent.List {
				if containsPosition(expression, call) {
					return true
				}
			}
		}
	}
	return false
}

func assignedCallObject(
	pass *analysis.Pass,
	targets []ast.Expr,
	values []ast.Expr,
	call *ast.CallExpr,
) *types.Var {
	index := containingExpressionIndex(values, call)
	if index < 0 {
		return nil
	}
	if len(targets) != len(values) {
		return nil
	}
	identifier, ok := ast.Unparen(targets[index]).(*ast.Ident)
	if !ok || identifier.Name == "_" {
		return nil
	}
	object, _ := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
	return object
}

func errorObjectIsChecked(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	parents map[ast.Node]ast.Node,
	object *types.Var,
	lowerBound,
	upperBound token.Pos,
) bool {
	if object == nil {
		return false
	}

	checked := false
	inspectFunctionBody(body, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok ||
			identifier.Pos() <= lowerBound ||
			identifier.End() >= upperBound ||
			pass.TypesInfo.ObjectOf(identifier) != object {
			return true
		}
		if identifierMeaningfullyChecksError(identifier, parents) {
			checked = true
			return false
		}
		return true
	})
	return checked
}

func identifierMeaningfullyChecksError(
	identifier *ast.Ident,
	parents map[ast.Node]ast.Node,
) bool {
	for node := parents[identifier]; node != nil; node = parents[node] {
		switch parent := node.(type) {
		case *ast.ReturnStmt:
			return true
		case *ast.IfStmt:
			return containsPosition(parent.Cond, identifier)
		case *ast.ForStmt:
			return containsPosition(parent.Cond, identifier)
		case *ast.SwitchStmt:
			return parent.Tag != nil && containsPosition(parent.Tag, identifier)
		case *ast.CaseClause:
			for _, expression := range parent.List {
				if containsPosition(expression, identifier) {
					return true
				}
			}
		case ast.Stmt:
			return false
		}
	}
	return false
}

func containingExpressionIndex(expressions []ast.Expr, target ast.Node) int {
	for index, expression := range expressions {
		if containsPosition(expression, target) {
			return index
		}
	}
	return -1
}

func containsPosition(container, target ast.Node) bool {
	return container != nil &&
		target != nil &&
		target.Pos() >= container.Pos() &&
		target.End() <= container.End()
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
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func containsMethodCall(pass *analysis.Pass, root ast.Node, object *types.Var, name string) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if found {
			return false
		}
		if loop, nested := node.(*ast.ForStmt); nested && loop != root {
			return false
		}
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		call, ok := node.(*ast.CallExpr)
		if ok && isMethodCallOn(pass, call, object, name) {
			found = true
			return false
		}
		return true
	})
	return found
}

func isMethodCallOn(pass *analysis.Pass, call *ast.CallExpr, object *types.Var, name string) bool {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != name {
		return false
	}
	return isSelectorCallOn(pass, selector, object)
}

func isCallOn(pass *analysis.Pass, call *ast.CallExpr, object *types.Var) bool {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	return ok && isSelectorCallOn(pass, selector, object)
}

func isSelectorCallOn(pass *analysis.Pass, selector *ast.SelectorExpr, object *types.Var) bool {
	identifier, ok := ast.Unparen(selector.X).(*ast.Ident)
	return ok && pass.TypesInfo.ObjectOf(identifier) == object
}

func scannerEscapes(
	pass *analysis.Pass,
	functionType *ast.FuncType,
	body *ast.BlockStmt,
	object *types.Var,
	lowerBound,
	upperBound token.Pos,
) bool {
	namedResults := namedResultVariables(pass, functionType)
	if _, namedResult := namedResults[object]; namedResult &&
		hasBareReturn(body, lowerBound, upperBound) {
		return true
	}

	escaped := false
	inspectFunctionBody(body, func(node ast.Node) bool {
		if escaped || node.Pos() <= lowerBound || node.Pos() >= upperBound {
			return !escaped
		}

		switch value := node.(type) {
		case *ast.FuncLit:
			if referencesObject(pass, value, object) {
				escaped = true
			}
			return false
		case *ast.CallExpr:
			for _, argument := range value.Args {
				if isObjectValue(pass, argument, object) {
					escaped = true
					break
				}
			}
		case *ast.ReturnStmt:
			for _, result := range value.Results {
				if isObjectValue(pass, result, object) {
					escaped = true
					break
				}
			}
		case *ast.AssignStmt:
			for index, right := range value.Rhs {
				if storedObjectValue(pass, right, object) ||
					(isDirectObjectValue(pass, right, object) &&
						assignmentEscapes(
							pass,
							value,
							index,
							namedResults,
							body,
							upperBound,
						)) {
					escaped = true
					break
				}
			}
		case *ast.ValueSpec:
			for _, initializer := range value.Values {
				if storedObjectValue(pass, initializer, object) {
					escaped = true
					break
				}
			}
		case *ast.SendStmt:
			escaped = isObjectValue(pass, value.Value, object)
		case *ast.GoStmt:
			escaped = isCallOn(pass, value.Call, object)
		}

		return !escaped
	})
	return escaped
}

func namedResultVariables(pass *analysis.Pass, functionType *ast.FuncType) map[*types.Var]struct{} {
	results := make(map[*types.Var]struct{})
	if functionType.Results == nil {
		return results
	}
	for _, field := range functionType.Results.List {
		for _, name := range field.Names {
			if object, ok := pass.TypesInfo.ObjectOf(name).(*types.Var); ok {
				results[object] = struct{}{}
			}
		}
	}
	return results
}

func assignmentEscapes(
	pass *analysis.Pass,
	assignment *ast.AssignStmt,
	rightIndex int,
	namedResults map[*types.Var]struct{},
	body *ast.BlockStmt,
	upperBound token.Pos,
) bool {
	if rightIndex >= len(assignment.Lhs) {
		return false
	}

	target := ast.Unparen(assignment.Lhs[rightIndex])
	switch value := target.(type) {
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr, *ast.StarExpr:
		return true
	case *ast.Ident:
		if value.Name == "_" {
			return false
		}
		targetObject, _ := pass.TypesInfo.ObjectOf(value).(*types.Var)
		if targetObject == nil {
			return false
		}
		if targetObject.Parent() == pass.Pkg.Scope() {
			return true
		}
		if _, namedResult := namedResults[targetObject]; namedResult {
			return hasBareReturn(body, assignment.End(), upperBound)
		}
	}
	return false
}

func hasBareReturn(body *ast.BlockStmt, lowerBound, upperBound token.Pos) bool {
	found := false
	inspectFunctionBody(body, func(node ast.Node) bool {
		result, ok := node.(*ast.ReturnStmt)
		if ok &&
			result.Pos() > lowerBound &&
			result.End() < upperBound &&
			len(result.Results) == 0 {
			found = true
			return false
		}
		return !found
	})
	return found
}

func storedObjectValue(pass *analysis.Pass, expression ast.Expr, object *types.Var) bool {
	return isObjectValue(pass, expression, object) &&
		!isDirectObjectValue(pass, expression, object)
}

func isDirectObjectValue(pass *analysis.Pass, expression ast.Expr, object *types.Var) bool {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	return ok && pass.TypesInfo.ObjectOf(identifier) == object
}

func isObjectValue(pass *analysis.Pass, expression ast.Expr, object *types.Var) bool {
	switch value := ast.Unparen(expression).(type) {
	case *ast.Ident:
		return pass.TypesInfo.ObjectOf(value) == object
	case *ast.UnaryExpr:
		return isObjectValue(pass, value.X, object)
	case *ast.SelectorExpr:
		return isObjectValue(pass, value.X, object)
	case *ast.CompositeLit:
		for _, element := range value.Elts {
			if isObjectValue(pass, element, object) {
				return true
			}
		}
	case *ast.KeyValueExpr:
		return isObjectValue(pass, value.Key, object) ||
			isObjectValue(pass, value.Value, object)
	}
	return false
}

func referencesObject(pass *analysis.Pass, root ast.Node, object *types.Var) bool {
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if ok && pass.TypesInfo.ObjectOf(identifier) == object {
			found = true
			return false
		}
		return !found
	})
	return found
}

func inspectFunctionBody(body *ast.BlockStmt, visit func(ast.Node) bool) {
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		if _, nested := node.(*ast.FuncLit); nested {
			visit(node)
			return false
		}
		return visit(node)
	})
}
