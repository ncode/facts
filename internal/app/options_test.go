package app

import (
	"flag"
	"io"
	"testing"

	"github.com/ncode/facts/internal/cli"
)

func TestParsedOptionFlagSetMatchesRegistry(t *testing.T) {
	flags, _, err := newParsedOptionFlagSet("test", io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	wantNames := map[string]bool{}
	wantCanonicals := map[string]bool{}
	for _, option := range cli.Options() {
		if option.TaskFlag {
			continue
		}
		wantCanonicals[option.Canonical] = true
		wantNames[flagName(option.Canonical)] = true
		for _, alias := range option.Aliases {
			wantNames[flagName(alias)] = true
		}
	}

	gotNames := map[string]bool{}
	flags.VisitAll(func(f *flag.Flag) {
		gotNames[f.Name] = true
	})
	for name := range wantNames {
		if !gotNames[name] {
			t.Fatalf("parser missing registry option %q", name)
		}
	}
	for name := range gotNames {
		if !wantNames[name] {
			t.Fatalf("parser accepts %q outside registry", name)
		}
	}

	bindings := (&parsedOptions{}).optionBindings()
	for canonical := range wantCanonicals {
		if bindings[canonical] == nil {
			t.Fatalf("registry option %q has no parser binding", canonical)
		}
	}
}

func TestParsedOptionFlagSetKeepsHeadUsageStrings(t *testing.T) {
	flags, _, err := newParsedOptionFlagSet("test", io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"json":                 "render facts as JSON",
		"j":                    "render facts as JSON",
		"no-json":              "do not render facts as JSON",
		"yaml":                 "render facts as YAML",
		"y":                    "render facts as YAML",
		"no-yaml":              "do not render facts as YAML",
		"hocon":                "render facts as HOCON",
		"no-hocon":             "do not render facts as HOCON",
		"debug":                "write debug logs",
		"d":                    "write debug logs",
		"color":                "colorize diagnostic output",
		"no-color":             "disable colorized diagnostic output",
		"log-level":            "accepted for Facter compatibility",
		"l":                    "accepted for Facter compatibility",
		"timing":               "write fact timing",
		"t":                    "write fact timing",
		"strict":               "return an error when queried facts are missing",
		"force-dot-resolution": "merge dotted facts into structured facts",
		"config":               "load configuration from file",
		"c":                    "load configuration from file",
		"no-block":             "disable fact blocking",
		"no-cache":             "disable loading and refreshing facts from the cache",
		"verbose":              "write info logs",
		"sequential":           "accepted for Facter compatibility",
		"http-debug":           "accepted for Facter compatibility",
		"no-external-facts":    "accepted for Facter compatibility",
		"external-dir":         "load external facts from directory",
		"disable":              "disable facts or fact groups (comma-separated, repeatable)",
	}

	for name, usage := range want {
		got := flags.Lookup(name)
		if got == nil {
			t.Fatalf("flags.Lookup(%q) = nil", name)
		}
		if got.Usage != usage {
			t.Fatalf("usage for %q = %q, want %q", name, got.Usage, usage)
		}
	}
}
