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
var expectedNamePattern = regexp.MustCompile(`^(?i:want|wanted|expect|expected|exp|golden)($|[A-Z_0-9])`)

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
	changing := changingHelpers(funcs, syms.Funcs)
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
					if !bindsExpected(lhs) {
						continue
					}
					// `a, want := f()` has one call feeding several results.
					if rhs, ok := pairedValue(n.Rhs, i, len(n.Lhs)); ok && callsChanged(rhs, syms.Funcs, changing) {
						r.buggyWant = true
					}
				}
			case *ast.ValueSpec:
				for i, id := range n.Names {
					if !bindsExpected(id) {
						continue
					}
					if v, ok := pairedValue(n.Values, i, len(n.Names)); ok && callsChanged(v, syms.Funcs, changing) {
						r.buggyWant = true
					}
				}
			case *ast.KeyValueExpr:
				if bindsExpected(n.Key) && callsChanged(n.Value, syms.Funcs, changing) {
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

// pairedValue returns the right-hand expression that feeds the i-th of n
// left-hand bindings: the i-th value when there are as many values as bindings,
// or the sole value when one expression (a multi-result call) feeds them all.
func pairedValue(values []ast.Expr, i, n int) (ast.Expr, bool) {
	switch {
	case len(values) == n && i < len(values):
		return values[i], true
	case len(values) == 1:
		return values[0], true
	}
	return nil, false
}

func bindsExpected(e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.Ident:
		return expectedNamePattern.MatchString(e.Name)
	case *ast.SelectorExpr: // tc.want = ..., got.expected = ...
		return expectedNamePattern.MatchString(e.Sel.Name)
	}
	return false
}

// calleeName returns the bare name of the called function, looking through
// parentheses and explicit generic instantiation (f[int](x), f[int, string](x)).
func calleeName(fun ast.Expr) string {
	for {
		switch f := fun.(type) {
		case *ast.ParenExpr:
			fun = f.X
		case *ast.IndexExpr:
			fun = f.X
		case *ast.IndexListExpr:
			fun = f.X
		case *ast.Ident:
			return f.Name
		case *ast.SelectorExpr:
			return f.Sel.Name
		default:
			return ""
		}
	}
}

// callsChanged reports whether expr contains a call to a changed function or
// method, or to a same-file helper that (transitively) calls one. Matching is by
// bare name.
func callsChanged(expr ast.Node, changed map[string]struct{}, changing map[string]bool) bool {
	if len(changed) == 0 {
		return false
	}
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		ce, ok := n.(*ast.CallExpr)
		if !ok {
			return !found
		}
		name := calleeName(ce.Fun)
		if _, hit := changed[name]; hit || changing[name] {
			found = true
		}
		return !found
	})
	return found
}

// changingHelpers returns the same-file functions whose bodies call a changed
// function, directly or through other such helpers (fixpoint bounded by
// maxHelperDepth rounds).
func changingHelpers(funcs map[string]*ast.FuncDecl, changed map[string]struct{}) map[string]bool {
	out := map[string]bool{}
	if len(changed) == 0 {
		return out
	}
	for round := 0; round < maxHelperDepth; round++ {
		// Collect this round's discoveries and publish them afterwards, so the
		// result never depends on map iteration order: after k rounds exactly the
		// helpers within k call levels of a changed function are included.
		var found []string
		for name, fd := range funcs {
			if out[name] {
				continue
			}
			if callsChanged(fd.Body, changed, out) {
				found = append(found, name)
			}
		}
		if len(found) == 0 {
			break
		}
		for _, name := range found {
			out[name] = true
		}
	}
	return out
}
