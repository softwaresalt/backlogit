// Package locktimeout declares the FL005 timeout-vs-uncancellable-lock analyzer.
package locktimeout

import (
	"go/ast"
	"go/token"
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

type directAssignment struct {
	container ast.Node
	statement *ast.AssignStmt
	variable  *types.Var
	value     ast.Expr
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
	assignments := make([]directAssignment, 0, 1)

	inspectFunctionBody(body, func(node ast.Node) bool {
		if assignment, ok := node.(*ast.AssignStmt); ok {
			assignments = append(assignments, directAssignments(pass, parents, assignment)...)
		}

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
			if claimReachesAcquisition(pass, parents, claim, acquisition, assignments) {
				pass.Reportf(acquisition.call.Pos(), diagnostic)
				break
			}
		}
	}
}

func directAssignments(
	pass *analysis.Pass,
	parents map[ast.Node]ast.Node,
	statement *ast.AssignStmt,
) []directAssignment {
	if len(statement.Lhs) != len(statement.Rhs) {
		return nil
	}

	assignments := make([]directAssignment, 0, len(statement.Lhs))
	for index, expression := range statement.Lhs {
		identifier, ok := ast.Unparen(expression).(*ast.Ident)
		if !ok || identifier.Name == "_" {
			continue
		}
		variable, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Var)
		if !ok {
			continue
		}
		assignments = append(assignments, directAssignment{
			container: statementListContainer(parents, statement),
			statement: statement,
			variable:  variable,
			value:     statement.Rhs[index],
		})
	}
	return assignments
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
	if !ok {
		return false
	}

	selection := pass.TypesInfo.Selections[selector]
	if selection == nil {
		return false
	}
	method, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	signature, ok := method.Type().(*types.Signature)
	if !ok ||
		signature.Recv() == nil ||
		callSuppliesContext(call, signature, contextType) {
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

func callSuppliesContext(
	call *ast.CallExpr,
	signature *types.Signature,
	contextType types.Type,
) bool {
	parameters := signature.Params()
	for index := range parameters.Len() {
		parameterType := parameters.At(index).Type()
		if signature.Variadic() && index == parameters.Len()-1 {
			if slice, ok := parameterType.Underlying().(*types.Slice); ok {
				parameterType = slice.Elem()
			}
		}
		if types.AssignableTo(parameterType, contextType) && index < len(call.Args) {
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

func claimReachesAcquisition(
	pass *analysis.Pass,
	parents map[ast.Node]ast.Node,
	claim timeoutClaim,
	acquisition lockAcquisition,
	assignments []directAssignment,
) bool {
	if claim.block == nil ||
		!sameOrDescendantBlock(parents, claim.block, acquisition.block) ||
		claim.call.End() >= acquisition.call.Pos() ||
		!variableVisibleAt(pass, claim.contextVar, acquisition.call.Pos()) ||
		claimEndedBeforeAcquisition(pass, parents, claim, acquisition, assignments) {
		return false
	}
	return types.AssignableTo(claim.contextVar.Type(), claim.contextType)
}

func sameOrDescendantBlock(
	parents map[ast.Node]ast.Node,
	ancestor *ast.BlockStmt,
	descendant *ast.BlockStmt,
) bool {
	for block := descendant; block != nil; block = containingBlock(parents, block) {
		if block == ancestor {
			return true
		}
	}
	return false
}

func variableVisibleAt(pass *analysis.Pass, variable *types.Var, position token.Pos) bool {
	scope := innermostScopeAt(pass, position)
	if scope == nil {
		return variable.Parent() != nil && variable.Parent().Contains(position)
	}
	_, object := scope.LookupParent(variable.Name(), position)
	return object == variable
}

func innermostScopeAt(pass *analysis.Pass, position token.Pos) *types.Scope {
	var innermost *types.Scope
	for _, scope := range pass.TypesInfo.Scopes {
		if !scope.Contains(position) {
			continue
		}
		if innermost == nil || scopeDescendsFrom(scope, innermost) {
			innermost = scope
		}
	}
	return innermost
}

func scopeDescendsFrom(scope *types.Scope, ancestor *types.Scope) bool {
	for current := scope.Parent(); current != nil; current = current.Parent() {
		if current == ancestor {
			return true
		}
	}
	return false
}

func claimEndedBeforeAcquisition(
	pass *analysis.Pass,
	parents map[ast.Node]ast.Node,
	claim timeoutClaim,
	acquisition lockAcquisition,
	assignments []directAssignment,
) bool {
	sourceStatement := containingBlockStatement(parents, claim.call)
	if sourceStatement == nil || parents[sourceStatement] != claim.block {
		return false
	}

	for _, assignment := range assignments {
		if assignment.variable != claim.contextVar ||
			assignment.container == nil ||
			!hasAncestor(acquisition.call, assignment.container, parents) ||
			assignment.statement.Pos() <= claim.call.End() ||
			assignment.statement.End() >= acquisition.call.Pos() ||
			expressionReferencesVariable(pass, assignment.value, claim.contextVar) {
			continue
		}
		return true
	}
	return false
}

func statementListContainer(
	parents map[ast.Node]ast.Node,
	statement ast.Stmt,
) ast.Node {
	switch parent := parents[statement].(type) {
	case *ast.BlockStmt:
		return parent
	case *ast.CaseClause:
		return parent
	case *ast.CommClause:
		return parent
	default:
		return nil
	}
}

func hasAncestor(
	node ast.Node,
	ancestor ast.Node,
	parents map[ast.Node]ast.Node,
) bool {
	for current := parents[node]; current != nil; current = parents[current] {
		if current == ancestor {
			return true
		}
	}
	return false
}

func expressionReferencesVariable(
	pass *analysis.Pass,
	expression ast.Expr,
	variable *types.Var,
) bool {
	references := false
	ast.Inspect(expression, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok || pass.TypesInfo.ObjectOf(identifier) != variable {
			return true
		}
		references = true
		return false
	})
	return references
}

func isSuppressed(
	pass *analysis.Pass,
	file *ast.File,
	parents map[ast.Node]ast.Node,
	call *ast.CallExpr,
) bool {
	statement := containingBlockStatement(parents, call)
	if statement == nil {
		return false
	}
	block := containingBlock(parents, statement)
	if block == nil {
		return false
	}

	startLine := pass.Fset.Position(statement.Pos()).Line
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.TrimSpace(comment.Text) != suppressionDirective {
				continue
			}
			line := pass.Fset.Position(comment.Pos()).Line
			if line == pass.Fset.Position(call.End()).Line &&
				trailingStatement(pass, block, comment) == statement &&
				nearestCallBefore(statement, comment) == call {
				return true
			}
			if line == pass.Fset.Position(call.Pos()).Line-1 &&
				line == startLine-1 &&
				isDedicatedComment(pass, file, comment) &&
				followingStatement(pass, block, comment) == statement &&
				nearestCallAfter(statement, comment) == call {
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
		if candidate.Pos() > comment.Pos() ||
			(candidate.End() < comment.Pos() &&
				pass.Fset.Position(candidate.End()).Line != commentLine) {
			continue
		}
		if owner == nil || candidate.Pos() > owner.Pos() {
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

func nearestCallBefore(statement ast.Stmt, comment *ast.Comment) *ast.CallExpr {
	var nearest *ast.CallExpr
	ast.Inspect(statement, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || call.End() > comment.Pos() {
			return true
		}
		if nearest == nil || call.End() > nearest.End() {
			nearest = call
		}
		return true
	})
	return nearest
}

func nearestCallAfter(statement ast.Stmt, comment *ast.Comment) *ast.CallExpr {
	var nearest *ast.CallExpr
	ast.Inspect(statement, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || call.Pos() < comment.End() {
			return true
		}
		if nearest == nil || call.Pos() < nearest.Pos() {
			nearest = call
		}
		return true
	})
	return nearest
}

func containingBlockStatement(parents map[ast.Node]ast.Node, node ast.Node) ast.Stmt {
	var owner ast.Stmt
	for parent := parents[node]; parent != nil; parent = parents[parent] {
		if _, ok := parent.(*ast.BlockStmt); ok {
			return owner
		}
		if statement, ok := parent.(ast.Stmt); ok {
			owner = statement
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
