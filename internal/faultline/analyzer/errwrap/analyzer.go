// Package errwrap declares the FL002 error-wrapping analyzer.
package errwrap

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/astutil"
)

const (
	diagnostic           = "FL002: error argument to fmt.Errorf should use %w, not %v/%s"
	suppressionDirective = "// faultline:errwrap-ok"
	maxFormatNumber      = 1_000_000
)

// Analyzer declares the FL002 error-wrapping analyzer.
var Analyzer = &analysis.Analyzer{
	Name: "FL002errwrap",
	Doc:  "reports error arguments formatted without preserving their error chain",
	Run:  run,
}

type formatUse struct {
	argument int
	verb     rune
}

type formatParser struct {
	format       string
	numArguments int
	offset       int
	nextArgument int
}

func run(pass *analysis.Pass) (any, error) {
	errorInterface := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

	for _, file := range pass.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || !isErrorfCall(pass, call) || isSuppressed(pass, file, call) {
				return true
			}

			reportBadErrorFormats(pass, call, errorInterface)
			return true
		})
	}

	return nil, nil
}

func isErrorfCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	function := calledFunction(pass, call)
	return function != nil &&
		function.Pkg() != nil &&
		function.Pkg().Path() == "fmt" &&
		function.Name() == "Errorf"
}

func calledFunction(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	var object types.Object
	switch function := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		object = pass.TypesInfo.ObjectOf(function)
	case *ast.SelectorExpr:
		object = pass.TypesInfo.ObjectOf(function.Sel)
	}

	function, _ := object.(*types.Func)
	return function
}

func reportBadErrorFormats(pass *analysis.Pass, call *ast.CallExpr, errorInterface *types.Interface) {
	if len(call.Args) < 2 {
		return
	}

	literal, ok := ast.Unparen(call.Args[0]).(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return
	}
	format, err := strconv.Unquote(literal.Value)
	if err != nil {
		return
	}

	reported := make(map[int]struct{})
	for _, use := range parseFormat(format, len(call.Args)-1) {
		if use.verb != 'v' && use.verb != 's' {
			continue
		}

		argument := call.Args[use.argument+1]
		if isErrorsNewCall(pass, argument) {
			continue
		}
		switch ast.Unparen(argument).(type) {
		case *ast.Ident, *ast.CallExpr:
		default:
			continue
		}
		argumentType := pass.TypesInfo.TypeOf(argument)
		if argumentType == nil || !types.Implements(argumentType, errorInterface) {
			continue
		}
		if _, alreadyReported := reported[use.argument]; alreadyReported {
			continue
		}

		pass.Report(analysis.Diagnostic{
			Pos:     argument.Pos(),
			Message: diagnostic,
		})
		reported[use.argument] = struct{}{}
	}
}

func isErrorsNewCall(pass *analysis.Pass, expression ast.Expr) bool {
	call, ok := ast.Unparen(expression).(*ast.CallExpr)
	if !ok {
		return false
	}

	function := calledFunction(pass, call)
	return function != nil &&
		function.Pkg() != nil &&
		function.Pkg().Path() == "errors" &&
		function.Name() == "New"
}

func isSuppressed(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) bool {
	statement, direct := enclosingCallStatement(file, call)
	if statement == nil {
		return false
	}

	start := statement.Pos()
	end := statement.End()
	if !direct {
		start = call.Pos()
		end = call.End()
	}
	startLine := pass.Fset.Position(start).Line
	endLine := pass.Fset.Position(end).Line
	statementStartLine := pass.Fset.Position(statement.Pos()).Line
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if strings.TrimSpace(comment.Text) != suppressionDirective {
				continue
			}

			line := pass.Fset.Position(comment.Pos()).Line
			sameLine := line == endLine && comment.Pos() >= end
			if !direct {
				sameLine = sameLine && comment.Pos() < statement.End()
			}
			if sameLine {
				return true
			}
			precedingLine := line == startLine-1
			if !direct {
				precedingLine = precedingLine && startLine > statementStartLine
			}
			if precedingLine && isDedicatedComment(pass, file, comment) {
				return true
			}
		}
	}
	return false
}

func enclosingCallStatement(file *ast.File, call *ast.CallExpr) (ast.Stmt, bool) {
	path, _ := astutil.PathEnclosingInterval(file, call.Pos(), call.End())
	for _, node := range path {
		statement, ok := node.(ast.Stmt)
		if !ok {
			continue
		}
		return statement, statementDirectlyUsesCall(statement, call)
	}
	return nil, false
}

func statementDirectlyUsesCall(statement ast.Stmt, call *ast.CallExpr) bool {
	isCall := func(expression ast.Expr) bool {
		return ast.Unparen(expression) == call
	}
	containsCall := func(expressions []ast.Expr) bool {
		for _, expression := range expressions {
			if isCall(expression) {
				return true
			}
		}
		return false
	}

	switch statement := statement.(type) {
	case *ast.AssignStmt:
		return containsCall(statement.Rhs)
	case *ast.DeclStmt:
		declaration, ok := statement.Decl.(*ast.GenDecl)
		if !ok {
			return false
		}
		for _, spec := range declaration.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if ok && containsCall(value.Values) {
				return true
			}
		}
	case *ast.DeferStmt:
		return statement.Call == call
	case *ast.ExprStmt:
		return isCall(statement.X)
	case *ast.GoStmt:
		return statement.Call == call
	case *ast.ReturnStmt:
		return containsCall(statement.Results)
	case *ast.SendStmt:
		return isCall(statement.Value)
	}
	return false
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

func parseFormat(format string, numArguments int) []formatUse {
	uses := make([]formatUse, 0, strings.Count(format, "%"))
	parser := formatParser{
		format:       format,
		numArguments: numArguments,
	}

	for parser.offset < len(format) {
		percent := strings.IndexByte(format[parser.offset:], '%')
		if percent < 0 {
			break
		}
		parser.offset += percent + 1
		if parser.offset >= len(format) {
			break
		}

		if use, ok := parser.parseDirective(); ok {
			uses = append(uses, use)
		}
	}

	return uses
}

func (parser *formatParser) parseDirective() (formatUse, bool) {
	parser.offset = skipFlags(parser.format, parser.offset)

	argument, afterIndex, valid := parser.parseArgumentNumber(parser.nextArgument)

	if parser.offset < len(parser.format) && parser.format[parser.offset] == '*' {
		parser.offset++
		argument = parser.consumeArgument(argument)
		afterIndex = false
	} else {
		var widthPresent bool
		_, widthPresent, parser.offset = parseNumber(parser.format, parser.offset, len(parser.format))
		if afterIndex && widthPresent {
			valid = false
		}
	}

	if parser.offset+1 < len(parser.format) && parser.format[parser.offset] == '.' {
		parser.offset++
		if afterIndex {
			valid = false
		}

		var indexValid bool
		argument, afterIndex, indexValid = parser.parseArgumentNumber(argument)
		valid = valid && indexValid
		if parser.offset < len(parser.format) && parser.format[parser.offset] == '*' {
			parser.offset++
			argument = parser.consumeArgument(argument)
			afterIndex = false
		} else {
			_, _, parser.offset = parseNumber(parser.format, parser.offset, len(parser.format))
		}
	}

	if !afterIndex {
		var indexValid bool
		argument, _, indexValid = parser.parseArgumentNumber(argument)
		valid = valid && indexValid
	}

	parser.nextArgument = argument
	if parser.offset >= len(parser.format) {
		return formatUse{}, false
	}

	verb, size := utf8.DecodeRuneInString(parser.format[parser.offset:])
	parser.offset += size
	if verb == '%' || !valid || argument >= parser.numArguments {
		return formatUse{}, false
	}

	parser.nextArgument++
	return formatUse{argument: argument, verb: verb}, true
}

func (parser *formatParser) parseArgumentNumber(argument int) (int, bool, bool) {
	if parser.offset >= len(parser.format) || parser.format[parser.offset] != '[' {
		return argument, false, true
	}

	closeOffset := strings.IndexByte(parser.format[parser.offset+1:], ']')
	if closeOffset < 0 {
		parser.offset++
		return argument, false, false
	}
	closeOffset += parser.offset + 1

	index, ok, end := parseNumber(parser.format, parser.offset+1, closeOffset)
	parser.offset = closeOffset + 1
	if !ok || end != closeOffset {
		return argument, false, false
	}

	index--
	if index < 0 || index >= parser.numArguments {
		return argument, true, false
	}
	return index, true, true
}

func (parser *formatParser) consumeArgument(argument int) int {
	if argument < parser.numArguments {
		return argument + 1
	}
	return argument
}

func parseNumber(format string, start, end int) (number int, present bool, next int) {
	if start >= end {
		return 0, false, end
	}

	next = start
	for next < end && format[next] >= '0' && format[next] <= '9' {
		if number > maxFormatNumber {
			return 0, false, end
		}
		number = number*10 + int(format[next]-'0')
		present = true
		next++
	}
	return number, present, next
}

func skipFlags(format string, offset int) int {
	for offset < len(format) && strings.ContainsRune("#0+- ", rune(format[offset])) {
		offset++
	}
	return offset
}
