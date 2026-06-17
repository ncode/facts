package main

import (
	"os"
	"path/filepath"
	"testing"
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

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(dir, "..", ".."))
}
