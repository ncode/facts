package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestNoRawHostIOInResolvers freezes the collapsed state: fact resolvers reach
// the host only through the Session seam (s.readFile/readDir/stat/lstat/glob/
// commandOutput). A raw os.ReadDir/os.ReadFile/os.Stat/os.Lstat/filepath.Glob/
// exec.Command in a resolver file bypasses the seam and makes the resolver
// untestable with a fake host, so it fails here.
//
// The exclusion list is the documented seam boundary: the seam implementation
// itself (session*.go), the external-fact loader (external.go), the persistent
// cache (cache.go), config parsing (config.go), and the syscall-tagged statfs
// files. Test files are not resolvers and are not scanned.
func TestNoRawHostIOInResolvers(t *testing.T) {
	forbidden := map[string]map[string]bool{
		"os":       {"ReadDir": true, "ReadFile": true, "Stat": true, "Lstat": true},
		"filepath": {"Glob": true},
		"exec":     {"Command": true, "CommandContext": true},
	}
	excludedExact := map[string]bool{
		"external.go": true,
		"cache.go":    true,
		"config.go":   true,
	}
	excludedPrefix := []string{"session", "statfs"}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if excludedExact[name] {
			continue
		}
		skip := false
		for _, prefix := range excludedPrefix {
			if strings.HasPrefix(name, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if methods := forbidden[pkg.Name]; methods[sel.Sel.Name] {
				pos := fset.Position(sel.Pos())
				t.Errorf("%s:%d: raw host I/O %s.%s bypasses the Session seam; route it through s.readFile/readDir/stat/lstat/glob/commandOutput",
					name, pos.Line, pkg.Name, sel.Sel.Name)
			}
			return true
		})
	}
}

func TestNoMutableDiscoveryPolicyGlobals(t *testing.T) {
	files := []string{"cache.go", "config.go", "defaults.go", "external.go", "../app/app.go"}
	forbiddenNames := map[string]bool{
		"DefaultCachePath":           true,
		"DefaultConfigPath":          true,
		"NativeDefaultConfigPath":    true,
		"defaultExternalFactDirs":    true,
		"cacheWriteFile":             true,
		"cacheRemove":                true,
		"externalFactCommandTimeout": true,
		"externalFactMaxBytes":       true,
	}

	fset := token.NewFileSet()
	for _, path := range files {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				valueSpec := spec.(*ast.ValueSpec)
				for i, name := range valueSpec.Names {
					if forbiddenNames[name.Name] || mutableDiscoveryPolicyReplacement(path, name.Name, valueSpec, i) {
						pos := fset.Position(name.Pos())
						t.Errorf("%s:%d: mutable discovery policy %s must be invocation-local data or a constant", path, pos.Line, name.Name)
					}
				}
			}
		}
	}
}

func TestMutableDiscoveryPolicyReplacement_rejectsEquivalentLimits(t *testing.T) {
	tests := []struct {
		path string
		name string
	}{
		{path: "defaults.go", name: "commandDeadline"},
		{path: "defaults.go", name: "sizeCap"},
		{path: "external.go", name: "commandDeadline"},
		{path: "external.go", name: "sizeCap"},
	}
	for _, tt := range tests {
		t.Run(tt.path+"/"+tt.name, func(t *testing.T) {
			spec := &ast.ValueSpec{
				Names:  []*ast.Ident{ast.NewIdent(tt.name)},
				Values: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: "1"}},
			}
			if !mutableDiscoveryPolicyReplacement(tt.path, tt.name, spec, 0) {
				t.Fatalf("mutableDiscoveryPolicyReplacement(%q, %q) = false, want true", tt.path, tt.name)
			}
		})
	}
}

func mutableDiscoveryPolicyReplacement(path, name string, spec *ast.ValueSpec, index int) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline") ||
		((strings.Contains(lower, "byte") || strings.Contains(lower, "size")) &&
			(strings.Contains(lower, "max") || strings.Contains(lower, "limit") || strings.Contains(lower, "cap"))) {
		return true
	}
	if path != "cache.go" && path != "config.go" && path != "defaults.go" && path != "../app/app.go" {
		return false
	}
	if _, ok := spec.Type.(*ast.FuncType); ok {
		return true
	}
	if index >= len(spec.Values) {
		return false
	}
	switch value := spec.Values[index].(type) {
	case *ast.FuncLit:
		return true
	case *ast.Ident:
		return strings.HasPrefix(value.Name, "platform") || strings.Contains(strings.ToLower(value.Name), "cache")
	case *ast.SelectorExpr:
		return strings.Contains(strings.ToLower(value.Sel.Name), "default") || value.Sel.Name == "Remove"
	default:
		return false
	}
}
