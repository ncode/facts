package engine

import (
	"os"
	"strings"
)

// This file adds the "extra" Linux package sources (snap, flatpak, nix) to the
// packages fact (ADR-0014). Each reader is a pure function over injected probe
// output and follows the packages.go conventions: {name, version, ...} records
// via packageRecord, deterministic ordering via sortPackages, and nil when the
// source is absent or empty. Sources are never merged.
//
// Wiring (added to packagesCoreFacts' `case "linux":`, gated so a tool is only
// spawned when its state directory exists):
//
//	if snapdPresent(s.stat) {
//		add("snap", snapPackages(s.commandOutput))
//	}
//	if flatpakPresent(s.stat) {
//		add("flatpak", flatpakPackages(s.commandOutput))
//	}
//	if nixProfilePresent(s.stat) {
//		add("nix", nixPackages(s.commandOutput))
//	}

// snapPackages parses `snap list` (columns: Name Version Rev Tracking Publisher
// Notes). The header row is required: its absence means snap printed "No snaps
// are installed yet." and there is nothing to record.
func snapPackages(run commandRunner) []any {
	out := run("snap", "list")
	var records []any
	sawHeader := false
	for line := range strings.Lines(out) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if !sawHeader {
			if fields[0] == "Name" && fields[1] == "Version" {
				sawHeader = true
			}
			continue
		}
		records = append(records, packageRecord(fields[0], fields[1]))
	}
	sortPackages(records)
	return records
}

// flatpakPackages parses `flatpak list --columns=application,version,arch,branch`
// (the system installation), whose rows are tab-separated with no header. The
// application id is the record name; arch and branch are identity fields —
// branch is load-bearing, not decorative: the same application id can be
// installed twice with an identical version and arch, distinguishable only by
// branch (observed live: GL.default 25.08 vs 25.08-extra). Extensions with no
// version (e.g. codecs-extra) are dropped by the name+version invariant.
func flatpakPackages(run commandRunner) []any {
	out := run("flatpak", "list", "--columns=application,version,arch,branch")
	var records []any
	for line := range strings.Lines(out) {
		fields := strings.Split(strings.TrimRight(line, "\n"), "\t")
		if len(fields) < 2 {
			continue
		}
		name, version := strings.TrimSpace(fields[0]), strings.TrimSpace(fields[1])
		if name == "" || version == "" {
			continue
		}
		var arch, branch string
		if len(fields) >= 3 {
			arch = strings.TrimSpace(fields[2])
		}
		if len(fields) >= 4 {
			branch = strings.TrimSpace(fields[3])
		}
		records = append(records, packageRecord(name, version, "architecture", arch, "branch", branch))
	}
	sortPackages(records)
	return records
}

// nixPackages enumerates the packages that make up the running NixOS system
// environment via `nix-store -q --references /run/current-system/sw` (the direct
// references of the system-path buildEnv). This is the installed profile set, not
// the whole /nix/store. nix-env -q cannot be used here: the NixOS system-path is
// a buildEnv without the manifest.nix nix-env requires, so it lists nothing.
//
// Each reference is a store path "/nix/store/<hash>-<name>-<version>[-<output>]";
// the hash is dropped and the tail split by parseNixNameVersion. Split-output
// derivations (name-version-doc, -man, -bin, ...) collapse to one record via
// dedup on name+version; unversioned environment members are skipped.
func nixPackages(run commandRunner) []any {
	// nix-store lives in the NixOS system profile, outside the engine's trusted
	// command PATH (/usr/sbin:/usr/bin:/sbin:/bin); call it by absolute path.
	out := run("/run/current-system/sw/bin/nix-store", "-q", "--references", "/run/current-system/sw")
	var records []any
	seen := map[string]bool{}
	for line := range strings.Lines(out) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		base := line[strings.LastIndexByte(line, '/')+1:]
		_, tail, ok := strings.Cut(base, "-") // drop the store hash
		if !ok {
			continue
		}
		name, version := parseNixNameVersion(tail)
		if name == "" || version == "" {
			continue
		}
		key := name + "\x00" + version
		if seen[key] {
			continue
		}
		seen[key] = true
		records = append(records, packageRecord(name, version))
	}
	sortPackages(records)
	return records
}

// nixOutputs are the standard nix output component names appended to a store-path
// tail (e.g. glibc-2.42-67-bin). They are trimmed so split outputs of one
// derivation collapse onto its base version.
var nixOutputs = map[string]bool{
	"out": true, "bin": true, "dev": true, "lib": true, "doc": true,
	"man": true, "info": true, "devdoc": true, "devman": true,
	"static": true, "dist": true, "debug": true, "terminfo": true,
}

// parseNixNameVersion splits a store-path tail "<name>-<version>[-<output>]" using
// Nix's own boundary rule: the name ends at the first hyphen-delimited component
// that begins with a digit, and the version is the rest (so multi-component
// versions like glibc's "2.42-67" stay intact). A trailing standard output name
// is dropped. Environment members with no version (e.g. "nixos-help") yield "".
func parseNixNameVersion(tail string) (name, version string) {
	parts := strings.Split(tail, "-")
	boundary := -1
	// Start at 1: the name is always at least the first hyphen component, so a
	// package whose name starts with a digit (7zip, 0ad, 389-ds-base) is not
	// mistaken for a bare version and dropped.
	for i := 1; i < len(parts); i++ {
		if p := parts[i]; p != "" && p[0] >= '0' && p[0] <= '9' {
			boundary = i
			break
		}
	}
	if boundary < 1 {
		return "", ""
	}
	ver := parts[boundary:]
	if len(ver) > 1 && nixOutputs[ver[len(ver)-1]] {
		ver = ver[:len(ver)-1]
	}
	return strings.Join(parts[:boundary], "-"), strings.Join(ver, "-")
}

// snapdPresent, flatpakPresent and nixProfilePresent gate each spawn on a cheap
// stat of the source's state directory, mirroring rpmDatabasePresent, so tools
// are never run on hosts that lack them.
func snapdPresent(stat func(string) (os.FileInfo, error)) bool {
	return dirPresent(stat, "/var/lib/snapd")
}

func flatpakPresent(stat func(string) (os.FileInfo, error)) bool {
	return dirPresent(stat, "/var/lib/flatpak")
}

func nixProfilePresent(stat func(string) (os.FileInfo, error)) bool {
	return dirPresent(stat, "/nix/var/nix/profiles/system")
}

func dirPresent(stat func(string) (os.FileInfo, error), path string) bool {
	info, err := stat(path)
	return err == nil && info.IsDir()
}
