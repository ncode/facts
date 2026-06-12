package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func overridePuppetCacheDir(t *testing.T, dir string) {
	t.Helper()
	previous := puppetCacheDirFn
	puppetCacheDirFn = func() string { return dir }
	t.Cleanup(func() { puppetCacheDirFn = previous })
}

func TestPuppetPluginFactDirs_returnsExistingPluginFactDest(t *testing.T) {
	cache := t.TempDir()
	factsD := filepath.Join(cache, "facts.d")
	if err := os.MkdirAll(factsD, 0o755); err != nil {
		t.Fatal(err)
	}
	overridePuppetCacheDir(t, cache)

	got := PuppetPluginFactDirs()
	if len(got) != 1 || got[0] != factsD {
		t.Fatalf("PuppetPluginFactDirs() = %v, want [%s]", got, factsD)
	}
}

func TestPuppetPluginFactDirs_skipsMissingPluginFactDest(t *testing.T) {
	overridePuppetCacheDir(t, t.TempDir())

	if got := PuppetPluginFactDirs(); got != nil {
		t.Fatalf("PuppetPluginFactDirs() = %v, want nil for missing facts.d", got)
	}
}

func TestWarnPuppetRubyPluginFacts_warnsOnlyWhenRubyFactsPresent(t *testing.T) {
	cache := t.TempDir()
	libFacter := filepath.Join(cache, "lib", "facter")
	if err := os.MkdirAll(libFacter, 0o755); err != nil {
		t.Fatal(err)
	}
	overridePuppetCacheDir(t, cache)

	var warnings []string
	SetWarningHandler(func(message string) { warnings = append(warnings, message) })
	t.Cleanup(func() { SetWarningHandler(nil) })

	WarnPuppetRubyPluginFacts()
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none for empty lib/facter", warnings)
	}

	if err := os.WriteFile(filepath.Join(libFacter, "synced.rb"), []byte("Facter.add(:x)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	WarnPuppetRubyPluginFacts()
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly one", warnings)
	}
	for _, fragment := range []string{libFacter, "not loaded by the Go port"} {
		if !strings.Contains(warnings[0], fragment) {
			t.Fatalf("warning %q missing %q", warnings[0], fragment)
		}
	}
}

func TestPuppetPluginExternalFactsLoadThroughExternalLoader(t *testing.T) {
	cache := t.TempDir()
	factsD := filepath.Join(cache, "facts.d")
	if err := os.MkdirAll(factsD, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(factsD, "synced.txt"), []byte("pluginfact=from-puppet\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	overridePuppetCacheDir(t, cache)

	facts, err := LoadExternalFacts(testSession, PuppetPluginFactDirs())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, fact := range facts {
		if fact.Name == "pluginfact" && fact.Value == "from-puppet" {
			found = true
		}
	}
	if !found {
		t.Fatalf("facts = %#v, want pluginfact from Puppet facts.d", facts)
	}
}
