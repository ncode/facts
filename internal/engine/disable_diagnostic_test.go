package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func warnContains(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

func TestDiscover_diagnosesConfigDisabledExplicitQuery(t *testing.T) {
	var warnings []string
	eng, err := NewEngine(EngineConfig{
		ConfigLoaded:    true,
		Config:          Config{Disabled: []string{"site_role"}},
		NoExternalFacts: true,
		Logger:          captureLogger(nil, &warnings, nil),
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(context.Background(), "site_role")
	if err != nil {
		t.Fatal(err)
	}
	if !warnContains(warnings, `fact "site_role" is disabled by `+disabledSourceConfig) {
		t.Fatalf("warnings = %#v, want config-disable diagnostic for site_role", warnings)
	}
	// The diagnostic and the suppression are one contract: the queried fact must
	// also be absent from the snapshot, not merely warned about.
	if _, err := snap.Value("site_role"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("snapshot.Value(site_role) err = %v, want ErrFactNotFound (disabled fact suppressed)", err)
	}
}

func TestDiscover_diagnosesEnvDisabledExplicitQuery(t *testing.T) {
	t.Setenv("FACTS_DISABLE", "networking")
	var warnings []string
	eng, err := NewEngine(EngineConfig{
		CLICompat:       true,
		NoExternalFacts: true,
		Logger:          captureLogger(nil, &warnings, nil),
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(context.Background(), "networking")
	if err != nil {
		t.Fatal(err)
	}
	if !warnContains(warnings, `fact "networking" is disabled by FACTS_DISABLE`) {
		t.Fatalf("warnings = %#v, want FACTS_DISABLE diagnostic for networking", warnings)
	}
	if _, err := snap.Value("networking"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("snapshot.Value(networking) err = %v, want ErrFactNotFound (disabled fact suppressed)", err)
	}
}

func TestDiscover_doesNotDiagnoseCLIDisabledExplicitQuery(t *testing.T) {
	var warnings []string
	eng, err := NewEngine(EngineConfig{
		CLICompat:       true,
		NoExternalFacts: true,
		ExtraDisabled:   []string{"networking"},
		Logger:          captureLogger(nil, &warnings, nil),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := eng.Discover(context.Background(), "networking"); err != nil {
		t.Fatal(err)
	}
	if warnContains(warnings, "is disabled by") {
		t.Fatalf("warnings = %#v, want no diagnostic for a --disable on the same command line", warnings)
	}
}

// A --disable on the command line subsumes an ambient (config/env) disable of a
// descendant, so the ambient diagnostic must stay quiet: the operator already
// asked for the broader disable on this very command line.
func TestDiscover_cliDisableSilencesAmbientDescendantDiagnostic(t *testing.T) {
	var warnings []string
	eng, err := NewEngine(EngineConfig{
		CLICompat:       true,
		ConfigLoaded:    true,
		Config:          Config{Disabled: []string{"networking.ip"}},
		ExtraDisabled:   []string{"networking"},
		NoExternalFacts: true,
		Logger:          captureLogger(nil, &warnings, nil),
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := eng.Discover(context.Background(), "networking.ip"); err != nil {
		t.Fatal(err)
	}
	if warnContains(warnings, "is disabled by") {
		t.Fatalf("warnings = %#v, want no diagnostic: --disable networking subsumes the ambient networking.ip", warnings)
	}
}

// An ambient disable of an ancestor must diagnose (and suppress) a descendant
// query: config disable of os.release fires for a query of os.release.major,
// matching FilterDisabledFacts' descendant pruning.
func TestDiscover_diagnosesAmbientAncestorDisableForDescendantQuery(t *testing.T) {
	var warnings []string
	eng, err := NewEngine(EngineConfig{
		ConfigLoaded:    true,
		Config:          Config{Disabled: []string{"os.release"}},
		NoExternalFacts: true,
		Logger:          captureLogger(nil, &warnings, nil),
	})
	if err != nil {
		t.Fatal(err)
	}

	snap, err := eng.Discover(context.Background(), "os.release.major")
	if err != nil {
		t.Fatal(err)
	}
	if !warnContains(warnings, `fact "os.release.major" is disabled by `+disabledSourceConfig) {
		t.Fatalf("warnings = %#v, want config-disable diagnostic for os.release.major via os.release ancestor", warnings)
	}
	if _, err := snap.Value("os.release.major"); !errors.Is(err, ErrFactNotFound) {
		t.Fatalf("snapshot.Value(os.release.major) err = %v, want ErrFactNotFound (ancestor disable suppresses descendant)", err)
	}
}
