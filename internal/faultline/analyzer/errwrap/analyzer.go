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
)

const (
	diagnostic           = "FL002: error argument to fmt.Errorf should use %w, not %v/%s"
	suppressionDirective = "// faultline:errwrap-ok"
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
	var object types.Object
	switch function := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		object = pass.TypesInfo.ObjectOf(function)
	case *ast.SelectorExpr:
		object = pass.TypesInfo.ObjectOf(function.Sel)
	}

	errorf, ok := object.(*types.Func)
	return ok &&
		errorf.Pkg() != nil &&
		errorf.Pkg().Path() == "fmt" &&
		errorf.Name() == "Errorf"
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
	for _, use := range parseFormat(format) {
		if use.argument < 0 || use.argument >= len(call.Args)-1 {
			continue
		}
		if use.verb != 'v' && use.verb != 's' {
			continue
		}

		argument := call.Args[use.argument+1]
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

func isSuppressed(pass *analysis.Pass, file *ast.File, call *ast.CallExpr) bool {
	startLine := pass.Fset.Position(call.Pos()).Line
	endLine := pass.Fset.Position(call.End()).Line
	for _, group := range file.Comments {
		for _, comment := range group.List {
			line := pass.Fset.Position(comment.Pos()).Line
			if (line == startLine-1 || line >= startLine && line <= endLine) &&
				strings.TrimSpace(comment.Text) == suppressionDirective {
				return true
			}
		}
	}
	return false
}

func parseFormat(format string) []formatUse {
	uses := make([]formatUse, 0, strings.Count(format, "%"))
	nextArgument := 0

	for offset := 0; offset < len(format); {
		percent := strings.IndexByte(format[offset:], '%')
		if percent < 0 {
			break
		}
		offset += percent + 1
		if offset >= len(format) {
			break
		}
		if format[offset] == '%' {
			offset++
			continue
		}

		explicitArgument := -1
		if index, next, ok := parseIndex(format, offset); ok {
			explicitArgument = index
			offset = next
		}

		offset = skipFlags(format, offset)
		if explicitArgument < 0 {
			if index, next, ok := parseIndex(format, offset); ok {
				explicitArgument = index
				offset = next
			}
		}

		if offset < len(format) && format[offset] == '*' {
			consumeArgument(explicitArgument, &nextArgument)
			explicitArgument = -1
			offset++
		} else {
			offset = skipDigits(format, offset)
		}

		if offset < len(format) && format[offset] == '.' {
			offset++
			precisionArgument := -1
			if index, next, ok := parseIndex(format, offset); ok {
				precisionArgument = index
				offset = next
			}
			if offset < len(format) && format[offset] == '*' {
				consumeArgument(precisionArgument, &nextArgument)
				offset++
			} else {
				offset = skipDigits(format, offset)
				if precisionArgument >= 0 {
					explicitArgument = precisionArgument
				}
			}
		}

		if index, next, ok := parseIndex(format, offset); ok {
			explicitArgument = index
			offset = next
		}
		if offset >= len(format) {
			break
		}

		verb, size := utf8.DecodeRuneInString(format[offset:])
		offset += size
		if verb == '%' {
			continue
		}

		argument := explicitArgument
		if argument < 0 {
			argument = nextArgument
		}
		nextArgument = argument + 1
		uses = append(uses, formatUse{argument: argument, verb: verb})
	}

	return uses
}

func parseIndex(format string, offset int) (int, int, bool) {
	if offset >= len(format) || format[offset] != '[' {
		return 0, offset, false
	}

	end := offset + 1
	for end < len(format) && format[end] >= '0' && format[end] <= '9' {
		end++
	}
	if end == offset+1 || end >= len(format) || format[end] != ']' {
		return 0, offset, false
	}

	index, err := strconv.Atoi(format[offset+1 : end])
	if err != nil || index < 1 {
		return 0, offset, false
	}
	return index - 1, end + 1, true
}

func skipFlags(format string, offset int) int {
	for offset < len(format) && strings.ContainsRune("#0+- '", rune(format[offset])) {
		offset++
	}
	return offset
}

func skipDigits(format string, offset int) int {
	for offset < len(format) && format[offset] >= '0' && format[offset] <= '9' {
		offset++
	}
	return offset
}

func consumeArgument(explicit int, next *int) {
	if explicit >= 0 {
		*next = explicit + 1
		return
	}
	*next++
}
