package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

type fakeExternalFactLoaderHost struct {
	externalFactOSHost

	goosValue         string
	env               []string
	recursive         bool
	openFunc          func(string) (io.ReadCloser, error)
	fileReadableFunc  func(string) bool
	runCommandFunc    func(context.Context, string, ...string) ([]byte, []byte, error)
	readDirFunc       func(string) ([]os.DirEntry, error)
	runCommandNames   []string
	runCommandArgsets [][]string
}

func (h *fakeExternalFactLoaderHost) goos() string {
	if h.goosValue != "" {
		return h.goosValue
	}
	return h.externalFactOSHost.goos()
}

func (h *fakeExternalFactLoaderHost) environ() []string {
	if h.env != nil {
		return slices.Clone(h.env)
	}
	return h.externalFactOSHost.environ()
}

func (h *fakeExternalFactLoaderHost) externalFactResolutionRunning() bool {
	return h.recursive
}

func (h *fakeExternalFactLoaderHost) open(path string) (io.ReadCloser, error) {
	if h.openFunc != nil {
		return h.openFunc(path)
	}
	return h.externalFactOSHost.open(path)
}

func (h *fakeExternalFactLoaderHost) fileReadable(path string) bool {
	if h.fileReadableFunc != nil {
		return h.fileReadableFunc(path)
	}
	return h.externalFactOSHost.fileReadable(path)
}

func (h *fakeExternalFactLoaderHost) runCommand(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	h.runCommandNames = append(h.runCommandNames, name)
	h.runCommandArgsets = append(h.runCommandArgsets, slices.Clone(args))
	if h.runCommandFunc != nil {
		return h.runCommandFunc(ctx, name, args...)
	}
	return h.externalFactOSHost.runCommand(ctx, name, args...)
}

func (h *fakeExternalFactLoaderHost) readDir(dir string) ([]os.DirEntry, error) {
	if h.readDirFunc != nil {
		return h.readDirFunc(dir)
	}
	return h.externalFactOSHost.readDir(dir)
}

func TestExternalFactLoader_cliModeIncludesEnvironmentAndSkipsExecutableFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("site=lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken_fact.exe"), []byte("ignored=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := &fakeExternalFactLoaderHost{
		goosValue: "windows",
		env:       []string{"FACTS_from_env=yes"},
		runCommandFunc: func(context.Context, string, ...string) ([]byte, []byte, error) {
			return nil, nil, errors.New("boom")
		},
	}
	got, err := externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderCLI,
		dirs:       []string{dir},
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatalf("externalFactLoader.load() err = %v, want nil", err)
	}
	want := []ResolvedFact{
		{Name: "site", Value: "lab", Type: "external"},
		{Name: "from_env", Value: "yes", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("externalFactLoader.load() = %#v, want %#v", got, want)
	}
	if len(host.runCommandNames) != 1 {
		t.Fatalf("runCommand calls = %#v, want one failed executable call", host.runCommandNames)
	}
}

func TestExternalFactLoader_libraryModeReturnsPartialFailuresAndControlsEnvironment(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("site=lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken_fact.exe"), []byte("ignored=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := &fakeExternalFactLoaderHost{
		goosValue: "windows",
		env:       []string{"FACTS_from_env=yes"},
		runCommandFunc: func(context.Context, string, ...string) ([]byte, []byte, error) {
			return nil, nil, errors.New("boom")
		},
	}
	got, err := externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderLibrary,
		dirs:       []string{dir},
		host:       host,
		includeEnv: false,
	}.load()
	if !errors.Is(err, errExternalFactExec) {
		t.Fatalf("externalFactLoader.load() err = %v, want errExternalFactExec", err)
	}
	want := []ResolvedFact{{Name: "site", Value: "lab", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("externalFactLoader.load() = %#v, want %#v", got, want)
	}

	host.runCommandNames = nil
	got, err = externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderLibrary,
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatalf("externalFactLoader.load() env-only err = %v, want nil", err)
	}
	want = []ResolvedFact{{Name: "from_env", Value: "yes", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("externalFactLoader.load() env-only = %#v, want %#v", got, want)
	}
}

func TestExternalFactLoader_cliModeSkipsCancelledExecutableFailures(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cancelled_fact"), []byte("ignored=true\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := NewSessionContext(ctx)
	host := &fakeExternalFactLoaderHost{
		goosValue: "linux",
		runCommandFunc: func(ctx context.Context, _ string, _ ...string) ([]byte, []byte, error) {
			return nil, nil, ctx.Err()
		},
	}

	got, err := externalFactLoader{
		s:    s,
		mode: externalFactLoaderCLI,
		dirs: []string{dir},
		host: host,
	}.load()
	if err != nil {
		t.Fatalf("externalFactLoader.load() err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("externalFactLoader.load() = %#v, want no facts", got)
	}
}

func TestLoadExternalFacts_txtFacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("one=two\nthree=four=five\nempty=\n=value\ninvalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.toml"), []byte("toml_fact = ignored"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "one", Value: "two", Type: "external"},
		{Name: "three", Value: "four=five", Type: "external"},
	}
	if len(got) != len(want) {
		t.Fatalf("LoadExternalFacts(testSession) len = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("LoadExternalFacts(testSession)[%d] = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestLoadExternalFacts_processesDirectoryEntriesInReverseLexicographicOrderLikeRubyDirectoryLoader(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("first=a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "z.txt"), []byte("last=z\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "last", Value: "z", Type: "external"},
		{Name: "first", Value: "a", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_reportsBlockedFilesLikeRubyDirectoryLoader(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "data.yaml"), []byte("f1: one\nf2: two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("f3=three\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var debugMessages []string
	s := NewSession()
	s.logger = captureLogger(&debugMessages, nil, nil)

	got, err := LoadExternalFactsWithBlocklist(s, []string{dir}, map[string]bool{"data.yaml": true})
	if err != nil {
		t.Fatal(err)
	}
	wantFacts := []ResolvedFact{{Name: "f3", Value: "three", Type: "external"}}
	if !reflect.DeepEqual(got, wantFacts) {
		t.Fatalf("LoadExternalFactsWithBlocklist(testSession) = %#v, want %#v", got, wantFacts)
	}
	wantDebug := []string{"External fact file data.yaml blocked."}
	if !reflect.DeepEqual(debugMessages, wantDebug) {
		t.Fatalf("debug messages = %#v, want %#v", debugMessages, wantDebug)
	}
}

func TestLoadExternalFacts_reportsIgnoredBackupFilesLikeRubyDirectoryLoader(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"data.bak", "data.orig"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("foo=bar\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var debugMessages []string
	s := NewSession()
	s.logger = captureLogger(&debugMessages, nil, nil)

	got, err := LoadExternalFacts(s, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts", got)
	}
	for _, ext := range []string{"orig", "bak"} {
		found := false
		for _, message := range debugMessages {
			if strings.Contains(message, ext) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("debug messages = %#v, want message mentioning %s", debugMessages, ext)
		}
	}
}

func TestLoadExternalFacts_ignoresMissingDirectories(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts", got)
	}
}

func TestLoadExternalFacts_loadsEnvironmentFactsWithoutUnderscore(t *testing.T) {
	t.Setenv("FACTERsite_location", "lab")

	got, err := LoadExternalFacts(testSession, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "site_location", Value: "lab", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_ignoresEnvironmentFactsWithoutRubyWordNames(t *testing.T) {
	env := []string{
		"FACTER_role=web",
		"FACTER_TWO=boo",
		"FACTER_os.name=ignored",
		"FACTER_role-name=ignored",
		"FACTER_foo bar=ignored",
	}

	got, err := loadExternalEnvFacts(env)
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "role", Value: "web", Type: "external"},
		{Name: "two", Value: "boo", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadExternalEnvFacts() = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_loadsFactsNativeEnvironmentFacts(t *testing.T) {
	env := []string{
		"FACTS_site_role=native",
		"FACTSsite_location=lab",
		"FACTS_os.name=ignored",
	}

	got, err := loadExternalEnvFacts(env)
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "site_location", Value: "lab", Type: "external"},
		{Name: "site_role", Value: "native", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loadExternalEnvFacts() = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_factsNativeEnvironmentFactsWinCollisions(t *testing.T) {
	// Native wins regardless of environment ordering; names set through only
	// one prefix resolve from that prefix.
	for _, env := range [][]string{
		{"FACTS_site_role=native", "FACTER_site_role=compat", "FACTER_compat_only=compat", "FACTS_native_only=native"},
		{"FACTER_site_role=compat", "FACTS_site_role=native", "FACTS_native_only=native", "FACTER_compat_only=compat"},
	} {
		got, err := loadExternalEnvFacts(env)
		if err != nil {
			t.Fatal(err)
		}
		want := []ResolvedFact{
			{Name: "compat_only", Value: "compat", Type: "external"},
			{Name: "native_only", Value: "native", Type: "external"},
			{Name: "site_role", Value: "native", Type: "external"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("loadExternalEnvFacts(%v) = %#v, want %#v", env, got, want)
		}
	}
}

func TestLoadExternalFacts_jsonFacts(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`{"site":"lab","features":["json","external"],"nested":{"enabled":true}}`)
	if err := os.WriteFile(filepath.Join(dir, "site.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"features": []any{"json", "external"},
		"nested":   map[string]any{"enabled": true},
		"site":     "lab",
	}
	if len(got) != len(want) {
		t.Fatalf("LoadExternalFacts(testSession) len = %d, want %d: %#v", len(got), len(want), got)
	}
	for _, fact := range got {
		if fact.Type != "external" {
			t.Fatalf("fact %q type = %q, want external", fact.Name, fact.Type)
		}
		if !reflect.DeepEqual(fact.Value, want[fact.Name]) {
			t.Fatalf("fact %q value = %#v, want %#v", fact.Name, fact.Value, want[fact.Name])
		}
		delete(want, fact.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing facts: %#v", want)
	}
}

func TestLoadExternalFacts_ignoresJSONWithTrailingTokens(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.json"), []byte(`{"site":"lab"} garbage`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil for malformed structured file", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts", got)
	}
}

func TestLoadExternalFacts_preservesLargeJSONIntegerAsInt64(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.json"), []byte(`{"big":2147483648}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "big", Value: int64(2147483648), Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_yamlFacts(t *testing.T) {
	dir := t.TempDir()
	content := []byte("site: lab\nfeatures:\n  - yaml\n  - external\nnested:\n  enabled: true\ncount: 3\n")
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"count":    3,
		"features": []any{"yaml", "external"},
		"nested":   map[string]any{"enabled": true},
		"site":     "lab",
	}
	if len(got) != len(want) {
		t.Fatalf("LoadExternalFacts(testSession) len = %d, want %d: %#v", len(got), len(want), got)
	}
	for _, fact := range got {
		if fact.Type != "external" {
			t.Fatalf("fact %q type = %q, want external", fact.Name, fact.Type)
		}
		if !reflect.DeepEqual(fact.Value, want[fact.Name]) {
			t.Fatalf("fact %q value = %#v, want %#v", fact.Name, fact.Value, want[fact.Name])
		}
		delete(want, fact.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing facts: %#v", want)
	}
}

func TestExternalFactLoader_ignoresNonRegularStructuredFiles(t *testing.T) {
	opened := false
	host := &fakeExternalFactLoaderHost{
		openFunc: func(string) (io.ReadCloser, error) {
			opened = true
			return io.NopCloser(strings.NewReader(`{"site":"lab"}`)), nil
		},
	}

	got, err := externalFactLoader{s: testSession, host: host}.loadExternalFactFile("site.json", os.ModeNamedPipe)
	if err != nil {
		t.Fatalf("loadExternalFactFile() err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("loadExternalFactFile() = %#v, want no facts", got)
	}
	if opened {
		t.Fatal("loadExternalFactFile opened non-regular structured file")
	}
}

func TestLoadExternalFacts_acceptsLongKeyValueLineWithinLimit(t *testing.T) {
	dir := t.TempDir()
	value := strings.Repeat("x", 70*1024)
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("site="+value+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil", err)
	}
	want := []ResolvedFact{{Name: "site", Value: value, Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want long site fact", got)
	}
}

func TestLoadExternalFacts_yamlTimestampValuesStayStrings(t *testing.T) {
	dir := t.TempDir()
	content := []byte("testsfact:\n  time: 2020-04-28 01:44:08.148119000 +01:01\n")
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "testsfact", Value: map[string]any{"time": "2020-04-28 01:44:08.148119000 +01:01"}, Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_yamlTimestampWithoutZoneStaysString(t *testing.T) {
	dir := t.TempDir()
	content := []byte("testsfact:\n  time: 2020-04-28 01:44:08.148119000\n")
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "testsfact", Value: map[string]any{"time": "2020-04-28 01:44:08.148119000"}, Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_yamlDateLoadsAsDateLikeRubyParser(t *testing.T) {
	dir := t.TempDir()
	content := []byte("testsfact:\n  date: 2020-04-28\n")
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "testsfact", Value: map[string]any{"date": time.Date(2020, 4, 28, 0, 0, 0, 0, time.UTC)}, Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_yamlAnchors(t *testing.T) {
	dir := t.TempDir()
	content := []byte("one:\n  test: &anchored\n    a:\n      - foo\ntwo:\n  TEST: *anchored\n")
	if err := os.WriteFile(filepath.Join(dir, "site.yaml"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "one", Value: map[string]any{"test": map[string]any{"a": []any{"foo"}}}, Type: "external"},
		{Name: "two", Value: map[string]any{"TEST": map[string]any{"a": []any{"foo"}}}, Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_ignoresStructuredFilesWithoutKeyValueData(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"empty.yaml":  []byte(""),
		"scalar.yaml": []byte("foo\n"),
		"array.json":  []byte(`["foo"]`),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts", got)
	}
}

func TestLoadExternalFacts_reportsStructuredFilesWithoutKeyValueData(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"array.json":  []byte(`["foo"]`),
		"scalar.yaml": []byte("foo\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var messages []string
	s := NewSession()
	s.logger = captureLogger(nil, nil, &messages)

	got, err := LoadExternalFacts(s, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts", got)
	}
	want := []string{
		fmt.Sprintf("Structured data fact file %s was parsed but no key=>value data was returned.", filepath.Join(dir, "scalar.yaml")),
		fmt.Sprintf("Structured data fact file %s was parsed but no key=>value data was returned.", filepath.Join(dir, "array.json")),
	}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("error messages = %#v, want %#v", messages, want)
	}
}

func TestLoadExternalFacts_reportsEmptyStructuredFilesLikeRubyDirectoryLoader(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"empty.json": []byte(`{}`),
		"empty.yaml": []byte("{}\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var messages []string
	s := NewSession()
	s.logger = captureLogger(&messages, nil, nil)

	got, err := LoadExternalFacts(s, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts", got)
	}
	want := []string{
		fmt.Sprintf("Structured data fact file %s was parsed but was either empty or an invalid filetype (valid filetypes are .yaml, .json, and .txt).", filepath.Join(dir, "empty.yaml")),
		fmt.Sprintf("Structured data fact file %s was parsed but was either empty or an invalid filetype (valid filetypes are .yaml, .json, and .txt).", filepath.Join(dir, "empty.json")),
	}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("debug messages = %#v, want %#v", messages, want)
	}
}

func TestLoadExternalFacts_reportsUnsupportedVisibleFilesLikeRubyDirectoryLoader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.unknownfiletype")
	if err := os.WriteFile(path, []byte("stuff=bar\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var messages []string
	s := NewSession()
	s.logger = captureLogger(&messages, nil, nil)

	got, err := LoadExternalFacts(s, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts", got)
	}
	want := []string{
		fmt.Sprintf("Structured data fact file %s was parsed but was either empty or an invalid filetype (valid filetypes are .yaml, .json, and .txt).", path),
	}
	if !reflect.DeepEqual(messages, want) {
		t.Fatalf("debug messages = %#v, want %#v", messages, want)
	}
}

func TestLoadExternalFacts_skipsRubyFactFileWithWarningNamingTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fact.rb")
	if err := os.WriteFile(path, []byte("Facter.add(:rb_fact) do\n  setcode { 'x' }\nend\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var warnings []string
	s := NewSession()
	s.logger = captureLogger(nil, &warnings, nil)

	got, err := LoadExternalFacts(s, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want Ruby fact file unread", got)
	}
	want := []string{
		fmt.Sprintf("Ruby fact files are not supported by the Go port; skipping %s. Rewrite it as an executable external fact (see docs/CUSTOM_FACT_MIGRATION.md).", path),
	}
	if !reflect.DeepEqual(warnings, want) {
		t.Fatalf("warnings = %#v, want %#v", warnings, want)
	}
}

func TestLoadExternalFacts_ignoresUnreadableStaticFactFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"unreadable.txt":  []byte("txt_fact=ignored\n"),
		"unreadable.json": []byte(`{"json_fact":"ignored"}`),
		"unreadable.yaml": []byte("yaml_fact: ignored\n"),
		"site.txt":        []byte("site=lab\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	host := &fakeExternalFactLoaderHost{
		env: []string{},
		openFunc: func(path string) (io.ReadCloser, error) {
			if strings.HasPrefix(filepath.Base(path), "unreadable.") {
				return nil, os.ErrPermission
			}
			return externalFactOSHost{}.open(path)
		},
	}

	got, err := externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderCLI,
		dirs:       []string{dir},
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil", err)
	}
	want := []ResolvedFact{{Name: "site", Value: "lab", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_ignoresMalformedStructuredFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"good.txt": []byte("site=lab\n"),
		"bad.json": []byte(`{"broken":`),
		"bad.yaml": []byte("broken: [unterminated\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil for malformed structured files", err)
	}
	want := []ResolvedFact{{Name: "site", Value: "lab", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_matchesExtensionsCaseInsensitively(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"site.TXT":      []byte("txt_fact=loaded\n"),
		"metadata.JSON": []byte(`{"json_fact":"loaded"}`),
		"region.YAML":   []byte("yaml_fact: loaded\n"),
		"zone.YML":      []byte("yml_fact: loaded\n"),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"json_fact": "loaded",
		"txt_fact":  "loaded",
		"yaml_fact": "loaded",
		"yml_fact":  "loaded",
	}
	if len(got) != len(want) {
		t.Fatalf("LoadExternalFacts(testSession) len = %d, want %d: %#v", len(got), len(want), got)
	}
	for _, fact := range got {
		if fact.Type != "external" {
			t.Fatalf("fact %q type = %q, want external", fact.Name, fact.Type)
		}
		if fact.Value != want[fact.Name] {
			t.Fatalf("fact %q value = %#v, want %#v", fact.Name, fact.Value, want[fact.Name])
		}
		delete(want, fact.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing facts: %#v", want)
	}
}

func TestLoadExternalFacts_ignoresHiddenAndBackupFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"visible.txt":     "visible=true\n",
		".hidden.txt":     "hidden=true\n",
		"ignored.bak":     "backup=true\n",
		"ignored.orig":    "original=true\n",
		"ignored.txt.bak": "backup_txt=true\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "visible", Value: "true", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_txtFactsNormalizeNamesAndPreserveValueWhitespace(t *testing.T) {
	dir := t.TempDir()
	content := []byte(" Site_Location = lab \nOWNER = platform team\n")
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "site_location", Value: " lab ", Type: "external"},
		{Name: "owner", Value: " platform team", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_txtFactsPreserveValueWhitespaceLikeRubyParser(t *testing.T) {
	dir := t.TempDir()
	content := []byte("site= lab \n")
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "site", Value: " lab ", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_txtFactsIgnoreUTF8BOM(t *testing.T) {
	dir := t.TempDir()
	content := []byte("\xef\xbb\xbfsite_location=lab\nowner=platform\n")
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), content, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "site_location", Value: "lab", Type: "external"},
		{Name: "owner", Value: "platform", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_executableKeyValueFacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("extensionless shell-script external facts are a POSIX mechanism; Windows executables use .bat/.cmd/.exe/.ps1")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site_fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'script_one=two\\nscript_three=four=five\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "script_one", Value: "two", Type: "external"},
		{Name: "script_three", Value: "four=five", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_executableScriptPathWithSpacesMatchesRubyParser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("extensionless shell-script external facts are a POSIX mechanism; Windows executables use .bat/.cmd/.exe/.ps1")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'script_fact=loaded\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	host := &fakeExternalFactLoaderHost{
		goosValue: "linux",
		env:       []string{},
		runCommandFunc: func(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
			if len(args) != 0 {
				t.Fatalf("script args = %#v, want none", args)
			}
			return []byte("script_fact=loaded\n"), nil, nil
		},
	}

	got, err := externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderCLI,
		dirs:       []string{dir},
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "script_fact", Value: "loaded", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
	wantName := `"` + path + `"`
	if gotName := host.runCommandNames[0]; gotName != wantName {
		t.Fatalf("script command = %q, want %q", gotName, wantName)
	}
}

func TestLoadExternalFacts_skipsWindowsExecutableExtensionsOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows executable extensions are script facts on Windows")
	}

	dir := t.TempDir()
	for _, ext := range []string{".bat", ".cmd", ".exe", ".com"} {
		path := filepath.Join(dir, "windows_fact"+ext)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'windows_fact=loaded\\n'\n"), 0o700); err != nil {
			t.Fatal(err)
		}
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts from Windows executable extensions", got)
	}
}

func TestLoadExternalFacts_windowsScriptExtensionsDoNotRequireUnixExecutableBit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.bat")
	if err := os.WriteFile(path, []byte("win_fact=loaded\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := &fakeExternalFactLoaderHost{
		goosValue: "windows",
		env:       []string{},
		runCommandFunc: func(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
			if len(args) != 0 {
				t.Fatalf("script args = %#v, want none", args)
			}
			return []byte("win_fact=loaded\n"), nil, nil
		},
	}

	got, err := externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderCLI,
		dirs:       []string{dir},
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "win_fact", Value: "loaded", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
	if gotName := host.runCommandNames[0]; gotName != path {
		t.Fatalf("script command = %q, want %q", gotName, path)
	}
}

func TestLoadExternalFacts_windowsPowerShellFacts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.ps1")
	if err := os.WriteFile(path, []byte("Write-Output 'ps_fact=loaded'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := &fakeExternalFactLoaderHost{
		goosValue:        "windows",
		env:              []string{`SYSTEMROOT=C:\Windows`},
		fileReadableFunc: func(string) bool { return false },
		runCommandFunc: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
			return []byte("ps_fact=loaded\n"), nil, nil
		},
	}

	got, err := externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderCLI,
		dirs:       []string{dir},
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "ps_fact", Value: "loaded", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
	if gotName := host.runCommandNames[0]; gotName != "powershell.exe" {
		t.Fatalf("PowerShell command = %q, want powershell.exe", gotName)
	}
	wantArgs := []string{"-NoProfile", "-NonInteractive", "-NoLogo", "-ExecutionPolicy", "Bypass", "-File", path}
	if gotArgs := host.runCommandArgsets[0]; !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("PowerShell args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestLoadExternalFacts_windowsPowerShellExtensionIsCaseInsensitiveLikeRubyParser(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.PS1")
	if err := os.WriteFile(path, []byte("Write-Output 'ps_fact=loaded'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	host := &fakeExternalFactLoaderHost{
		goosValue:        "windows",
		env:              []string{`SYSTEMROOT=C:\Windows`},
		fileReadableFunc: func(string) bool { return false },
		runCommandFunc: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
			return []byte("ps_fact=loaded\n"), nil, nil
		},
	}

	got, err := externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderCLI,
		dirs:       []string{dir},
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "ps_fact", Value: "loaded", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
	wantArgs := []string{"-NoProfile", "-NonInteractive", "-NoLogo", "-ExecutionPolicy", "Bypass", "-File", path}
	if gotName := host.runCommandNames[0]; gotName != "powershell.exe" {
		t.Fatalf("PowerShell command = %q, want powershell.exe", gotName)
	}
	if gotArgs := host.runCommandArgsets[0]; !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("PowerShell args = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestLoadExternalFacts_windowsPowerShellSkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.ps1")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}

	host := &fakeExternalFactLoaderHost{
		goosValue: "windows",
		env:       []string{},
		runCommandFunc: func(_ context.Context, name string, args ...string) ([]byte, []byte, error) {
			t.Fatalf("PowerShell command ran for directory: %s %#v", name, args)
			return nil, nil, nil
		},
	}

	got, err := externalFactLoader{
		s:          testSession,
		mode:       externalFactLoaderCLI,
		dirs:       []string{dir},
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts from PowerShell directory", got)
	}
}

func TestLoadExternalFacts_windowsPowerShellWarnsWithRubyCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.ps1")
	if err := os.WriteFile(path, []byte("Write-Output 'ps_fact=loaded'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	warnings := []string{}
	s := NewSession()
	s.logger = captureLogger(nil, &warnings, nil)
	host := &fakeExternalFactLoaderHost{
		goosValue:        "windows",
		env:              []string{},
		fileReadableFunc: func(string) bool { return false },
		runCommandFunc: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
			return []byte("ps_fact=loaded\n"), []byte("some error\n"), nil
		},
	}

	got, err := externalFactLoader{
		s:          s,
		mode:       externalFactLoaderCLI,
		dirs:       []string{dir},
		host:       host,
		includeEnv: true,
	}.load()
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "ps_fact", Value: "loaded", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
	wantWarning := "Command \"powershell.exe\" -NoProfile -NonInteractive -NoLogo -ExecutionPolicy Bypass -File \"" + path + "\" completed with the following stderr message: some error"
	if !reflect.DeepEqual(warnings, []string{wantWarning}) {
		t.Fatalf("warnings = %#v, want %#v", warnings, []string{wantWarning})
	}
}

func TestCurrentPowerShellPathPrefersSysnativeThenSystem32(t *testing.T) {
	readable := map[string]bool{
		`C:\Windows\sysnative\WindowsPowershell\v1.0\powershell.exe`: true,
		`C:\Windows\system32\WindowsPowershell\v1.0\powershell.exe`:  true,
	}
	readableFunc := func(path string) bool { return readable[path] }

	got := currentPowerShellPath(`C:\Windows`, readableFunc)
	want := `C:\Windows\sysnative\WindowsPowershell\v1.0\powershell.exe`
	if got != want {
		t.Fatalf("currentPowerShellPath() = %q, want %q", got, want)
	}

	delete(readable, want)
	got = currentPowerShellPath(`C:\Windows`, readableFunc)
	want = `C:\Windows\system32\WindowsPowershell\v1.0\powershell.exe`
	if got != want {
		t.Fatalf("currentPowerShellPath() = %q, want %q", got, want)
	}
}

func TestLoadExternalFacts_executableInvalidOrEmptyOutputReturnsNoFacts(t *testing.T) {
	tests := []struct {
		name   string
		output string
	}{
		{name: "invalid", output: "random"},
		{name: "empty", output: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "site_fact")
			content := "#!/bin/sh\n"
			if tt.output != "" {
				content += "printf '" + tt.output + "'\n"
			}
			if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
				t.Fatal(err)
			}

			got, err := LoadExternalFacts(testSession, []string{dir})
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 0 {
				t.Fatalf("LoadExternalFacts(testSession) = %#v, want no facts", got)
			}
		})
	}
}

func TestLoadExternalFacts_executableWarnsWhenCommandWritesStderr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("extensionless shell-script external facts are a POSIX mechanism; Windows executables use .bat/.cmd/.exe/.ps1")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site_fact")
	content := "#!/bin/sh\nprintf 'script_one=two\\n'\nprintf 'some error' >&2\n"
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	warnings := []string{}
	s := NewSession()
	s.logger = captureLogger(nil, &warnings, nil)

	got, err := LoadExternalFacts(s, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "script_one", Value: "two", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want one warning", warnings)
	}
	if !strings.Contains(warnings[0], "Command "+path+" completed with the following stderr message: some error") {
		t.Fatalf("warning = %q, want command stderr warning", warnings[0])
	}
}

func TestLoadExternalFacts_ignoresFailedExecutableFact(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("site=lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "broken_fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'ignored=true\\n'\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil for failed executable fact", err)
	}
	want := []ResolvedFact{{Name: "site", Value: "lab", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_timesOutHungExecutableFact(t *testing.T) {
	oldTimeout := externalFactCommandTimeout
	externalFactCommandTimeout = 10 * time.Millisecond
	t.Cleanup(func() { externalFactCommandTimeout = oldTimeout })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("site=lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "hung_fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nsleep 10\nprintf 'ignored=true\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatalf("LoadExternalFacts(testSession) err = %v, want nil for timed out executable fact", err)
	}
	want := []ResolvedFact{{Name: "site", Value: "lab", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestExternalFactLoader_rejectsOversizedStructuredFactFile(t *testing.T) {
	oldLimit := externalFactMaxBytes
	externalFactMaxBytes = 8
	t.Cleanup(func() { externalFactMaxBytes = oldLimit })

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "huge.json"), []byte(`{"site":"larger than limit"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := externalFactLoader{
		s:    NewSession(),
		mode: externalFactLoaderLibrary,
		dirs: []string{dir},
	}.load()
	if !errors.Is(err, ErrExternalFactTooLarge) {
		t.Fatalf("load() err = %v, want ErrExternalFactTooLarge", err)
	}
}

func TestExternalFactLoader_reportsReadErrorsAfterOpen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "site.txt")
	if err := os.WriteFile(path, []byte("site=lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	readErr := errors.New("read exploded")
	host := &fakeExternalFactLoaderHost{
		openFunc: func(string) (io.ReadCloser, error) {
			return io.NopCloser(errorReader{err: readErr}), nil
		},
	}

	_, err := externalFactLoader{
		s:    NewSession(),
		mode: externalFactLoaderLibrary,
		dirs: []string{dir},
		host: host,
	}.load()
	if !errors.Is(err, readErr) {
		t.Fatalf("load() err = %v, want read error", err)
	}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func TestExternalFactLoader_rejectsOversizedExecutableFactOutput(t *testing.T) {
	dir := t.TempDir()
	// Windows classifies external executable facts by extension, not the mode
	// bit, so name the file accordingly to exercise the executable path there.
	name := "huge_fact"
	if runtime.GOOS == "windows" {
		name = "huge_fact.bat"
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'site=larger-than-limit\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	host := &fakeExternalFactLoaderHost{
		runCommandFunc: func(context.Context, string, ...string) ([]byte, []byte, error) {
			return []byte("site=lar"), nil, ErrExternalFactTooLarge
		},
	}

	_, err := externalFactLoader{
		s:    NewSession(),
		mode: externalFactLoaderLibrary,
		dirs: []string{dir},
		host: host,
	}.load()
	if !errors.Is(err, ErrExternalFactTooLarge) {
		t.Fatalf("load() err = %v, want ErrExternalFactTooLarge", err)
	}
}

func TestRunExternalFactCommand_rejectsOversizedStdout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses /bin/sh")
	}
	oldLimit := externalFactMaxBytes
	externalFactMaxBytes = 8
	t.Cleanup(func() { externalFactMaxBytes = oldLimit })

	out, stderr, err := runExternalFactCommand(context.Background(), "/bin/sh", "-c", "printf 'site=larger-than-limit\\n'")
	if !errors.Is(err, ErrExternalFactTooLarge) {
		t.Fatalf("runExternalFactCommand() stdout=%q stderr=%q err=%v, want ErrExternalFactTooLarge", out, stderr, err)
	}
}

func TestLimitedBuffer_marksOverflowAndRetainsPrefix(t *testing.T) {
	var buf limitedBuffer
	buf.limit = 8

	n, err := buf.Write([]byte("site=larger-than-limit\n"))
	if !errors.Is(err, ErrExternalFactTooLarge) {
		t.Fatalf("Write() err = %v, want ErrExternalFactTooLarge", err)
	}
	if n != 8 {
		t.Fatalf("Write() n = %d, want retained byte count", n)
	}
	if !buf.tooLarge {
		t.Fatal("limitedBuffer did not mark overflow")
	}
	if got := string(buf.Bytes()); got != "site=lar" {
		t.Fatalf("limitedBuffer bytes = %q, want retained prefix", got)
	}
}

func TestLoadExternalFacts_executableYAMLFacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("extensionless shell-script external facts are a POSIX mechanism; Windows executables use .bat/.cmd/.exe/.ps1")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site_fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'script_one: two\\nscript_three: four\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "script_one", Value: "two", Type: "external"},
		{Name: "script_three", Value: "four", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_executableYAMLSymbolFacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("extensionless shell-script external facts are a POSIX mechanism; Windows executables use .bat/.cmd/.exe/.ps1")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site_fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf -- '---\\n:script_one: :two\\nscript_three: four\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "script_one", Value: "two", Type: "external"},
		{Name: "script_three", Value: "four", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_executableYAMLTimestampNormalizesLikeRubyParser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("extensionless shell-script external facts are a POSIX mechanism; Windows executables use .bat/.cmd/.exe/.ps1")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site_fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf -- '---\\nfirst: 2020-07-15 05:38:12.427678398 +00:00\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "first", Value: "2020-07-15T05:38:12Z", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_executableJSONFacts(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("extensionless shell-script external facts are a POSIX mechanism; Windows executables use .bat/.cmd/.exe/.ps1")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "site_fact")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '{\"script_one\":\"two\",\"script_count\":3}\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{
		{Name: "script_count", Value: 3, Type: "external"},
		{Name: "script_one", Value: "two", Type: "external"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_skipsExecutableFactsDuringRecursiveResolution(t *testing.T) {
	t.Setenv(externalFactResolutionEnv, "1")
	dir := t.TempDir()
	executablePath := filepath.Join(dir, "dynamic_fact")
	if err := os.WriteFile(executablePath, []byte("#!/bin/sh\nprintf 'dynamic=true\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	staticPath := filepath.Join(dir, "site.txt")
	if err := os.WriteFile(staticPath, []byte("static=true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadExternalFacts(testSession, []string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []ResolvedFact{{Name: "static", Value: "true", Type: "external"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("LoadExternalFacts(testSession) = %#v, want %#v", got, want)
	}
}

func TestLoadExternalFacts_rejectsNullBytes(t *testing.T) {
	tests := []struct {
		name      string
		fileName  string
		content   []byte
		mode      os.FileMode
		posixOnly bool
	}{
		{name: "txt key", fileName: "bad.txt", content: []byte("bad\x00key=value\n"), mode: 0o600},
		{name: "txt value", fileName: "bad.txt", content: []byte("good=bad\x00value\n"), mode: 0o600},
		{name: "json key", fileName: "bad.json", content: []byte(`{"bad\u0000key":"value"}`), mode: 0o600},
		{name: "json value", fileName: "bad.json", content: []byte(`{"good":"bad\u0000value"}`), mode: 0o600},
		{name: "yaml key", fileName: "bad.yaml", content: []byte("bad\x00key: value\n"), mode: 0o600},
		{name: "yaml value", fileName: "bad.yaml", content: []byte("good: bad\x00value\n"), mode: 0o600},
		{name: "executable yaml", fileName: "bad_fact", content: []byte("#!/bin/sh\nprintf 'good: bad\\0value\\n'\n"), mode: 0o700, posixOnly: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.posixOnly && runtime.GOOS == "windows" {
				t.Skip("extensionless executable external facts are a POSIX mechanism")
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tt.fileName), tt.content, tt.mode); err != nil {
				t.Fatal(err)
			}

			_, err := LoadExternalFacts(testSession, []string{dir})
			if !errors.Is(err, ErrNullByte) {
				t.Fatalf("LoadExternalFacts(testSession) err = %v, want ErrNullByte", err)
			}
		})
	}
}

func TestExternalFactGroups_includesEveryExternalDirectoryEntry(t *testing.T) {
	dir := t.TempDir()
	entries := map[string]os.FileMode{
		"external.sh": 0o700,
		".hidden":     0o600,
		"ignored.bak": 0o600,
	}
	for name, mode := range entries {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("ignored=true\n"), mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := ExternalFactGroups([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	want := []FactGroup{
		{Name: ".hidden"},
		{Name: "external.sh"},
		{Name: "ignored.bak"},
		{Name: "nested"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExternalFactGroups() = %#v, want %#v", got, want)
	}
}

func BenchmarkLoadExternalFacts(b *testing.B) {
	dir := b.TempDir()
	files := map[string]string{
		"site.txt":      "site=lab\nowner=platform=team\n",
		"metadata.json": `{"roles":["web","db"],"enabled":true,"count":3}`,
		"region.yaml":   "region: us-west\nnested:\n  enabled: true\n",
		"ignored.bak":   "ignored=true\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			b.Fatal(err)
		}
	}

	b.ReportAllocs()
	for b.Loop() {
		facts, err := LoadExternalFacts(testSession, []string{dir})
		if err != nil {
			b.Fatal(err)
		}
		if len(facts) != 7 {
			b.Fatalf("LoadExternalFacts(testSession) len = %d, want 7", len(facts))
		}
	}
}
