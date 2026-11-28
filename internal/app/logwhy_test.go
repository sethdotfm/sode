package app

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// keptLogWhys are whys of rows written before the v2 renames (docs/persisted-names.md). No code writes them now, but
// they get words too.
var keptLogWhys = []string{"refreshNames", "unlock", "userUnlock"}

// Every reason that the server writes in a Communication Log row has words in WHY_LABEL (web/logkinds.js). Thus the
// page and the download files never show a code name. The whys come from the source: string literals that reach event() or a
// swp08.Why's Reason, through every function that passes its parameter on. A message type that a refusal logs
// (m.T) is a key of the policy table.
func TestEveryLogReasonHasWords(t *testing.T) {
	src, err := web.FS.ReadFile("logkinds.js")
	if err != nil {
		t.Fatal(err)
	}
	body := regexp.MustCompile(`(?s)export const WHY_LABEL = \{(.*?)\n\};`).FindSubmatch(src)
	if body == nil {
		t.Fatal("no WHY_LABEL in web/logkinds.js")
	}
	labels := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*([A-Za-z]+):`).FindAllSubmatch(body[1], -1) {
		labels[string(m[1])] = true
	}

	whys, unknown := serverLogWhys(t)
	for _, u := range unknown {
		t.Errorf("cannot tell which whys %s writes: give it a string literal, or teach serverLogWhys the source", u)
	}
	for k, r := range policy {
		whys[k] = true
		if r.why != "" {
			whys[r.why] = true
		}
		if r.op != "" {
			whys[r.op] = true
		}
	}
	for _, w := range keptLogWhys {
		whys[w] = true
	}
	if len(whys) < 100 {
		t.Fatalf("found only %d whys: the scan is broken", len(whys))
	}
	var missing []string
	for w := range whys {
		if w != "" && !labels[w] {
			missing = append(missing, w)
		}
	}
	slices.Sort(missing)
	if missing != nil {
		t.Errorf("whys with no words in WHY_LABEL (web/logkinds.js): %s", strings.Join(missing, ", "))
	}
}

// fnInfo is one function of internal/app: its name, its parameter names, and its body for local assignments and
// returns. A function literal in a package var (the policy table's handlers) has no name.
type fnInfo struct {
	name   string
	body   *ast.BlockStmt
	params []string
}

func (f fnInfo) param(name string) int {
	return slices.Index(f.params, name)
}

// serverLogWhys gives the whys the server can write, and the places whose why it cannot follow.
func serverLogWhys(t *testing.T) (map[string]bool, []string) {
	t.Helper()
	fset := token.NewFileSet()
	parse := func(glob string) []*ast.File {
		files, err := filepath.Glob(glob)
		if err != nil || len(files) == 0 {
			t.Fatalf("no files %s: %v", glob, err)
		}
		var out []*ast.File
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(fset, name, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, f)
		}
		return out
	}
	app, swp := parse("*.go"), parse("../swp08/*.go")

	// String constants, by name: swp08's Reason values and this package's own.
	consts := map[string]string{}
	for _, f := range append(slices.Clone(app), swp...) {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, s := range g.Specs {
				vs := s.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if i < len(vs.Values) {
						if l, ok := vs.Values[i].(*ast.BasicLit); ok && l.Kind == token.STRING {
							consts[n.Name], _ = strconv.Unquote(l.Value)
						}
					}
				}
			}
		}
	}
	whys := map[string]bool{}
	for _, f := range swp {
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.CONST {
				for _, s := range g.Specs {
					vs := s.(*ast.ValueSpec)
					if id, ok := vs.Type.(*ast.Ident); ok && id.Name == "Reason" {
						for _, n := range vs.Names {
							whys[consts[n.Name]] = true
						}
					}
				}
			}
		}
	}

	// Functions by name and parameter count, since methods of two types can share a name.
	fns := map[string]fnInfo{}
	var lits []fnInfo
	key := func(name string, n int) string { return fmt.Sprintf("%s/%d", name, n) }
	params := func(ft *ast.FuncType) []string {
		var ps []string
		for _, fl := range ft.Params.List {
			for _, n := range fl.Names {
				ps = append(ps, n.Name)
			}
			if fl.Names == nil {
				ps = append(ps, "_")
			}
		}
		return ps
	}
	for _, f := range app {
		for _, d := range f.Decls {
			switch fd := d.(type) {
			case *ast.FuncDecl:
				if fd.Body != nil {
					ps := params(fd.Type)
					fns[key(fd.Name.Name, len(ps))] = fnInfo{name: fd.Name.Name, body: fd.Body, params: ps}
				}
			case *ast.GenDecl:
				if fd.Tok != token.VAR {
					continue
				}
				ast.Inspect(fd, func(n ast.Node) bool {
					if fl, ok := n.(*ast.FuncLit); ok {
						lits = append(lits, fnInfo{body: fl.Body, params: params(fl.Type)})
						return false
					}
					return true
				})
			}
		}
	}

	// wrappers: functions whose parameter at the index becomes a row's why. event() is the first.
	wrappers := map[string]int{key("event", 4): 1}
	var unknown []string

	// resolve adds the string values expr can hold in fn. A parameter makes fn a wrapper.
	var resolve func(fn fnInfo, e ast.Expr, depth int) bool
	resolve = func(fn fnInfo, e ast.Expr, depth int) bool {
		if depth > 6 {
			return false
		}
		switch x := e.(type) {
		case *ast.BasicLit:
			if x.Kind != token.STRING {
				return false
			}
			s, _ := strconv.Unquote(x.Value)
			whys[s] = true
			return true
		case *ast.ParenExpr:
			return resolve(fn, x.X, depth+1)
		case *ast.SelectorExpr:
			if v, ok := consts[x.Sel.Name]; ok {
				whys[v] = true
				return true
			}
			// A message type, or the why or op of its policy rule: the policy table's keys and values.
			if id, ok := x.X.(*ast.Ident); ok && ((id.Name == "m" && x.Sel.Name == "T") || (id.Name == "r" && (x.Sel.Name == "why" || x.Sel.Name == "op"))) {
				return true
			}
			return false
		case *ast.CallExpr:
			// string(x)
			if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "string" && len(x.Args) == 1 {
				return resolve(fn, x.Args[0], depth+1)
			}
			id, ok := x.Fun.(*ast.Ident)
			if !ok {
				return false
			}
			callee, ok := fns[key(id.Name, len(x.Args))]
			if !ok {
				return false
			}
			found := false
			ast.Inspect(callee.body, func(n ast.Node) bool {
				if r, ok := n.(*ast.ReturnStmt); ok && len(r.Results) == 1 {
					found = resolve(callee, r.Results[0], depth+1) || found
				}
				return true
			})
			return found
		case *ast.Ident:
			if v, ok := consts[x.Name]; ok {
				whys[v] = true
				return true
			}
			if i := fn.param(x.Name); i >= 0 && fn.name != "" {
				k := key(fn.name, len(fn.params))
				if _, ok := wrappers[k]; !ok {
					wrappers[k] = i
				}
				return true
			}
			found := false
			ast.Inspect(fn.body, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok || len(as.Lhs) != len(as.Rhs) {
					return true
				}
				for i, l := range as.Lhs {
					if lid, ok := l.(*ast.Ident); ok && lid.Name == x.Name {
						found = resolve(fn, as.Rhs[i], depth+1) || found
					}
				}
				return true
			})
			return found
		}
		return false
	}

	// Each pass can find new wrappers; stop when a pass finds none.
	for {
		before := len(wrappers)
		unknown = nil
		for _, fn := range append(slices.Collect(maps.Values(fns)), lits...) {
			ast.Inspect(fn.body, func(n ast.Node) bool {
				var arg ast.Expr
				switch x := n.(type) {
				case *ast.CallExpr:
					name := ""
					switch f := x.Fun.(type) {
					case *ast.Ident:
						name = f.Name
					case *ast.SelectorExpr:
						name = f.Sel.Name
					}
					i, ok := wrappers[key(name, len(x.Args))]
					if !ok {
						return true
					}
					arg = x.Args[i]
				case *ast.CompositeLit:
					// swp08.Why{Reason: ...}
					sel, ok := x.Type.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "Why" {
						return true
					}
					for _, el := range x.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "Reason" {
								arg = kv.Value
							}
						}
					}
					if arg == nil {
						return true
					}
				default:
					return true
				}
				if !resolve(fn, arg, 0) {
					unknown = append(unknown, fset.Position(arg.Pos()).String())
				}
				return true
			})
		}
		if len(wrappers) == before {
			break
		}
	}
	slices.Sort(unknown)
	return whys, unknown
}
