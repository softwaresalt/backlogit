package redhonesty

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func isTestdata(p string) bool {
	return strings.HasPrefix(p, "testdata/") || strings.Contains(p, "/testdata/")
}

// isTestFile reports a real Go test file. Anything under testdata is fixture
// data the go tool ignores, never a test or production source.
func isTestFile(p string) bool { return strings.HasSuffix(p, "_test.go") && !isTestdata(p) }

// isGoSource reports a Go source file the go tool compiles (testdata excluded).
func isGoSource(p string) bool { return strings.HasSuffix(p, ".go") && !isTestdata(p) }

// isTestName reports whether name is a valid Go test function name: "Test"
// followed by nothing or a rune that is not a lower-case letter.
func isTestName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok {
		return false
	}
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return !unicode.IsLower(r)
}

// isTestSignature reports whether fd has the shape func(*testing.T) the way the
// go tool discovers tests: the parameter is a pointer to a type named T, either
// qualified (testing.T) or not (a dot import or a `type T = testing.T` alias).
// A type that is not really *testing.T is rejected later by the compiler.
func isTestSignature(fd *ast.FuncDecl) bool {
	// An empty but non-nil result list (`func TestX(t *testing.T) ()`) is
	// syntactically distinct from an absent one yet declares no results, so
	// only a non-empty list disqualifies the function.
	if fd.Recv != nil || fd.Type.TypeParams != nil ||
		(fd.Type.Results != nil && len(fd.Type.Results.List) > 0) {
		return false
	}
	params := fd.Type.Params
	if params == nil || len(params.List) != 1 || len(params.List[0].Names) > 1 {
		return false
	}
	star, ok := params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	switch x := star.X.(type) {
	case *ast.SelectorExpr:
		if x.Sel.Name != "T" {
			return false
		}
		_, ok = x.X.(*ast.Ident)
		return ok
	case *ast.Ident:
		return x.Name == "T"
	}
	return false
}

func parseSource(file string, src []byte) (*token.FileSet, *ast.File, error) {
	if len(src) > maxSourceBytes {
		return nil, nil, fmt.Errorf("redhonesty: %s exceeds %d bytes", file, maxSourceBytes)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, fmt.Errorf("redhonesty: parse %s: %w", file, err)
	}
	return fset, f, nil
}

// testFuncNames returns the top-level test functions declared in src mapped to
// the Go package clause of the file (so internal and external test packages are
// distinguishable).
func testFuncNames(file string, src []byte) (map[string]string, error) {
	out := map[string]string{}
	if len(src) == 0 {
		return out, nil
	}
	_, f, err := parseSource(file, src)
	if err != nil {
		return nil, err
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && isTestName(fd.Name.Name) && isTestSignature(fd) {
			out[fd.Name.Name] = f.Name.Name
		}
	}
	return out, nil
}

// ambiguousPkg marks a base test name declared in more than one Go package.
const ambiguousPkg = "\x00ambiguous"

// IdentifyNewTests returns the top-level test functions present at head that
// were not present at base in the same package directory (so a test moved
// between files of one package is not reported as new). The result is sorted
// by package, file and name. An unparseable test file fails closed, and so does
// a test name declared in two different Go packages of one directory (an
// internal and an external test package): `go test -run` cannot tell them apart,
// so their results could not be attributed to a single test.
func IdentifyNewTests(changes []FileChange) ([]NewTest, error) {
	baseNames := map[string]map[string]string{} // package dir -> name -> Go package clause at base
	for _, c := range changes {
		if !isTestFile(c.Path) || c.Base == nil {
			continue
		}
		names, err := testFuncNames(c.Path, c.Base)
		if err != nil {
			return nil, err
		}
		dir := path.Dir(c.Path)
		if baseNames[dir] == nil {
			baseNames[dir] = map[string]string{}
		}
		for n, pkg := range names {
			if prev, ok := baseNames[dir][n]; ok && prev != pkg {
				pkg = ambiguousPkg
			}
			baseNames[dir][n] = pkg
		}
	}
	headNames := map[string]map[string]string{}
	var out []NewTest
	for _, c := range changes {
		if !isTestFile(c.Path) || c.Head == nil || c.Status == 'D' {
			continue
		}
		names, err := testFuncNames(c.Path, c.Head)
		if err != nil {
			return nil, err
		}
		dir := path.Dir(c.Path)
		if headNames[dir] == nil {
			headNames[dir] = map[string]string{}
		}
		for n, pkg := range names {
			if prev, ok := headNames[dir][n]; ok && prev != pkg {
				return nil, fmt.Errorf("redhonesty: test %s in %s is declared in both package %s and %s; it cannot be attributed to one test", n, dir, prev, pkg)
			}
			headNames[dir][n] = pkg
			if bp, existed := baseNames[dir][n]; existed {
				if bp != pkg && bp != ambiguousPkg {
					return nil, fmt.Errorf("redhonesty: test %s in %s exists at base in package %s and at head in package %s; it cannot be attributed to one test", n, dir, bp, pkg)
				}
				continue
			}
			out = append(out, NewTest{Package: dir, File: c.Path, Name: n})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Name < b.Name
	})
	return out, nil
}

// declSet is the per-file declaration inventory used to diff base and head.
type declSet struct {
	funcs  map[string]string            // recv+name -> normalized text
	types  map[string]string            // named type/const/var -> normalized text
	fields map[string]map[string]string // type -> field name -> type+tag text
}

func normText(b []byte) string {
	return strings.ReplaceAll(string(b), "\r", "")
}

func inventory(file string, src []byte) (*declSet, error) {
	ds := &declSet{funcs: map[string]string{}, types: map[string]string{}, fields: map[string]map[string]string{}}
	if len(src) == 0 {
		return ds, nil
	}
	fset, f, err := parseSource(file, src)
	if err != nil {
		return nil, err
	}
	text := func(n ast.Node) string {
		s, e := fset.Position(n.Pos()).Offset, fset.Position(n.End()).Offset
		if s < 0 || e > len(src) || s > e {
			return ""
		}
		return normText(src[s:e])
	}
	// init functions and blank identifiers may be declared many times; give
	// each occurrence its own key so a change to an earlier one is not lost.
	ordinal := 0
	multi := func(name string) string {
		ordinal++
		return fmt.Sprintf("%s\x00%s#%d", name, file, ordinal)
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			key := d.Name.Name
			if d.Recv != nil && len(d.Recv.List) == 1 {
				key = text(d.Recv.List[0].Type) + "." + key
			} else if key == "init" || key == "_" {
				key = multi(key)
			}
			ds.funcs[key] = text(d)
		case *ast.GenDecl:
			for _, sp := range d.Specs {
				switch sp := sp.(type) {
				case *ast.TypeSpec:
					ds.types[sp.Name.Name] = text(sp)
					if st, ok := sp.Type.(*ast.StructType); ok {
						fm := map[string]string{}
						for _, fld := range st.Fields.List {
							ft := text(fld.Type)
							if fld.Tag != nil {
								ft += " " + fld.Tag.Value
							}
							if len(fld.Names) == 0 {
								fm[embeddedName(fld.Type)] = ft
								continue
							}
							for _, n := range fld.Names {
								fm[n.Name] = ft
							}
						}
						ds.fields[sp.Name.Name] = fm
					}
				case *ast.ValueSpec:
					for _, n := range sp.Names {
						key := n.Name
						if key == "_" {
							key = multi(key)
						}
						ds.types[key] = text(sp)
					}
				}
			}
		}
	}
	return ds, nil
}

func embeddedName(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return embeddedName(e.X)
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.IndexExpr:
		return embeddedName(e.X)
	}
	return "_embedded"
}

func bareFuncName(key string) string {
	key, _, _ = strings.Cut(key, "\x00")
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[i+1:]
	}
	return key
}

// merge unions src into d (used to compare declarations per package directory
// so a declaration moved between files is not mistaken for a change). Same-named
// declarations with different text (for example build-tag variants in different
// files) are all retained, so a change to any variant is visible.
func (d *declSet) merge(src *declSet) {
	for k, v := range src.funcs {
		d.funcs[k] = combine(d.funcs[k], v)
	}
	for k, v := range src.types {
		d.types[k] = combine(d.types[k], v)
	}
	for t, fm := range src.fields {
		if d.fields[t] == nil {
			d.fields[t] = map[string]string{}
		}
		for k, v := range fm {
			d.fields[t][k] = combine(d.fields[t][k], v)
		}
	}
}

// combine joins distinct declaration texts into one canonical (sorted,
// de-duplicated) value so equal sets compare equal regardless of file order.
func combine(existing, add string) string {
	if existing == "" || existing == add {
		return add
	}
	parts := strings.Split(existing, "\x02")
	for _, p := range parts {
		if p == add {
			return existing
		}
	}
	parts = append(parts, add)
	sort.Strings(parts)
	return strings.Join(parts, "\x02")
}

func newDeclSet() *declSet {
	return &declSet{funcs: map[string]string{}, types: map[string]string{}, fields: map[string]map[string]string{}}
}

// ChangedSymbols computes the production-symbol delta across non-test Go
// files: functions/methods and named declarations that were added, deleted or
// whose text changed, struct fields that were added/deleted/changed, and the
// untouched sibling fields of structs that have a changed field. Declarations
// are compared per package directory, so moving one between files of the same
// package is not a change. Symbols are matched by bare name only (no type
// resolution). An unparseable production file fails closed.
func ChangedSymbols(changes []FileChange) (Symbols, error) {
	s := Symbols{Funcs: map[string]struct{}{}, Types: map[string]struct{}{}, Fields: map[string]struct{}{}, Siblings: map[string]struct{}{}}
	bases, heads := map[string]*declSet{}, map[string]*declSet{}
	var dirs []string
	for _, c := range changes {
		if !isGoSource(c.Path) || isTestFile(c.Path) {
			continue
		}
		base, err := inventory(c.Path, c.Base)
		if err != nil {
			return Symbols{}, err
		}
		head, err := inventory(c.Path, c.Head)
		if err != nil {
			return Symbols{}, err
		}
		dir := path.Dir(c.Path)
		if bases[dir] == nil {
			bases[dir], heads[dir] = newDeclSet(), newDeclSet()
			dirs = append(dirs, dir)
		}
		bases[dir].merge(base)
		heads[dir].merge(head)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		diffFuncs(s, bases[dir], heads[dir])
		diffTypes(s, bases[dir], heads[dir])
	}
	for f := range s.Fields {
		delete(s.Siblings, f) // a field changed anywhere is never merely a sibling
	}
	return s, nil
}

func diffFuncs(s Symbols, base, head *declSet) {
	for k, ht := range head.funcs {
		if bt, ok := base.funcs[k]; !ok || bt != ht {
			s.Funcs[bareFuncName(k)] = struct{}{}
		}
	}
	for k := range base.funcs {
		if _, ok := head.funcs[k]; !ok {
			s.Funcs[bareFuncName(k)] = struct{}{}
		}
	}
}

func diffTypes(s Symbols, base, head *declSet) {
	names := map[string]struct{}{}
	for k, ht := range head.types {
		if bt, ok := base.types[k]; !ok || bt != ht {
			names[k] = struct{}{}
		}
	}
	for k := range base.types {
		if _, ok := head.types[k]; !ok {
			names[k] = struct{}{}
		}
	}
	for name := range names {
		s.Types[name] = struct{}{}
		bf, hf := base.fields[name], head.fields[name]
		changed := map[string]struct{}{}
		for fn, ht := range hf {
			if bt, ok := bf[fn]; !ok || bt != ht {
				changed[fn] = struct{}{}
			}
		}
		for fn := range bf {
			if _, ok := hf[fn]; !ok {
				changed[fn] = struct{}{}
			}
		}
		if len(changed) == 0 {
			continue
		}
		for fn := range changed {
			s.Fields[fn] = struct{}{}
		}
		for fn := range hf {
			if _, isChanged := changed[fn]; !isChanged {
				s.Siblings[fn] = struct{}{}
			}
		}
	}
}
