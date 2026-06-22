package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	factschema "github.com/ncode/facts/internal/schema"
)

func TestGeneratedDocsAreCurrent(t *testing.T) {
	root := repoRoot(t)
	docs, err := renderDocs(filepath.Join(root, "docs", "schema", "facts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range docs {
		got, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != want {
			t.Fatalf("%s is stale; run `make docs`", path)
		}
	}
}

func TestRenderedDocsUseSchemaPlatformVocabulary(t *testing.T) {
	root := repoRoot(t)
	docs, err := renderDocs(filepath.Join(root, "docs", "schema", "facts.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]bool{"docs/supported-facts/README.md": true}
	for _, platform := range factschema.Platforms() {
		want["docs/supported-facts/"+platform.ID+".md"] = true
	}

	got := make(map[string]bool, len(docs))
	for path := range docs {
		got[path] = true
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("renderDocs() paths = %#v, want schema platform docs %#v", got, want)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}
