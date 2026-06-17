package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkFactCacheResolveFacts(b *testing.B) {
	dir := b.TempDir()
	cachePath := filepath.Join(dir, "networking")
	writeBenchmarkJSONFile(b, cachePath, map[string]any{
		"cache_format_version": float64(1),
		"networking.hostname":  "node.example",
		"networking.domain":    "example",
		"networking.ip":        "192.0.2.10",
	})
	cache := NewFactCache(dir, []FactTTL{{Fact: "networking", TTL: "1 hour"}}, nil, discardLog())
	searched := []ResolvedFact{
		{Name: "networking.hostname", Type: "core"},
		{Name: "networking.domain", Type: "core"},
		{Name: "networking.ip", Type: "core"},
		{Name: "site_role", Type: "custom"},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		remaining, cached := cache.ResolveFacts(searched)
		if len(remaining) != 1 || len(cached) != 3 {
			b.Fatalf("remaining=%d cached=%d", len(remaining), len(cached))
		}
	}
}

func BenchmarkCacheDataHasKeyMatchingFact(b *testing.B) {
	data := map[string]any{
		"cache_format_version": float64(1),
		"networking.hostname":  "node.example",
		"networking.domain":    "example",
		"networking.ip":        "192.0.2.10",
		"networking.ip6":       "2001:db8::10",
		"networking.mac":       "00:00:5e:00:53:01",
	}

	b.ReportAllocs()
	for b.Loop() {
		if !cacheDataHasKeyMatchingFact(data, "networking.hostname") {
			b.Fatal("expected match")
		}
	}
}

func writeBenchmarkJSONFile(tb testing.TB, path string, data map[string]any) {
	tb.Helper()
	encoded, err := json.Marshal(data)
	if err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		tb.Fatal(err)
	}
}
