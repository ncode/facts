package facts

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_gateableCoreFactsPreservePublicPruningAndQueries(t *testing.T) {
	tests := []struct {
		name      string
		disabled  string
		keptQuery string
		pruned    string
		wantWarn  string
	}{
		{
			name:      "multi-output sibling remains resolved",
			disabled:  `"filesystems", "os", "system_profiler"`,
			keptQuery: "kernel",
			pruned:    "os",
			wantWarn:  `fact "os" is disabled by the configuration file`,
		},
		{
			name:      "disabled subfact is pruned after resolution",
			disabled:  `"os.release"`,
			keptQuery: "os.name",
			pruned:    "os.release",
			wantWarn:  `fact "os.release" is disabled by the configuration file`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := writeTestFile(t, t.TempDir(), "facter.conf", "facts : {\n  blocklist : [ "+tt.disabled+" ],\n}\n")
			handler := &recordingHandler{}
			eng, err := New(WithConfigFile(config), WithLogger(slog.New(handler)))
			if err != nil {
				t.Fatal(err)
			}

			snap, err := eng.Discover(context.Background(), tt.keptQuery, tt.pruned)
			if err != nil {
				t.Fatalf("Discover() err = %v", err)
			}
			if _, err := snap.Value(tt.keptQuery); err != nil {
				t.Fatalf("Value(%q) err = %v, want kept sibling resolved", tt.keptQuery, err)
			}
			if _, err := snap.Value(tt.pruned); !errors.Is(err, ErrFactNotFound) {
				t.Fatalf("Value(%q) err = %v, want ErrFactNotFound", tt.pruned, err)
			}
			if !handler.hasWarn(tt.wantWarn) {
				t.Fatalf("diagnostics = %#v, want WARN %q", handler.records, tt.wantWarn)
			}
		})
	}
}

func TestDiscover_fullyDisabledCoreResolverSkipsCacheConsultation(t *testing.T) {
	dir, defaults := cacheDefaults(t)
	cachePath := filepath.Join(dir, "operating system")
	if err := os.WriteFile(cachePath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := writeTestFile(t, t.TempDir(), "facter.conf", `facts : {
  blocklist : [ "filesystems", "kernel", "os", "system_profiler" ],
  ttls : [ { "operating system" : "30 days" } ],
}
`)
	handler := &recordingHandler{}
	eng, err := New(WithCache(), WithConfigFile(config), WithLogger(slog.New(handler)), defaults)
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(context.Background(), "kernel")
	if err != nil {
		t.Fatalf("Discover() err = %v", err)
	}
	if _, err := snap.Value("kernel"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(kernel) err = %v, want ErrFactNotFound", err)
	}
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("malformed cache was consulted and removed: %v", err)
	}
	if want := `fact "kernel" is disabled by the configuration file`; !handler.hasWarn(want) {
		t.Fatalf("diagnostics = %#v, want WARN %q", handler.records, want)
	}
}
