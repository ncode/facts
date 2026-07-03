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
