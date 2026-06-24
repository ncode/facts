package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestFactCache_resolvesFreshCachedFactAndSkipsSearch(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "operating system")
	writeJSONFile(t, cachePath, map[string]any{
		"cache_format_version": float64(1),
		"os":                   "Ubuntu",
	})

	cache := NewFactCache(dir, []FactTTL{{Fact: "operating system", TTL: "1 hour"}}, nil, discardLog())
	searched := []ResolvedFact{{Name: "os", Type: "core"}, {Name: "site_role", Type: "custom"}}

	remaining, cached := cache.ResolveFacts(searched)

	if want := []ResolvedFact{{Name: "site_role", Type: "custom"}}; !reflect.DeepEqual(remaining, want) {
		t.Fatalf("remaining = %#v, want %#v", remaining, want)
	}
	if want := []ResolvedFact{{Name: "os", Value: "Ubuntu", Type: "core"}}; !reflect.DeepEqual(cached, want) {
		t.Fatalf("cached = %#v, want %#v", cached, want)
	}
}

func TestPlatformDefaultCachePathForSupportedPlatforms(t *testing.T) {
	tests := []struct {
		name        string
		goos        string
		programData string
		appData     string
		want        string
	}{
		{
			name: "linux",
			goos: "linux",
			want: "/opt/puppetlabs/facts/cache/cached_facts",
		},
		{
			name: "darwin",
			goos: "darwin",
			want: "/opt/puppetlabs/facts/cache/cached_facts",
		},
		{
			name: "freebsd",
			goos: "freebsd",
			want: "/opt/puppetlabs/facts/cache/cached_facts",
		},
		{
			name:        "windows ProgramData",
			goos:        "windows",
			programData: `C:\ProgramData`,
			want:        `C:\ProgramData/PuppetLabs/facts/cache/cached_facts`,
		},
		{
			name:    "windows APPDATA fallback",
			goos:    "windows",
			appData: `C:\Users\Alice\AppData\Roaming`,
			want:    `C:\Users\Alice\AppData\Roaming/PuppetLabs/facts/cache/cached_facts`,
		},
		{
			name: "windows without data dir",
			goos: "windows",
			want: "/opt/puppetlabs/facts/cache/cached_facts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := platformDefaultCachePathFor(tt.goos, tt.programData, tt.appData)
			if got != tt.want {
				t.Fatalf("platformDefaultCachePathFor(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}

func TestFactCache_resolvesExternalFactFromFileBasenameCacheGroup(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "ext_file.txt")
	writeJSONFile(t, cachePath, map[string]any{
		"cache_format_version": float64(1),
		"my_external_fact":     "ext_fact",
		"other_external_fact":  "other_ext_fact",
	})

	cache := NewFactCache(dir, []FactTTL{{Fact: "ext_file.txt", TTL: "1 hour"}}, nil, discardLog())
	searched := []ResolvedFact{{Name: "my_external_fact", Type: "file", File: "/tmp/ext_file.txt"}}

	remaining, cached := cache.ResolveFacts(searched)

	if len(remaining) != 0 {
		t.Fatalf("remaining = %#v, want none", remaining)
	}
	if want := []ResolvedFact{
		{Name: "my_external_fact", Value: "ext_fact", Type: "file", File: "/tmp/ext_file.txt"},
		{Name: "other_external_fact", Value: "other_ext_fact", Type: "file", File: "/tmp/ext_file.txt"},
	}; !reflect.DeepEqual(cached, want) {
		t.Fatalf("cached = %#v, want %#v", cached, want)
	}
}

func TestFactCache_refusesExternalFactFromCustomGroupLikeRubyCacheManager(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "cached-custom-facts")
	writeJSONFile(t, cachePath, map[string]any{
		"cache_format_version": float64(1),
		"my_external_fact":     "ext_fact",
	})
	errors := []string{}
	logger := captureLogger(nil, nil, &errors)

	cache := NewFactCache(
		dir,
		[]FactTTL{{Fact: "cached-custom-facts", TTL: "1 hour"}},
		[]FactGroup{{Name: "cached-custom-facts", Facts: []string{"ext_file.txt"}}},
		logger,
	)
	searched := []ResolvedFact{{Name: "my_external_fact", Type: "file", File: "/tmp/ext_file.txt"}}

	remaining, cached := cache.ResolveFacts(searched)

	if !reflect.DeepEqual(remaining, searched) {
		t.Fatalf("remaining = %#v, want searched fact", remaining)
	}
	if len(cached) != 0 {
		t.Fatalf("cached = %#v, want none", cached)
	}
	wantError := "Cannot cache 'ext_file.txt' fact from 'cached-custom-facts' group. Caching custom group is not supported for external facts."
	if !reflect.DeepEqual(errors, []string{wantError}) {
		t.Fatalf("errors = %#v, want %#v", errors, []string{wantError})
	}
}

func TestFactCache_ignoresExpiredCacheAndDeletesFile(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "operating system")
	writeJSONFile(t, cachePath, map[string]any{
		"cache_format_version": float64(1),
		"os":                   "Ubuntu",
	})
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(cachePath, old, old); err != nil {
		t.Fatal(err)
	}

	cache := NewFactCache(dir, []FactTTL{{Fact: "operating system", TTL: "1 hour"}}, nil, discardLog())
	searched := []ResolvedFact{{Name: "os", Type: "core"}}

	remaining, cached := cache.ResolveFacts(searched)

	if !reflect.DeepEqual(remaining, searched) {
		t.Fatalf("remaining = %#v, want searched fact", remaining)
	}
	if len(cached) != 0 {
		t.Fatalf("cached = %#v, want none", cached)
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("cache file exists after expiry, stat err = %v", err)
	}
}

func TestFactCache_resolveFactsDeletesCacheWhenSearchedCoreFactMissingLikeRubyCacheManager(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "operating system")
	writeJSONFile(t, cachePath, map[string]any{
		"cache_format_version": float64(1),
		"memory":               "1 GiB",
	})

	cache := NewFactCache(dir, []FactTTL{{Fact: "operating system", TTL: "1 hour"}}, nil, discardLog())
	searched := []ResolvedFact{{Name: "os", Type: "core"}}

	remaining, cached := cache.ResolveFacts(searched)

	if !reflect.DeepEqual(remaining, searched) {
		t.Fatalf("remaining = %#v, want searched fact", remaining)
	}
	if len(cached) != 0 {
		t.Fatalf("cached = %#v, want none", cached)
	}
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Fatalf("cache file still exists after missing fact, stat err = %v", err)
	}
}

func TestFactCache_cacheFactsWritesConfiguredGroups(t *testing.T) {
	dir := t.TempDir()
	cache := NewFactCache(dir, []FactTTL{{Fact: "networking", TTL: "30 minutes"}}, nil, discardLog())

	if err := cache.CacheFacts([]ResolvedFact{
		{Name: "networking.hostname", Value: "node.example", Type: "core"},
		{Name: "site_role", Value: "web", Type: "custom"},
	}); err != nil {
		t.Fatal(err)
	}

	data := readJSONFile(t, filepath.Join(dir, "networking"))
	if data["cache_format_version"] != float64(1) {
		t.Fatalf("cache_format_version = %#v, want 1", data["cache_format_version"])
	}
	if data["networking.hostname"] != "node.example" {
		t.Fatalf("networking.hostname = %#v, want node.example", data["networking.hostname"])
	}
	if _, ok := data["site_role"]; ok {
		t.Fatalf("site_role was cached in networking group: %#v", data)
	}
}

func TestFactCache_cacheFactsWritesExternalFileBasenameGroup(t *testing.T) {
	dir := t.TempDir()
	cache := NewFactCache(dir, []FactTTL{{Fact: "ext_file.txt", TTL: "1 hour"}}, nil, discardLog())

	if err := cache.CacheFacts([]ResolvedFact{
		{Name: "my_external_fact", Value: "ext_fact", Type: "file", File: "/tmp/ext_file.txt"},
	}); err != nil {
		t.Fatal(err)
	}

	data := readJSONFile(t, filepath.Join(dir, "ext_file.txt"))
	if data["cache_format_version"] != float64(1) {
		t.Fatalf("cache_format_version = %#v, want 1", data["cache_format_version"])
	}
	if data["my_external_fact"] != "ext_fact" {
		t.Fatalf("my_external_fact = %#v, want ext_fact", data["my_external_fact"])
	}
}

func TestWriteCacheFileWritesFinalFileAndRemovesTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "facts.cache")

	if err := writeCacheFile(path, []byte("cached"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "cached" {
		t.Fatalf("cache file content = %q, want cached", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); runtime.GOOS != "windows" && got != 0o600 {
		t.Fatalf("cache file mode = %v, want 0600", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "facts.cache" {
		t.Fatalf("cache dir entries = %#v, want only final cache file", entries)
	}
}

func TestFactCache_ignoresUnsafeCacheGroupNames(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(dir, "..", "outside-cache")
	cache := NewFactCache(
		dir,
		[]FactTTL{{Fact: "../outside-cache", TTL: "1 hour"}},
		nil,
		discardLog(),
	)

	if err := cache.CacheFacts([]ResolvedFact{{Name: "../outside-cache", Value: "pwned", Type: "external"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("outside cache path stat err = %v, want not exist", err)
	}
}

func TestSafeCacheGroupNameRejectsWindowsSpecialNames(t *testing.T) {
	unsafe := []string{"CON", "nul.txt", "COM1", "LPT9.log", "name.", "name "}
	for _, name := range unsafe {
		t.Run(name, func(t *testing.T) {
			if safeCacheGroupName(name) {
				t.Fatalf("safeCacheGroupName(%q) = true, want false", name)
			}
		})
	}
}

func TestFactCache_cacheFactsWarnsWhenCacheFileCannotBeWrittenLikeRubyCacheManager(t *testing.T) {
	dir := t.TempDir()
	originalWriteFile := cacheWriteFile
	cacheWriteFile = func(string, []byte, os.FileMode) error { return os.ErrPermission }
	t.Cleanup(func() {
		cacheWriteFile = originalWriteFile
	})
	warnings := []string{}
	logger := captureLogger(nil, &warnings, nil)
	cache := NewFactCache(dir, []FactTTL{{Fact: "operating system", TTL: "1 hour"}}, nil, logger)

	if err := cache.CacheFacts([]ResolvedFact{{Name: "os", Value: "Ubuntu", Type: "core"}}); err != nil {
		t.Fatalf("CacheFacts() err = %v, want nil", err)
	}

	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want one warning", warnings)
	}
	if !strings.Contains(warnings[0], "Could not write cache: ") {
		t.Fatalf("warning = %q, want cache write warning", warnings[0])
	}
}

func TestFactCache_cacheFactsOverwritesInvalidFreshCacheLikeRubyCacheManager(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "operating system")
	if err := os.WriteFile(cachePath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	debugMessages := []string{}
	logger := captureLogger(&debugMessages, nil, nil)

	cache := NewFactCache(dir, []FactTTL{{Fact: "operating system", TTL: "1 hour"}}, nil, logger)

	if err := cache.CacheFacts([]ResolvedFact{{Name: "os", Value: "Ubuntu", Type: "core"}}); err != nil {
		t.Fatal(err)
	}

	data := readJSONFile(t, cachePath)
	if data["os"] != "Ubuntu" {
		t.Fatalf("cached os = %#v, want Ubuntu", data["os"])
	}
	if data["cache_format_version"] != float64(1) {
		t.Fatalf("cache_format_version = %#v, want 1", data["cache_format_version"])
	}
	wantDebug := "Failed to read cache file " + cachePath + ". Detail:"
	foundDebug := false
	for _, message := range debugMessages {
		if strings.Contains(message, wantDebug) {
			foundDebug = true
			break
		}
	}
	if !foundDebug {
		t.Fatalf("debug messages = %#v, want one containing %q", debugMessages, wantDebug)
	}
}

func TestFactCache_cacheFactsLogsNoKeysForNonObjectFreshCacheLikeRubyCacheManager(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "operating system")
	if err := os.WriteFile(cachePath, []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	debugMessages := []string{}
	logger := captureLogger(&debugMessages, nil, nil)
	cache := NewFactCache(dir, []FactTTL{{Fact: "operating system", TTL: "1 hour"}}, nil, logger)

	if err := cache.CacheFacts([]ResolvedFact{{Name: "os", Value: "Ubuntu", Type: "core"}}); err != nil {
		t.Fatal(err)
	}

	data := readJSONFile(t, cachePath)
	if data["os"] != "Ubuntu" {
		t.Fatalf("cached os = %#v, want Ubuntu", data["os"])
	}
	wantDebug := "No keys found in " + cachePath + ". Detail:"
	foundDebug := false
	for _, message := range debugMessages {
		if strings.Contains(message, wantDebug) {
			foundDebug = true
			break
		}
	}
	if !foundDebug {
		t.Fatalf("debug messages = %#v, want one containing %q", debugMessages, wantDebug)
	}
}

func TestFactCache_resolveFactsWarnsWhenCorruptCacheCannotBeDeletedLikeRubyCacheManager(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "ext_file.txt")
	if err := os.WriteFile(cachePath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalRemove := cacheRemove
	cacheRemove = func(string) error { return os.ErrPermission }
	t.Cleanup(func() {
		cacheRemove = originalRemove
	})
	warnings := []string{}
	logger := captureLogger(nil, &warnings, nil)

	cache := NewFactCache(dir, []FactTTL{{Fact: "ext_file.txt", TTL: "1 hour"}}, nil, logger)
	searched := []ResolvedFact{{Name: "my_external_fact", Type: "file", File: "/tmp/ext_file.txt"}}

	remaining, cached := cache.ResolveFacts(searched)

	if !reflect.DeepEqual(remaining, searched) {
		t.Fatalf("remaining = %#v, want searched fact", remaining)
	}
	if len(cached) != 0 {
		t.Fatalf("cached = %#v, want none", cached)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %#v, want one warning", warnings)
	}
	if !strings.Contains(warnings[0], "Could not delete cache: ") {
		t.Fatalf("warning = %q, want cache delete warning", warnings[0])
	}
}

func TestParseTTLDuration_matchesRubyUnits(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{input: "10000", want: 10 * time.Second},
		{input: "30 h", want: 30 * time.Hour},
		{input: "1 hour", want: time.Hour},
		{input: "1 day", want: 24 * time.Hour},
		{input: "10 ns", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := parseTTLDuration(tt.input)
			if !ok {
				t.Fatalf("parseTTLDuration(%q) ok = false, want true", tt.input)
			}
			if got != tt.want {
				t.Fatalf("parseTTLDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseTTLDuration_rejectsNegativeAndOverflowingValues(t *testing.T) {
	tooManyHours := strconv.FormatInt(int64((time.Duration(1<<63-1)/time.Hour)+1), 10) + " h"
	for _, input := range []string{"-1 seconds", tooManyHours} {
		t.Run(input, func(t *testing.T) {
			if got, ok := parseTTLDuration(input); ok {
				t.Fatalf("parseTTLDuration(%q) = %v, true; want false", input, got)
			}
		})
	}
}

func writeJSONFile(t *testing.T, path string, data map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readJSONFile(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}
