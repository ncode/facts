package engine

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sync"
	"testing"
)

func TestEngineDiscover_isolatesInvocationDefaults(t *testing.T) {
	t.Parallel()

	type fixture struct {
		name       string
		factValue  string
		configPath string
		cachePath  string
		external   []string
		engine     *Engine
	}
	newFixture := func(name string) fixture {
		t.Helper()
		firstDir := t.TempDir()
		secondDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(firstDir, "first.json"), []byte(`{"order_`+name+`_first":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(secondDir, "second.json"), []byte(`{"order_`+name+`_second":true}`), 0o600); err != nil {
			t.Fatal(err)
		}
		configPath := filepath.Join(t.TempDir(), "facts.conf")
		config := `facts: { ttls: [ { "cache_probe": "30 days" } ] }`
		if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
		cachePath := t.TempDir()
		defaults := DiscoveryDefaults{
			NativeConfigPath: configPath,
			CachePath:        cachePath,
			ExternalFactDirs: []string{firstDir, secondDir},
		}
		eng, err := NewEngine(EngineConfig{
			SystemDefaults: true,
			UseCache:       true,
			Defaults:       &defaults,
			Facts: []ProgrammaticFact{{
				Name: "cache_probe",
				Resolve: func(context.Context) (any, error) {
					return name, nil
				},
			}},
		})
		if err != nil {
			t.Fatal(err)
		}
		defaults.ExternalFactDirs[0] = t.TempDir()
		return fixture{name: name, factValue: name, configPath: configPath, cachePath: cachePath, external: []string{firstDir, secondDir}, engine: eng}
	}

	fixtures := []fixture{newFixture("alpha"), newFixture("beta")}
	var wg sync.WaitGroup
	for i := range fixtures {
		f := &fixtures[i]
		wg.Go(func() {
			plan, failures := f.engine.planDiscovery(NewSession(), nil)
			if len(failures) != 0 {
				t.Errorf("%s plan failures = %v", f.name, failures)
				return
			}
			if plan.cachePath != f.cachePath {
				t.Errorf("%s cache path = %q, want %q", f.name, plan.cachePath, f.cachePath)
			}
			if !reflect.DeepEqual(plan.externalDirs, f.external) {
				t.Errorf("%s external dirs = %#v, want ordered %#v", f.name, plan.externalDirs, f.external)
			}
			snapshot, err := f.engine.Discover(t.Context(), "cache_probe")
			if err != nil {
				t.Errorf("%s Discover() error = %v", f.name, err)
				return
			}
			if got, err := snapshot.Value("cache_probe"); err != nil || got != f.factValue {
				t.Errorf("%s cache_probe = %#v, %v, want %q", f.name, got, err, f.factValue)
			}
		})
	}
	wg.Wait()

	for _, f := range fixtures {
		data, err := os.ReadFile(filepath.Join(f.cachePath, "cache_probe"))
		if err != nil {
			t.Fatalf("read %s cache: %v", f.name, err)
		}
		var cached map[string]any
		if err := json.Unmarshal(data, &cached); err != nil {
			t.Fatalf("decode %s cache: %v", f.name, err)
		}
		if got := cached["cache_probe"]; got != f.factValue {
			t.Fatalf("%s cached value = %#v, want %q", f.name, got, f.factValue)
		}
	}
}

func TestPlanDiscovery_rereadsInvocationConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "facts.conf")
	writeConfig := func(disabled string) {
		t.Helper()
		if err := os.WriteFile(configPath, []byte(`global: { disable: ["`+disabled+`"] }`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig("first")
	defaults := DiscoveryDefaults{NativeConfigPath: configPath}
	eng, err := NewEngine(EngineConfig{SystemDefaults: true, Defaults: &defaults})
	if err != nil {
		t.Fatal(err)
	}

	first, failures := eng.planDiscovery(NewSession(), nil)
	if len(failures) != 0 {
		t.Fatalf("first plan failures = %v", failures)
	}
	writeConfig("second")
	second, failures := eng.planDiscovery(NewSession(), nil)
	if len(failures) != 0 {
		t.Fatalf("second plan failures = %v", failures)
	}
	if !first.disabledFacts["first"] || first.disabledFacts["second"] {
		t.Fatalf("first disabled facts = %#v, want only first", first.disabledFacts)
	}
	if !second.disabledFacts["second"] || second.disabledFacts["first"] {
		t.Fatalf("second disabled facts = %#v, want only second", second.disabledFacts)
	}
}

func TestEngineDiscover_nilDefaultsRederivesAmbientDefaults(t *testing.T) {
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		t.Skip("root defaults do not depend on the process environment")
	}

	roots := []string{t.TempDir(), t.TempDir()}
	wants := []string{"first", "second"}
	for i, root := range roots {
		dir := filepath.Join(root, ".facts", "facts.d")
		if runtime.GOOS == "windows" {
			dir = filepath.Join(root, "facts", "facts.d")
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		content := []byte("ambient_defaults_probe: " + wants[i] + "\n")
		if err := os.WriteFile(filepath.Join(dir, "ambient.yaml"), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	ambientName := "HOME"
	if runtime.GOOS == "windows" {
		ambientName = "ProgramData"
	}
	t.Setenv(ambientName, roots[0])
	eng, err := NewEngine(EngineConfig{SystemDefaults: true, ConfigLoaded: true})
	if err != nil {
		t.Fatal(err)
	}

	for i, root := range roots {
		t.Setenv(ambientName, root)
		snapshot, err := eng.Discover(t.Context(), "ambient_defaults_probe")
		if err != nil {
			t.Fatalf("Discover() with %s=%q: %v", ambientName, root, err)
		}
		got, err := snapshot.Value("ambient_defaults_probe")
		if err != nil {
			t.Fatalf("Value(ambient_defaults_probe) with %s=%q: %v", ambientName, root, err)
		}
		if got != wants[i] {
			t.Fatalf("ambient_defaults_probe with %s=%q = %#v, want %q", ambientName, root, got, wants[i])
		}
	}
}
