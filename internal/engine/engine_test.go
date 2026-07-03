package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewEngineValidatesProgrammaticFactNames(t *testing.T) {
	tests := []struct {
		name    string
		facts   []ProgrammaticFact
		wantErr string
	}{
		{
			name:    "empty name",
			facts:   []ProgrammaticFact{{Name: ""}},
			wantErr: "fact 0: name is empty",
		},
		{
			name:    "null byte",
			facts:   []ProgrammaticFact{{Name: "site\x00role"}},
			wantErr: `fact "site\x00role": name contains a null byte`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEngine(EngineConfig{Facts: tt.facts})
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("NewEngine() err = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestNewEngineFreezesConfigAndNormalizesFactNames(t *testing.T) {
	externalDirs := []string{"/explicit"}
	blocked := map[string]bool{"networking": true}
	defaultDirs := []string{"/default"}
	config := Config{
		Disabled:     []string{"ssh"},
		ExternalDirs: []string{"/config"},
		TTLs:         []FactTTL{{Fact: "site_role", TTL: "30 days"}},
		FactGroups:   []FactGroup{{Name: "site", Facts: []string{"site_role"}}},
	}
	facts := []ProgrammaticFact{{
		Name: "Site.Role",
		Resolve: func(context.Context) (any, error) {
			return "web", nil
		},
	}}

	eng, err := NewEngine(EngineConfig{
		ExternalDirs:           externalDirs,
		DisabledFacts:          blocked,
		ConfigLoaded:           true,
		Config:                 config,
		DefaultExternalDirsSet: true,
		DefaultExternalDirs:    defaultDirs,
		Facts:                  facts,
	})
	if err != nil {
		t.Fatal(err)
	}

	externalDirs[0] = "/mutated-explicit"
	blocked["networking"] = false
	defaultDirs[0] = "/mutated-default"
	config.Disabled[0] = "mutated-blocklist"
	config.ExternalDirs[0] = "/mutated-config"
	config.TTLs[0] = FactTTL{Fact: "mutated", TTL: "1 second"}
	config.FactGroups[0].Facts[0] = "mutated_group_fact"
	facts[0].Name = "Mutated"

	if got, want := eng.cfg.ExternalDirs, []string{"/explicit"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ExternalDirs = %#v, want %#v", got, want)
	}
	if got := eng.cfg.DisabledFacts["networking"]; !got {
		t.Fatalf("DisabledFacts[networking] = false, want frozen true")
	}
	if got, want := eng.cfg.DefaultExternalDirs, []string{"/default"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultExternalDirs = %#v, want %#v", got, want)
	}
	if got, want := eng.cfg.Config.Disabled, []string{"ssh"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.Disabled = %#v, want %#v", got, want)
	}
	if got, want := eng.cfg.Config.ExternalDirs, []string{"/config"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.ExternalDirs = %#v, want %#v", got, want)
	}
	if got, want := eng.cfg.Config.TTLs, []FactTTL{{Fact: "site_role", TTL: "30 days"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.TTLs = %#v, want %#v", got, want)
	}
	if got, want := eng.cfg.Config.FactGroups, []FactGroup{{Name: "site", Facts: []string{"site_role"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Config.FactGroups = %#v, want %#v", got, want)
	}
	if got := eng.cfg.Facts[0].Name; got != "site.role" {
		t.Fatalf("Facts[0].Name = %q, want lowercase site.role", got)
	}
	if got, err := eng.cfg.Facts[0].Resolve(context.Background()); err != nil || got != "web" {
		t.Fatalf("Facts[0].Resolve() = %#v, %v, want web", got, err)
	}
}

func TestEngineWarnOnceDeduplicatesMessages(t *testing.T) {
	var warnings []string
	eng, err := NewEngine(EngineConfig{Logger: captureLogger(nil, &warnings, nil)})
	if err != nil {
		t.Fatal(err)
	}

	eng.warnOnce("same warning")
	eng.warnOnce("same warning")
	eng.warnOnce("different warning")

	want := []string{"same warning", "different warning"}
	if !reflect.DeepEqual(warnings, want) {
		t.Fatalf("warnings = %#v, want %#v", warnings, want)
	}
}

func TestPlanDiscoveryMergesLoadedConfig(t *testing.T) {
	config := Config{
		ExternalDirs:       []string{"/config"},
		NoExternalFacts:    true,
		Disabled:           []string{"site"},
		TTLs:               []FactTTL{{Fact: "site_role", TTL: "1 day"}},
		FactGroups:         []FactGroup{{Name: "site", Facts: []string{"site_role", "site_location"}}},
		ForceDotResolution: true,
	}
	eng, err := NewEngine(EngineConfig{
		ConfigLoaded: true,
		Config:       config,
		UseCache:     true,
		CLICompat:    true,
	})
	if err != nil {
		t.Fatal(err)
	}
	config.ExternalDirs[0] = "/mutated"
	config.FactGroups[0].Facts[0] = "mutated"

	plan, failures := eng.planDiscovery(NewSession(), []string{"site_role"})
	if len(failures) != 0 {
		t.Fatalf("failures = %#v, want none", failures)
	}
	if got, want := plan.externalDirs, []string{"/config"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("externalDirs = %#v, want %#v", got, want)
	}
	if !plan.noExternalFacts {
		t.Fatal("noExternalFacts = false, want config to disable external facts")
	}
	if !plan.useCache {
		t.Fatal("useCache = false, want EngineConfig.UseCache")
	}
	if got, want := plan.cacheTTLs, []FactTTL{{Fact: "site_role", TTL: "1 day"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cacheTTLs = %#v, want %#v", got, want)
	}
	if got, want := plan.cacheGroups, []FactGroup{{Name: "site", Facts: []string{"site_role", "site_location"}}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("cacheGroups = %#v, want %#v", got, want)
	}
	if !plan.disabledFacts["site_role"] || !plan.disabledFacts["site_location"] {
		t.Fatalf("disabledFacts = %#v, want configured group expanded", plan.disabledFacts)
	}
	if plan.loaderMode != externalFactLoaderCLI {
		t.Fatalf("loaderMode = %v, want CLI", plan.loaderMode)
	}
	if !plan.includeEnv {
		t.Fatal("includeEnv = false, want CLICompat environment facts")
	}
	if got, want := plan.queries, []string{"site_role"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("queries = %#v, want %#v", got, want)
	}
	if !plan.includeTypedDotted {
		t.Fatal("includeTypedDotted = false, want force-dot-resolution from CLI config")
	}
}

func TestPlanDiscoveryPreservesExplicitInputsAndUsesDefaultDirs(t *testing.T) {
	eng, err := NewEngine(EngineConfig{
		ExternalDirs:           []string{"/explicit"},
		DisabledFacts:          map[string]bool{"explicit": true},
		ConfigLoaded:           true,
		Config:                 Config{ExternalDirs: []string{"/config"}, Disabled: []string{"config"}},
		SystemDefaults:         true,
		DefaultExternalDirsSet: true,
		DefaultExternalDirs:    []string{"/default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, failures := eng.planDiscovery(NewSession(), nil)
	if len(failures) != 0 {
		t.Fatalf("failures = %#v, want none", failures)
	}
	if got, want := plan.externalDirs, []string{"/explicit"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("externalDirs = %#v, want explicit dirs %#v", got, want)
	}
	if got, want := plan.disabledFacts, map[string]bool{"explicit": true}; !reflect.DeepEqual(got, want) {
		t.Fatalf("disabledFacts = %#v, want explicit disabled %#v", got, want)
	}

	defaultEng, err := NewEngine(EngineConfig{
		SystemDefaults:         true,
		DefaultExternalDirsSet: true,
		DefaultExternalDirs:    []string{"/default"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defaultPlan, failures := defaultEng.planDiscovery(NewSession(), nil)
	if len(failures) != 0 {
		t.Fatalf("default failures = %#v, want none", failures)
	}
	if got, want := defaultPlan.externalDirs, []string{"/default"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("default externalDirs = %#v, want %#v", got, want)
	}
	defaultPlan.externalDirs[0] = "/mutated-plan"
	nextPlan, _ := defaultEng.planDiscovery(NewSession(), nil)
	if got, want := nextPlan.externalDirs, []string{"/default"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("next default externalDirs = %#v, want clone %#v", got, want)
	}
}

func TestDefaultExternalDirsWithoutOverrideUsesPlatformDefaults(t *testing.T) {
	eng, err := NewEngine(EngineConfig{})
	if err != nil {
		t.Fatal(err)
	}

	if got, want := eng.defaultExternalDirs(), CurrentDefaultExternalFactDirs(); !reflect.DeepEqual(got, want) {
		t.Fatalf("defaultExternalDirs() = %#v, want platform defaults %#v", got, want)
	}
}

func TestEngineDiscoverResolvesRegisteredAndExternalFacts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "site.txt"), []byte("external_site=lab\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resolverErr := errors.New("boom")
	var warnings []string
	eng, err := NewEngine(EngineConfig{
		Logger:       captureLogger(nil, &warnings, nil),
		ExternalDirs: []string{dir},
		Facts: []ProgrammaticFact{
			{
				Name: "site_map",
				Resolve: func(context.Context) (any, error) {
					return map[string]string{"role": "web"}, nil
				},
			},
			{Name: "blank"},
			{
				Name: "bad_value",
				Resolve: func(context.Context) (any, error) {
					return "bad\x00value", nil
				},
			},
			{
				Name: "failing",
				Resolve: func(context.Context) (any, error) {
					return nil, resolverErr
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(nil, "site_map.role", "external_site", "blank", "bad_value", "failing")
	if !errors.Is(err, resolverErr) || err.Error() != "fact failing: boom" {
		t.Fatalf("Discover() err = %v, want only resolver error", err)
	}
	if snap == nil {
		t.Fatal("Discover() snapshot = nil, want partial snapshot")
	}
	if got, err := snap.Value("site_map.role"); err != nil || got != "web" {
		t.Fatalf("Value(site_map.role) = %#v, %v, want web", got, err)
	}
	if got, err := snap.Value("external_site"); err != nil || got != "lab" {
		t.Fatalf("Value(external_site) = %#v, %v, want lab", got, err)
	}
	if got, err := snap.Value("blank"); err != nil || got != nil {
		t.Fatalf("Value(blank) = %#v, %v, want resolved nil", got, err)
	}
	if got, err := snap.Value("bad_value"); err != nil || got != nil {
		t.Fatalf("Value(bad_value) = %#v, %v, want rejected nil value", got, err)
	}
	if got, err := snap.Value("failing"); !errors.Is(err, ErrFactNotFound) || got != nil {
		t.Fatalf("Value(failing) = %#v, %v, want fact not found", got, err)
	}
	if want := []string{"custom fact value contains a null byte reference"}; !reflect.DeepEqual(warnings, want) {
		t.Fatalf("warnings = %#v, want %#v", warnings, want)
	}
}

func TestEngineDiscoverReturnsPartialSnapshotWhenContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ran := false
	eng, err := NewEngine(EngineConfig{
		Facts: []ProgrammaticFact{{
			Name: "should_not_run",
			Resolve: func(context.Context) (any, error) {
				ran = true
				return "ran", nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Discover() err = %v, want context.Canceled", err)
	}
	if snap == nil {
		t.Fatal("Discover() snapshot = nil, want partial snapshot")
	}
	if ran {
		t.Fatal("registered resolver ran after context cancellation")
	}
}

func TestEngineDiscoverUsesCachedValueForConfiguredFacts(t *testing.T) {
	cacheDir := t.TempDir()
	oldDefaultCachePath := DefaultCachePath
	DefaultCachePath = func() string { return cacheDir }
	t.Cleanup(func() { DefaultCachePath = oldDefaultCachePath })

	eng, err := NewEngine(EngineConfig{
		UseCache:     true,
		ConfigLoaded: true,
		Config:       Config{TTLs: []FactTTL{{Fact: "cache_probe", TTL: "30 days"}}},
		Facts: []ProgrammaticFact{{
			Name: "cache_probe",
			Resolve: func(context.Context) (any, error) {
				return "cached", nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() err = %v, want nil", err)
	}
	if got, err := snap.Value("cache_probe"); err != nil || got != "cached" {
		t.Fatalf("Value(cache_probe) = %#v, %v, want cached", got, err)
	}
	data := readJSONFile(t, filepath.Join(cacheDir, "cache_probe"))
	if got := data["cache_probe"]; got != "cached" {
		t.Fatalf("cached cache_probe = %#v, want cached", got)
	}

	// The current cache contract is value precedence: a fresh cache entry wins.
	cachedEng, err := NewEngine(EngineConfig{
		UseCache:     true,
		ConfigLoaded: true,
		Config:       Config{TTLs: []FactTTL{{Fact: "cache_probe", TTL: "30 days"}}},
		Facts: []ProgrammaticFact{{
			Name: "cache_probe",
			Resolve: func(context.Context) (any, error) {
				return "fresh", nil
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	cachedSnap, err := cachedEng.Discover(context.Background())
	if err != nil {
		t.Fatalf("cached Discover() err = %v, want nil", err)
	}
	if got, err := cachedSnap.Value("cache_probe"); err != nil || got != "cached" {
		t.Fatalf("cached Value(cache_probe) = %#v, %v, want cache file value", got, err)
	}
}

// externalFactErrorFixtureDir writes a good static fact file alongside a
// null-byte fact file. The loader loads the good file, then errors on the null
// byte — the fixture that distinguishes the CLI (fail-fast) and library
// (accumulate) error policies these pins lock before the Discover arms collapse.
func externalFactErrorFixtureDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.txt"), []byte("good_fact=ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.txt"), []byte("bad_fact=va\x00lue\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Library mode: a per-file loader failure is accumulated into the joined error
// while the successfully loaded facts are retained in a partial snapshot.
func TestEngineDiscoverLibraryModeAccumulatesLoaderErrorAndKeepsFacts(t *testing.T) {
	dir := externalFactErrorFixtureDir(t)
	eng, err := NewEngine(EngineConfig{ExternalDirs: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(context.Background(), "good_fact")
	if !errors.Is(err, ErrNullByte) {
		t.Fatalf("Discover() err = %v, want joined null-byte error", err)
	}
	if snap == nil {
		t.Fatal("Discover() snapshot = nil, want partial snapshot retaining loaded facts")
	}
	if got, err := snap.Value("good_fact"); err != nil || got != "ok" {
		t.Fatalf("Value(good_fact) = %#v, %v, want ok (library mode retains loaded facts)", got, err)
	}
}

// CLI mode: the first loader failure aborts discovery, discarding the facts
// loaded before it and returning the bare loader error (no core facts resolve).
func TestEngineDiscoverCLIModeFailsFastAndDiscardsFacts(t *testing.T) {
	dir := externalFactErrorFixtureDir(t)
	eng, err := NewEngine(EngineConfig{ExternalDirs: []string{dir}, CLICompat: true})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(context.Background(), "good_fact")
	if !errors.Is(err, ErrNullByte) {
		t.Fatalf("Discover() err = %v, want bare null-byte loader error", err)
	}
	if snap == nil {
		t.Fatal("Discover() snapshot = nil, want empty snapshot")
	}
	if _, err := snap.Value("good_fact"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("Value(good_fact) err = %v, want ErrFactNotFound (CLI fail-fast discards loaded facts)", err)
	}
}
