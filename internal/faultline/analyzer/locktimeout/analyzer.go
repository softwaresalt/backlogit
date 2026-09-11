// Package locktimeout declares the FL005 timeout-vs-uncancellable-lock analyzer.
package locktimeout

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	diagnostic           = "FL005: deadline/timeout context in scope but lock acquired via a non-ctx (uncancellable) call"
	suppressionDirective = "// faultline:lock-nonctx-ok"
)

// Analyzer declares the FL005 timeout-vs-uncancellable-lock analyzer.
var Analyzer = &analysis.Analyzer{
	Name: "FL005locktimeout",
	Doc:  "reports uncancellable lock acquisitions made while a timeout context is in scope",
	Run:  run,
}

type timeoutClaim struct {
	block       *ast.BlockStmt
	call        *ast.CallExpr
	contextVar  *types.Var
	contextType types.Type
}

type lockAcquisition struct {
	block *ast.BlockStmt
	call  *ast.CallExpr
}

func run(pass *analysis.Pass) (any, error) {
	contextType := importedContextType(pass)
	if contextType == nil {
		return nil, nil
	}

	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch function := node.(type) {
			case *ast.FuncDecl:
				analyzeFunction(pass, file, function.Body, contextType)
			case *ast.FuncLit:
				analyzeFunction(pass, file, function.Body, contextType)
			}
			return true
		})
	}

	return nil, nil
}

func analyzeFunction(
	pass *analysis.Pass,
	file *ast.File,
	body *ast.BlockStmt,
	contextType types.Type,
) {
	if body == nil {
		return
	}

	parents := functionParents(body)
	claims := make([]timeoutClaim, 0, 1)
	acquisitions := make([]lockAcquisition, 0, 1)

	inspectFunctionBody(body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		if isTimeoutSource(pass, call) {
			if variable := timeoutResultVariable(pass, parents, call, contextType); variable != nil {
				claims = append(claims, timeoutClaim{
					block:       containingBlock(parents, call),
					call:        call,
					contextVar:  variable,
					contextType: contextType,
				})
			}
		}

		if isUncancellableLock(pass, call, contextType) {
			acquisitions = append(acquisitions, lockAcquisition{
				block: containingBlock(parents, call),
				call:  call,
			})
		}
		return true
	})

	for _, acquisition := range acquisitions {
		if isSuppressed(pass, file, parents, acquisition.call) {
			continue
		}
		for _, claim := range claims {
			if claimReachesAcquisition(claim, acquisition) {
				pass.Reportf(acquisition.call.Pos(), diagnostic)
				break
			}
		}
	}
}

func importedContextType(pass *analysis.Pass) types.Type {
	for _, imported := range pass.Pkg.Imports() {
		if imported.Path() != "context" {
			continue
		}
		object := imported.Scope().Lookup("Context")
		if object != nil {
			return object.Type()
		}
	}
	return nil
}

func isTimeoutSource(pass *analysis.Pass, call *ast.CallExpr) bool {
	function := calledFunction(pass, call)
	if function == nil || function.Pkg() == nil {
		return false
	}

	for _, source := range timeoutSources {
		if function.Pkg().Path() == source.packagePath &&
			function.Name() == source.function {
			return true
		}
	}
	return false
}

func calledFunction(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	switch function := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		object, _ := pass.TypesInfo.ObjectOf(function).(*types.Func)
		return object
	case *ast.SelectorExpr:
		object, _ := pass.TypesInfo.ObjectOf(function.Sel).(*types.Func)
		return object
	default:
		return nil
	}
}

func timeoutResultVariable(
	pass *analysis.Pass,
	parents map[ast.Node]ast.Node,
	call *ast.CallExpr,
	contextType types.Type,
) *types.Var {
	for parent := parents[call]; parent != nil; parent = parents[parent] {
		switch binding := parent.(type) {
		case *ast.ParenExpr:
			continue
		case *ast.AssignStmt:
			if len(binding.Rhs) != 1 ||
				ast.Unparen(binding.Rhs[0]) != call ||
				len(binding.Lhs) == 0 {
				return nil
			}
			return localContextVariable(pass, binding.Lhs[0], contextType)
		case *ast.ValueSpec:
			if len(binding.Values) != 1 ||
				ast.Unparen(binding.Values[0]) != call ||
				len(binding.Names) == 0 {
				return nil
			}
			return localContextVariable(pass, binding.Names[0], contextType)
		default:
			return nil
		}
	}
	return nil
}

func localContextVariable(
	pass *analysis.Pass,
	expression ast.Expr,
	contextType types.Type,
) *types.Var {
	identifier, ok := ast.Unparen(expression).(*ast.Ident)
	if !ok || identifier.Name == "_" {
		return nil
	}

	variable, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
	if !ok ||
		variable.Pkg() != pass.Pkg ||
		variable.Parent() == nil ||
		variable.Parent() == pass.Pkg.Scope() ||
		!types.AssignableTo(variable.Type(), contextType) {
		return nil
	}
	return variable
}

func isUncancellableLock(
	pass *analysis.Pass,
	call *ast.CallExpr,
	contextType types.Type,
) bool {
	selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok || pass.TypesInfo.Selections[selector] == nil {
		return false
	}

	signature, ok := pass.TypesInfo.TypeOf(call.Fun).Underlying().(*types.Signature)
	if !ok || signatureTakesContext(signature, contextType) {
		return false
	}

	for _, sink := range uncancellableLockSinks {
		if selector.Sel.Name != sink.selector {
			continue
		}
		if sink.anyReceiver || matchesReceiver(pass, selector, sink) {
			return true
		}
	}
	return false
}

func signatureTakesContext(signature *types.Signature, contextType types.Type) bool {
	parameters := signature.Params()
	for index := range parameters.Len() {
		parameterType := parameters.At(index).Type()
		if signature.Variadic() && index == parameters.Len()-1 {
			if slice, ok := parameterType.Underlying().(*types.Slice); ok {
				parameterType = slice.Elem()
			}
		}
		if types.AssignableTo(parameterType, contextType) {
			return true
		}
	}
	return false
}

func matchesReceiver(pass *analysis.Pass, selector *ast.SelectorExpr, sink lockSink) bool {
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
	if pointer, ok := receiver.(*types.Pointer); ok {
		receiver = types.Unalias(pointer.Elem())
	}
	named, ok := receiver.(*types.Named)
	return ok &&
		named.Obj().Pkg() != nil &&
		named.Obj().Pkg().Path() == sink.receiverPackage &&
		named.Obj().Name() == sink.receiverName
}

func claimReachesAcquisition(claim timeoutClaim, acquisition lockAcquisition) bool {
	if claim.block == nil ||
		claim.block != acquisition.block ||
		claim.call.End() >= acquisition.call.Pos() ||
		claim.contextVar.Parent() == nil ||
		!claim.contextVar.Parent().Contains(acquisition.call.Pos()) {
		return false
	}
	return types.AssignableTo(claim.contextVar.Type(), claim.contextType)
}

func isSuppressed(
	pass *analysis.Pass,
	file *ast.File,
	parents map[ast.Node]ast.Node,
	call *ast.CallExpr,
) bool {
	statement := containingStatement(parents, call)
	if statement == nil {
		return false
	}

	startLine := pass.Fset.Position(statement.Pos()).Line
	endLine := pass.Fset.Position(statement.End()).Line
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.TrimSpace(comment.Text) != suppressionDirective {
				continue
			}

			line := pass.Fset.Position(comment.Pos()).Line
			if line == endLine && comment.Pos() >= statement.End() {
				return true
			}
			if line == startLine-1 && isDedicatedComment(pass, file, comment) {
				return true
			}
		}
	}
	return false
}

func containingStatement(parents map[ast.Node]ast.Node, node ast.Node) ast.Stmt {
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		if statement, ok := parent.(ast.Stmt); ok {
			return statement
		}
	}
	return nil
}

func containingBlock(parents map[ast.Node]ast.Node, node ast.Node) *ast.BlockStmt {
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		if block, ok := parent.(*ast.BlockStmt); ok {
			return block
		}
	}
	return nil
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

func functionParents(body *ast.BlockStmt) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	stack := make([]ast.Node, 0, 16)
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if _, nested := node.(*ast.FuncLit); nested {
			if len(stack) > 0 {
				parents[node] = stack[len(stack)-1]
			}
			return false
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func inspectFunctionBody(body *ast.BlockStmt, visit func(ast.Node) bool) {
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			return true
		}
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		return visit(node)
	})
}
