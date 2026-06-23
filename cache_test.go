package facts

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ncode/facts/internal/engine"
)

// redirectCacheDir points the engine's persistent-cache location at a temp dir
// for the duration of the test, so WithCache exercises a real round trip
// without reading or writing the host's actual fact cache.
func redirectCacheDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	original := engine.DefaultCachePath
	engine.DefaultCachePath = func() string { return dir }
	t.Cleanup(func() { engine.DefaultCachePath = original })
	return dir
}

// writeTTLConfig writes a config file giving group a 30-day TTL, which is what
// opts that group into the cache.
func writeTTLConfig(t *testing.T, group string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "facter.conf")
	content := "facts : {\n  ttls : [\n    { \"" + group + "\" : \"30 days\" }\n  ],\n}"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readCacheFile(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cache file %s: %v", path, err)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("decode cache file %s: %v", path, err)
	}
	return data
}

func seedCacheFile(t *testing.T, path string, data map[string]any) {
	t.Helper()
	data["cache_format_version"] = 1
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func cachingResolver(value string) func(context.Context) (any, error) {
	return func(context.Context) (any, error) { return value, nil }
}

// TestWithCache_persistsResolvedFactToDisk proves the write half of the cache
// contract: opting in with WithCache and a TTL'd group causes Discover to
// persist the freshly resolved fact to the cache directory.
func TestWithCache_persistsResolvedFactToDisk(t *testing.T) {
	dir := redirectCacheDir(t)
	conf := writeTTLConfig(t, "demo")

	eng, err := New(WithCache(), WithConfigFile(conf), WithFact("demo", cachingResolver("fresh")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := eng.Discover(context.Background()); err != nil {
		t.Fatalf("Discover() err = %v", err)
	}

	data := readCacheFile(t, filepath.Join(dir, "demo"))
	if data["demo"] != "fresh" {
		t.Fatalf("cached demo = %#v, want %q — WithCache should persist the resolved fact", data["demo"], "fresh")
	}
}

// TestWithCache_servesFreshCachedValueOverResolver proves the read half: when a
// fresh cache entry exists, WithCache returns the cached value instead of the
// freshly resolved one. A different resolver value makes the substitution
// observable.
func TestWithCache_servesFreshCachedValueOverResolver(t *testing.T) {
	dir := redirectCacheDir(t)
	conf := writeTTLConfig(t, "demo")
	seedCacheFile(t, filepath.Join(dir, "demo"), map[string]any{"demo": "from-cache"})

	eng, err := New(WithCache(), WithConfigFile(conf), WithFact("demo", cachingResolver("fresh")))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v", err)
	}

	got, err := snap.Value("demo")
	if err != nil {
		t.Fatalf("Value(demo) err = %v", err)
	}
	if got != "from-cache" {
		t.Fatalf("demo = %#v, want %q — a fresh cache entry must win over the resolver", got, "from-cache")
	}
}

func TestWithCache_selectsQueriedFactsThroughEngineCachePath(t *testing.T) {
	dir := redirectCacheDir(t)
	conf := writeTTLConfig(t, "demo")
	seedCacheFile(t, filepath.Join(dir, "demo"), map[string]any{"demo": map[string]any{"child": "from-cache"}})

	eng, err := New(
		WithCache(),
		WithConfigFile(conf),
		WithFact("demo", func(context.Context) (any, error) {
			return map[string]any{"child": "fresh"}, nil
		}),
		WithFact("other", cachingResolver("fresh")),
	)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background(), "demo.child")
	if err != nil {
		t.Fatalf("Discover() err = %v", err)
	}
	if got, err := snap.Value("demo.child"); err != nil || got != "from-cache" {
		t.Fatalf("Value(demo.child) = %#v, %v, want queried cached value", got, err)
	}
	if _, err := snap.Value("other"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(other) err = %v, want unqueried fact omitted from cached Snapshot", err)
	}
}

// TestWithoutCache_ignoresExistingCache is the toggle control: the same seeded
// cache that WithCache would serve is ignored when WithCache is absent, proving
// the option — not some always-on path — is what enables caching.
func TestWithoutCache_ignoresExistingCache(t *testing.T) {
	dir := redirectCacheDir(t)
	conf := writeTTLConfig(t, "demo")
	seedCacheFile(t, filepath.Join(dir, "demo"), map[string]any{"demo": "from-cache"})

	eng, err := New(WithConfigFile(conf), WithFact("demo", cachingResolver("fresh")))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v", err)
	}

	got, err := snap.Value("demo")
	if err != nil {
		t.Fatalf("Value(demo) err = %v", err)
	}
	if got != "fresh" {
		t.Fatalf("demo = %#v, want %q — without WithCache the seeded cache must be ignored", got, "fresh")
	}
}
