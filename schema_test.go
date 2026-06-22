package facts

// Schema conformance for docs/schema/facts.yaml (openspec change
// facts-schema). The test flattens a hermetic discovery into leaf paths and
// checks the schema two ways on every platform gate:
//
//	(a) no undocumented facts — every emitted leaf path matches a schema
//	    entry whose platforms include this host's platform;
//	(b) no overclaimed facts — every non-conditional schema entry for this
//	    platform matches at least one emitted path.
//
// Authoring helper: `go test -run TestFactsSchemaConformance . -args
// -schema-report` prints the undocumented paths grouped by top-level fact
// instead of failing, so a new fact tells you exactly what to document.

import (
	"flag"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	factschema "github.com/ncode/facts/internal/schema"
)

var schemaReport = flag.Bool("schema-report", false,
	"print undocumented fact paths grouped by top-level fact instead of failing")

const schemaPath = factschema.DefaultPath

func loadSchema(t *testing.T) factschema.Schema {
	t.Helper()
	schema, err := factschema.LoadFile(filepath.FromSlash(schemaPath))
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	return schema
}

func printSchemaReport(paths []string, undocumented []string) {
	if len(undocumented) == 0 {
		fmt.Printf("schema-report: all %d emitted leaf paths are documented in %s\n", len(paths), schemaPath)
		return
	}
	grouped := map[string][]string{}
	for _, path := range undocumented {
		top, _, _ := strings.Cut(path, ".")
		grouped[top] = append(grouped[top], path)
	}
	tops := make([]string, 0, len(grouped))
	for top := range grouped {
		tops = append(tops, top)
	}
	sort.Strings(tops)
	fmt.Printf("schema-report: %d undocumented leaf paths on %s\n", len(undocumented), runtime.GOOS)
	for _, top := range tops {
		fmt.Printf("%s:\n", top)
		for _, path := range grouped[top] {
			fmt.Printf("  %s\n", path)
		}
	}
}

func TestFactsSchemaConformance(t *testing.T) {
	schema := loadSchema(t)

	paths := factschema.FlattenTree(hermeticSnapshot().Tree())
	if len(paths) == 0 {
		t.Fatal("hermetic discovery emitted no facts")
	}

	undocumented := schema.UndocumentedPaths(paths, runtime.GOOS)
	if *schemaReport {
		printSchemaReport(paths, undocumented)
		return
	}

	// (a) No undocumented facts: every emitted path is described for this
	// platform.
	for _, path := range undocumented {
		t.Errorf("undocumented fact path %q: add an entry to %s (run `go test -run TestFactsSchemaConformance . -args -schema-report`)", path, schemaPath)
	}

	// (b) No overclaimed facts: every non-conditional entry for this platform
	// is present in the discovery.
	for _, pattern := range schema.MissingEntries(paths, runtime.GOOS) {
		t.Errorf("schema entry %q lists platform %s but no discovered fact matches it: mark it `conditional: true` or fix its platforms", pattern, runtime.GOOS)
	}
}
