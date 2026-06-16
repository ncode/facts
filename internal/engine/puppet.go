package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	puppetGOOS       = runtime.GOOS
	puppetGeteuid    = os.Geteuid
	puppetCacheDirFn = defaultPuppetCacheDir
)

// defaultPuppetCacheDir returns Puppet's default vardir (cachedir) for the
// current platform and user, mirroring Puppet's own defaults: the system
// cache for root runs and the per-user cache otherwise.
func defaultPuppetCacheDir() string {
	if puppetGOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return filepath.Join(programData, "PuppetLabs", "puppet", "cache")
	}
	if puppetGeteuid() == 0 {
		return "/opt/puppetlabs/puppet/cache"
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/opt/puppetlabs/puppet/cache"
	}
	return filepath.Join(home, ".puppetlabs", "opt", "puppet", "cache")
}

// PuppetPluginFactDirs returns Puppet's default plugin-fact destination
// (pluginfactdest, vardir/facts.d) paths that exist on this system. Under
// --puppet these directories are searched for external facts, matching the
// external-fact half of Ruby Facter's Puppet plugin loading.
func PuppetPluginFactDirs() []string {
	dir := filepath.Join(puppetCacheDirFn(), "facts.d")
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}
	return []string{dir}
}

// WarnPuppetRubyPluginFacts emits the documented --puppet deviation warning
// when Puppet has synced Ruby plugin custom facts (vardir/lib/facter) that
// Ruby Facter would have loaded; the Go port does not evaluate Ruby and
// skips them.
func WarnPuppetRubyPluginFacts() {
	dir := filepath.Join(puppetCacheDirFn(), "lib", "facter")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".rb" {
			continue
		}
		warn("Puppet Ruby plugin custom facts in " + dir + " are not loaded by the Go port; rewrite them as external facts (see docs/CUSTOM_FACT_MIGRATION.md)")
		return
	}
}
