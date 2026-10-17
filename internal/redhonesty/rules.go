package redhonesty

import (
	"go/ast"
	"regexp"
)

// maxHelperDepth bounds the same-file helper traversal used to decide what a
// test exercises, so a pathological call graph cannot make analysis unbounded.
const maxHelperDepth = 4

// expectedNamePattern matches identifiers conventionally holding the expected
// value of an assertion (want, wantErr, expected, exp, golden, ...).
var expectedNamePattern = regexp.MustCompile(`^(want|wanted|expect|expected|exp|golden)($|[A-Z_0-9])`)

// reach is what a test (plus its bounded same-file helper closure) references.
type reach struct {
	idents    map[string]struct{} // every identifier name
	selectors map[string]struct{} // names selected via x.Name
	buggyWant bool                // an expected-value binding calls a changed func
}

// Evaluate classifies each new test as an honest red/green reproducer or
// rejects it with one or more classes. It performs no I/O and is
// deterministic: verdicts follow the input order and rejections follow the
// fixed order Unanalyzable, UnverifiableBaseline, AlreadyPasses,
// NoChangedBranch, SiblingOnly, BuggyPathExpected, HeadNotGreen.
//
// The structural rules (NoChangedBranch, SiblingOnly, BuggyPathExpected) are
// syntactic heuristics over the test's AST and are skipped when no production
// symbol changed (a tests-only change with no production delta). The behavioural rules
// (AlreadyPasses, UnverifiableBaseline, HeadNotGreen) are exact.
func Evaluate(in EvalInput) []Verdict {
	type parsed struct {
		file *ast.File
		err  bool
	}
	cache := map[string]parsed{}
	parseOnce := func(file string) parsed {
		if p, ok := cache[file]; ok {
			return p
		}
		src, ok := in.Sources[file]
		var p parsed
		if !ok {
			p.err = true
		} else if _, f, err := parseSource(file, src); err != nil {
			p.err = true
		} else {
			p.file = f
		}
		cache[file] = p
		return p
	}

	out := make([]Verdict, 0, len(in.Tests))
	for _, t := range in.Tests {
		v := Verdict{Test: t, Baseline: observed(in.Baseline, t.Key()), Head: observed(in.Head, t.Key())}
		p := parseOnce(t.File)
		var r *reach
		if !p.err {
			r = analyze(p.file, t.Name, in.Symbols)
		}
		if r == nil {
			v.Rejections = append(v.Rejections, RejectUnanalyzable)
		}
		switch v.Baseline {
		case ResultPass:
			v.Rejections = append(v.Rejections, RejectAlreadyPasses)
		case ResultFail:
		default:
			v.Rejections = append(v.Rejections, RejectUnverifiableBaseline)
		}
		if r != nil && !in.Symbols.Empty() {
			if !touches(r.idents, in.Symbols.Funcs, in.Symbols.Types, in.Symbols.Fields) {
				v.Rejections = append(v.Rejections, RejectNoChangedBranch)
			}
			if len(in.Symbols.Fields) > 0 && !touches(r.selectors, in.Symbols.Fields) && touches(r.selectors, in.Symbols.Siblings) {
				v.Rejections = append(v.Rejections, RejectSiblingOnly)
			}
			if r.buggyWant {
				v.Rejections = append(v.Rejections, RejectBuggyPathExpected)
			}
		}
		if v.Head != ResultPass {
			v.Rejections = append(v.Rejections, RejectHeadNotGreen)
		}
		v.Rejections = orderRejections(v.Rejections)
		out = append(out, v)
	}
	return out
}

var rejectionOrder = []RejectionClass{
	RejectUnanalyzable, RejectUnverifiableBaseline, RejectAlreadyPasses, RejectNoChangedBranch,
	RejectSiblingOnly, RejectBuggyPathExpected, RejectHeadNotGreen,
}

// orderRejections returns rs in the fixed canonical order, without duplicates.
func orderRejections(rs []RejectionClass) []RejectionClass {
	if len(rs) == 0 {
		return nil
	}
	present := map[RejectionClass]bool{}
	for _, r := range rs {
		present[r] = true
	}
	out := make([]RejectionClass, 0, len(rs))
	for _, c := range rejectionOrder {
		if present[c] {
			out = append(out, c)
		}
	}
	return out
}

func observed(m map[string]Result, key string) Result {
	if r, ok := m[key]; ok {
		return r
	}
	return ResultUnrunnable
}

func touches(have map[string]struct{}, sets ...map[string]struct{}) bool {
	for _, s := range sets {
		for n := range have {
			if _, ok := s[n]; ok {
				return true
			}
		}
	}
	return false
}

// analyze collects what the named test references, following calls to
// same-file top-level functions up to maxHelperDepth. It returns nil when the
// test function is not present in the file.
func analyze(f *ast.File, name string, syms Symbols) *reach {
	funcs := map[string]*ast.FuncDecl{}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
			funcs[fd.Name.Name] = fd
		}
	}
	root, ok := funcs[name]
	if !ok {
		return nil
	}
	r := &reach{idents: map[string]struct{}{}, selectors: map[string]struct{}{}}
	visited := map[string]bool{name: true}
	var walk func(fd *ast.FuncDecl, depth int)
	walk = func(fd *ast.FuncDecl, depth int) {
		var callees []string
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				r.idents[n.Name] = struct{}{}
				if _, isFn := funcs[n.Name]; isFn {
					callees = append(callees, n.Name)
				}
			case *ast.SelectorExpr:
				r.selectors[n.Sel.Name] = struct{}{}
			case *ast.AssignStmt:
				for i, lhs := range n.Lhs {
					if i < len(n.Rhs) && bindsExpected(lhs) && callsChanged(n.Rhs[i], syms.Funcs) {
						r.buggyWant = true
					}
				}
			case *ast.ValueSpec:
				for i, id := range n.Names {
					if i < len(n.Values) && bindsExpected(id) && callsChanged(n.Values[i], syms.Funcs) {
						r.buggyWant = true
					}
				}
			case *ast.KeyValueExpr:
				if bindsExpected(n.Key) && callsChanged(n.Value, syms.Funcs) {
					r.buggyWant = true
				}
			}
			return true
		})
		if depth >= maxHelperDepth {
			return
		}
		for _, c := range callees {
			if !visited[c] {
				visited[c] = true
				walk(funcs[c], depth+1)
			}
		}
	}
	walk(root, 0)
	return r
}

func bindsExpected(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && expectedNamePattern.MatchString(id.Name)
}

// callsChanged reports whether expr contains a call to a changed function or
// method (matched by bare name).
func callsChanged(expr ast.Expr, changed map[string]struct{}) bool {
	if len(changed) == 0 {
		return false
	}
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return !found
		}
		var name string
		switch fn := ce.Fun.(type) {
		case *ast.Ident:
			name = fn.Name
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		}
		if _, hit := changed[name]; hit {
			found = true
		}
		return !found
	})
	return found
}
