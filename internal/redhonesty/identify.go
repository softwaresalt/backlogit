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

func isTestFile(p string) bool { return strings.HasSuffix(p, "_test.go") }

func isGoSource(p string) bool { return strings.HasSuffix(p, ".go") }

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

// isTestSignature reports whether fd has the shape func(*testing.T).
func isTestSignature(fd *ast.FuncDecl) bool {
	if fd.Recv != nil || fd.Type.TypeParams != nil || fd.Type.Results != nil {
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
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "T" {
		return false
	}
	_, ok = sel.X.(*ast.Ident)
	return ok
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

func testFuncNames(file string, src []byte) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	if len(src) == 0 {
		return out, nil
	}
	_, f, err := parseSource(file, src)
	if err != nil {
		return nil, err
	}
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && isTestName(fd.Name.Name) && isTestSignature(fd) {
			out[fd.Name.Name] = struct{}{}
		}
	}
	return out, nil
}

// IdentifyNewTests returns the top-level test functions present at head that
// were not present at base in the same package directory (so a test moved
// between files of one package is not reported as new). The result is sorted
// by package, file and name. An unparseable test file fails closed.
func IdentifyNewTests(changes []FileChange) ([]NewTest, error) {
	baseNames := map[string]map[string]struct{}{} // package dir -> names at base
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
			baseNames[dir] = map[string]struct{}{}
		}
		for n := range names {
			baseNames[dir][n] = struct{}{}
		}
	}
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
		for n := range names {
			if _, existed := baseNames[dir][n]; existed {
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
