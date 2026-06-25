package engine

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSafeRead_returnsFileContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fact.txt")
	if err := os.WriteFile(path, []byte("file content"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := safeRead(path, "", discardLog())
	if !ok {
		t.Fatal("safeRead() ok = false, want true")
	}
	if got != "file content" {
		t.Fatalf("safeRead() = %q, want file content", got)
	}
}

func TestSafeRead_returnsDefaultForUnreadablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")

	got, ok := safeRead(path, "default", discardLog())
	if ok {
		t.Fatal("safeRead() ok = true, want false")
	}
	if got != "default" {
		t.Fatalf("safeRead() = %q, want default", got)
	}
}

func TestSafeReadAcceptsNilLoggerForUnreadablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")

	got, ok := safeRead(path, "default", nil)
	if ok {
		t.Fatal("safeRead() ok = true, want false")
	}
	if got != "default" {
		t.Fatalf("safeRead() = %q, want default", got)
	}
}

func TestSafeRead_logsDebugForUnreadablePathLikeRubyFileHelper(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	var messages []string
	logger := captureLogger(&messages, nil, nil)

	got, ok := safeRead(path, "default", logger)

	if ok {
		t.Fatal("safeRead() ok = true, want false")
	}
	if got != "default" {
		t.Fatalf("safeRead() = %q, want default", got)
	}
	want := []string{"File at: " + path + " is not accessible."}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("debug messages = %#v, want %#v", messages, want)
	}
}

func TestSafeReadLines_returnsFileLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fact.txt")
	if err := os.WriteFile(path, []byte("line 1\nline 2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := safeReadLines(path, nil, discardLog())
	if !ok {
		t.Fatal("safeReadLines() ok = false, want true")
	}
	want := []string{"line 1\n", "line 2\n"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("safeReadLines() = %#v, want %#v", got, want)
	}
}

func TestSafeReadLines_preservesFinalLineWithoutNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fact.txt")
	if err := os.WriteFile(path, []byte("line 1\nline 2"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, ok := safeReadLines(path, nil, discardLog())
	if !ok {
		t.Fatal("safeReadLines() ok = false, want true")
	}
	want := []string{"line 1\n", "line 2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("safeReadLines() = %#v, want %#v", got, want)
	}
}

func TestSafeReadLines_returnsDefaultForUnreadablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	want := []string{"default"}

	got, ok := safeReadLines(path, want, discardLog())
	if ok {
		t.Fatal("safeReadLines() ok = true, want false")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("safeReadLines() = %#v, want %#v", got, want)
	}
}

func TestSafeReadLines_logsDebugForUnreadablePathLikeRubyFileHelper(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	defaultLines := []string{"default"}
	var messages []string
	logger := captureLogger(&messages, nil, nil)

	got, ok := safeReadLines(path, defaultLines, logger)

	if ok {
		t.Fatal("safeReadLines() ok = true, want false")
	}
	if !reflect.DeepEqual(got, defaultLines) {
		t.Fatalf("safeReadLines() = %#v, want %#v", got, defaultLines)
	}
	want := []string{"File at: " + path + " is not accessible."}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("debug messages = %#v, want %#v", messages, want)
	}
}

func TestDirChildren_returnsDirectoryEntries(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"file.txt", "a"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := dirChildren(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "file.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dirChildren() = %#v, want %#v", got, want)
	}
}
