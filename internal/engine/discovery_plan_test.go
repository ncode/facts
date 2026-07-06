package engine

import (
	"reflect"
	"testing"
)

// sessionWithEnviron returns a Session whose host.environ() reports env, used to
// drive the FACTS_DISABLE branch of planDiscovery without touching the process
// environment.
func sessionWithEnviron(env []string) *Session {
	s := NewSession()
	s.host = &fakeHostOS{environEntries: env}
	return s
}

func TestPlanDiscoveryUnionsConfigEnvAndCLISources(t *testing.T) {
	eng, err := NewEngine(EngineConfig{
		ConfigLoaded:   true,
		Config:         Config{Disabled: []string{"config_fact"}},
		ExtraDisabled:  []string{"cli_fact"},
		SystemDefaults: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, failures := eng.planDiscovery(sessionWithEnviron([]string{"FACTS_DISABLE=env_fact"}), nil)
	if len(failures) != 0 {
		t.Fatalf("failures = %#v, want none", failures)
	}
	for _, name := range []string{"config_fact", "env_fact", "cli_fact"} {
		if !plan.disabledFacts[name] {
			t.Fatalf("disabledFacts = %#v, want union to include %q", plan.disabledFacts, name)
		}
	}
}

func TestPlanDiscoveryExpandsGroupsAcrossEverySource(t *testing.T) {
	// Each source disables a DISTINCT group with disjoint members, so a single
	// working source cannot mask a broken one: every group's members must appear
	// only if that source's own expansion ran.
	groups := []FactGroup{
		{Name: "config_group", Facts: []string{"config_alpha", "config_beta"}},
		{Name: "env_group", Facts: []string{"env_alpha", "env_beta"}},
		{Name: "cli_group", Facts: []string{"cli_alpha", "cli_beta"}},
	}
	eng, err := NewEngine(EngineConfig{
		ConfigLoaded:   true,
		Config:         Config{Disabled: []string{"config_group"}, FactGroups: groups},
		ExtraDisabled:  []string{"cli_group"},
		SystemDefaults: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, _ := eng.planDiscovery(sessionWithEnviron([]string{"FACTS_DISABLE=env_group"}), nil)
	bySource := map[string][]string{
		"config (disable)":          {"config_alpha", "config_beta"},
		"FACTS_DISABLE":             {"env_alpha", "env_beta"},
		"ExtraDisabled (--disable)": {"cli_alpha", "cli_beta"},
	}
	for source, members := range bySource {
		for _, name := range members {
			if !plan.disabledFacts[name] {
				t.Fatalf("disabledFacts = %#v, want %s group member %q expanded", plan.disabledFacts, source, name)
			}
		}
	}
}

func TestPlanDiscoverySuppressesEnvDisableWhenIncludeEnvFalse(t *testing.T) {
	eng, err := NewEngine(EngineConfig{
		ConfigLoaded:  true,
		Config:        Config{Disabled: []string{"config_fact"}},
		ExtraDisabled: []string{"cli_fact"},
		// No SystemDefaults and no CLICompat: includeEnv is false, so the
		// ambient FACTS_DISABLE source must be ignored, just like env facts.
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, _ := eng.planDiscovery(sessionWithEnviron([]string{"FACTS_DISABLE=env_fact"}), nil)
	if plan.includeEnv {
		t.Fatal("includeEnv = true, want false for a hermetic engine")
	}
	if plan.disabledFacts["env_fact"] {
		t.Fatalf("disabledFacts = %#v, want env_fact suppressed when includeEnv is false", plan.disabledFacts)
	}
	for _, name := range []string{"config_fact", "cli_fact"} {
		if !plan.disabledFacts[name] {
			t.Fatalf("disabledFacts = %#v, want %q even with env suppressed", plan.disabledFacts, name)
		}
	}
}

func TestPlanDiscoveryDisabledFactsOverrideWinsOverUnion(t *testing.T) {
	eng, err := NewEngine(EngineConfig{
		DisabledFacts:  map[string]bool{"override": true},
		ConfigLoaded:   true,
		Config:         Config{Disabled: []string{"config_fact"}},
		ExtraDisabled:  []string{"cli_fact"},
		SystemDefaults: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	plan, _ := eng.planDiscovery(sessionWithEnviron([]string{"FACTS_DISABLE=env_fact"}), nil)
	want := map[string]bool{"override": true}
	if !reflect.DeepEqual(plan.disabledFacts, want) {
		t.Fatalf("disabledFacts = %#v, want override verbatim %#v (--no-block / explicit set wins)", plan.disabledFacts, want)
	}
}

func TestDiscoveryExternalDirsResolutionOrder(t *testing.T) {
	tests := []struct {
		name           string
		config         Config
		explicit       []string
		noExternal     bool
		systemDefaults bool
		defaults       []string
		want           []string
	}{
		{
			name:           "explicit wins",
			config:         Config{ExternalDirs: []string{"/config"}},
			explicit:       []string{"/cli"},
			systemDefaults: true,
			defaults:       []string{"/default"},
			want:           []string{"/cli"},
		},
		{
			name:           "config fallback",
			config:         Config{ExternalDirs: []string{"/config"}},
			systemDefaults: true,
			defaults:       []string{"/default"},
			want:           []string{"/config"},
		},
		{
			name:           "system defaults fallback",
			systemDefaults: true,
			defaults:       []string{"/default"},
			want:           []string{"/default"},
		},
		{
			name:       "explicit no external",
			explicit:   []string{"/cli"},
			noExternal: true,
		},
		{
			name:     "config no external",
			config:   Config{ExternalDirs: []string{"/config"}, NoExternalFacts: true},
			explicit: []string{"/cli"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DiscoveryExternalDirs(tt.config, tt.explicit, tt.noExternal, tt.systemDefaults, tt.defaults)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("DiscoveryExternalDirs() = %#v, want %#v", got, tt.want)
			}
			if len(got) > 0 {
				got[0] = "mutated"
				again := DiscoveryExternalDirs(tt.config, tt.explicit, tt.noExternal, tt.systemDefaults, tt.defaults)
				if reflect.DeepEqual(got, again) {
					t.Fatalf("DiscoveryExternalDirs() returned aliased slice %#v", again)
				}
			}
		})
	}
}
