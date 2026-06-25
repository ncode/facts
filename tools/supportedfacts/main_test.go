package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestMainWritesGeneratedDocs(t *testing.T) {
	root := repoRoot(t)
	schema, err := os.ReadFile(filepath.Join(root, "docs", "schema", "facts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("docs", "schema"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("docs", "schema", "facts.yaml"), schema, 0o600); err != nil {
		t.Fatal(err)
	}

	main()

	if _, err := os.Stat(filepath.Join("docs", "supported-facts", "README.md")); err != nil {
		t.Fatalf("generated README stat err = %v", err)
	}
}

func TestRunMainReportsRenderErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	var stderr bytes.Buffer

	if code := runMain(&stderr); code != 1 {
		t.Fatalf("runMain() code = %d, want 1", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("stderr is empty, want schema load error")
	}
}

func TestRunMainReportsCreateErrors(t *testing.T) {
	root := repoRoot(t)
	schema, err := os.ReadFile(filepath.Join(root, "docs", "schema", "facts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("docs", "schema"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("docs", "schema", "facts.yaml"), schema, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("docs", "supported-facts"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer

	if code := runMain(&stderr); code != 1 {
		t.Fatalf("runMain() code = %d, want 1", code)
	}
	if got := filepath.ToSlash(stderr.String()); !strings.Contains(got, "create docs/supported-facts:") {
		t.Fatalf("stderr = %q, want create error", got)
	}
}

func TestRunMainReportsWriteErrors(t *testing.T) {
	root := repoRoot(t)
	schema, err := os.ReadFile(filepath.Join(root, "docs", "schema", "facts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("docs", "schema"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("docs", "schema", "facts.yaml"), schema, 0o600); err != nil {
		t.Fatal(err)
	}
	docPaths := []string{filepath.Join("docs", "supported-facts", "README.md")}
	for _, platform := range factschema.Platforms() {
		docPaths = append(docPaths, filepath.Join("docs", "supported-facts", platform.ID+".md"))
	}
	for _, path := range docPaths {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var stderr bytes.Buffer

	if code := runMain(&stderr); code != 1 {
		t.Fatalf("runMain() code = %d, want 1", code)
	}
	if got := filepath.ToSlash(stderr.String()); !strings.Contains(got, "write docs/supported-facts/") {
		t.Fatalf("stderr = %q, want write error", got)
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

func TestExampleOutputReturnsErrorForMissingPlatform(t *testing.T) {
	if _, err := exampleOutput("missing-platform"); err == nil {
		t.Fatal("exampleOutput(missing-platform) err = nil, want error")
	}
}

func TestRenderPlatformReturnsExampleError(t *testing.T) {
	_, err := renderPlatform(factschema.Schema{}, factschema.Platform{ID: "missing-platform", Label: "Missing"})
	if err == nil {
		t.Fatal("renderPlatform(missing-platform) err = nil, want missing example error")
	}
}

func TestExampleOutputReturnsErrorForMalformedJSON(t *testing.T) {
	original := exampleJSON["linux"]
	exampleJSON["linux"] = "{"
	t.Cleanup(func() {
		exampleJSON["linux"] = original
	})

	if _, err := exampleOutput("linux"); err == nil {
		t.Fatal("exampleOutput(malformed) err = nil, want error")
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
