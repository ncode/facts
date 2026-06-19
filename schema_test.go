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
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var schemaReport = flag.Bool("schema-report", false,
	"print undocumented fact paths grouped by top-level fact instead of failing")

const schemaPath = "docs/schema/facts.yaml"

// schemaEntry is one documented fact: a dotted path or `*` pattern mapped to
// its type, description, platform list, and conditional marker.
type schemaEntry struct {
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Platforms   []string `yaml:"platforms"`
	Conditional bool     `yaml:"conditional"`
}

var schemaTypes = map[string]bool{
	"string":  true,
	"integer": true,
	"double":  true,
	"boolean": true,
	"map":     true,
	"array":   true,
}

var schemaPlatforms = map[string]bool{
	"linux":     true,
	"darwin":    true,
	"windows":   true,
	"freebsd":   true,
	"openbsd":   true,
	"netbsd":    true,
	"dragonfly": true,
	"illumos":   true,
}

func loadSchema(t *testing.T) map[string]schemaEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.FromSlash(schemaPath))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schema := map[string]schemaEntry{}
	if err := yaml.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse %s: %v", schemaPath, err)
	}
	if len(schema) == 0 {
		t.Fatalf("%s has no entries", schemaPath)
	}
	return schema
}

// validateSchema pins the schema file's own shape: every entry carries a
// known type, a description, and at least one valid platform.
func validateSchema(t *testing.T, schema map[string]schemaEntry) {
	t.Helper()
	for _, pattern := range sortedPatterns(schema) {
		entry := schema[pattern]
		if !schemaTypes[entry.Type] {
			t.Errorf("%s: entry %q has invalid type %q", schemaPath, pattern, entry.Type)
		}
		if strings.TrimSpace(entry.Description) == "" {
			t.Errorf("%s: entry %q has no description", schemaPath, pattern)
		}
		if len(entry.Platforms) == 0 {
			t.Errorf("%s: entry %q lists no platforms", schemaPath, pattern)
		}
		seen := map[string]bool{}
		for _, platform := range entry.Platforms {
			if !schemaPlatforms[platform] {
				t.Errorf("%s: entry %q lists invalid platform %q", schemaPath, pattern, platform)
			}
			if seen[platform] {
				t.Errorf("%s: entry %q lists platform %q twice", schemaPath, pattern, platform)
			}
			seen[platform] = true
		}
	}
}

// flattenTree reduces the canonical tree to sorted leaf paths: maps recurse
// with one segment per key (an empty map is itself a leaf), arrays contribute
// a single `path.*` without enumerating indices, and scalars (nil included)
// are leaves.
func flattenTree(tree map[string]any) []string {
	leaves := make([]string, 0, 256)
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch v := value.(type) {
		case map[string]any:
			if len(v) == 0 {
				leaves = append(leaves, prefix)
				return
			}
			for key, item := range v {
				walk(prefix+"."+key, item)
			}
		default:
			if value != nil && reflect.TypeOf(value).Kind() == reflect.Slice {
				leaves = append(leaves, prefix+".*")
				return
			}
			leaves = append(leaves, prefix)
		}
	}
	for key, value := range tree {
		walk(key, value)
	}
	sort.Strings(leaves)
	return leaves
}

// patternMatchesSegments reports whether the pattern matches exactly the
// given path segments, with `*` matching exactly one segment.
func patternMatchesSegments(pattern []string, segments []string) bool {
	if len(pattern) != len(segments) {
		return false
	}
	for i, part := range pattern {
		if part != "*" && part != segments[i] {
			return false
		}
	}
	return true
}

// entryMatchesPath reports whether a schema entry covers a leaf path: the
// pattern matches the whole path, or the entry is a map/array and its pattern
// matches a strict ancestor of the path (subtree coverage).
func entryMatchesPath(pattern string, entry schemaEntry, path string) bool {
	patternSegments := strings.Split(pattern, ".")
	pathSegments := strings.Split(path, ".")
	if patternMatchesSegments(patternSegments, pathSegments) {
		return true
	}
	if entry.Type != "map" && entry.Type != "array" {
		return false
	}
	if len(patternSegments) >= len(pathSegments) {
		return false
	}
	return patternMatchesSegments(patternSegments, pathSegments[:len(patternSegments)])
}

func platformsInclude(platforms []string, goos string) bool {
	for _, platform := range platforms {
		if platform == goos {
			return true
		}
	}
	return false
}

func sortedPatterns(schema map[string]schemaEntry) []string {
	patterns := make([]string, 0, len(schema))
	for pattern := range schema {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	return patterns
}

// undocumentedPaths returns the emitted leaf paths no platform-applicable
// schema entry covers.
func undocumentedPaths(paths []string, schema map[string]schemaEntry, goos string) []string {
	var unmatched []string
	for _, path := range paths {
		documented := false
		for pattern, entry := range schema {
			if !platformsInclude(entry.Platforms, goos) {
				continue
			}
			if entryMatchesPath(pattern, entry, path) {
				documented = true
				break
			}
		}
		if !documented {
			unmatched = append(unmatched, path)
		}
	}
	return unmatched
}

// missingEntries returns the non-conditional schema entries for goos that no
// emitted path satisfies.
func missingEntries(paths []string, schema map[string]schemaEntry, goos string) []string {
	var missing []string
	for _, pattern := range sortedPatterns(schema) {
		entry := schema[pattern]
		if entry.Conditional || !platformsInclude(entry.Platforms, goos) {
			continue
		}
		present := false
		for _, path := range paths {
			if entryMatchesPath(pattern, entry, path) {
				present = true
				break
			}
		}
		if !present {
			missing = append(missing, pattern)
		}
	}
	return missing
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
	validateSchema(t, schema)

	paths := flattenTree(hermeticSnapshot().Tree())
	if len(paths) == 0 {
		t.Fatal("hermetic discovery emitted no facts")
	}

	undocumented := undocumentedPaths(paths, schema, runtime.GOOS)
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
	for _, pattern := range missingEntries(paths, schema, runtime.GOOS) {
		t.Errorf("schema entry %q lists platform %s but no discovered fact matches it: mark it `conditional: true` or fix its platforms", pattern, runtime.GOOS)
	}
}
