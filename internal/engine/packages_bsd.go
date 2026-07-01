package engine

import (
	"os"
	"path/filepath"
	"strings"
)

// pkgngPackages queries the FreeBSD/DragonFly pkgng database with a single cheap
// `pkg query`, capturing name, version, and the package ABI (%q), which encodes
// the target architecture (e.g. FreeBSD:14:amd64, dragonfly:6.4:x86:64, or a
// FreeBSD:14:* wildcard for architecture-independent packages).
func pkgngPackages(run commandRunner) []any {
	// pkgng lives in the ports prefix, which is outside the engine's trusted
	// command PATH (/usr/sbin:/usr/bin:/sbin:/bin); call it by absolute path,
	// the same way the DMI probe reaches /usr/local/sbin/dmidecode. DragonFly
	// has no /usr/sbin/pkg bootstrap stub, so only the ports path is portable.
	out := run("/usr/local/sbin/pkg", "query", "-a", "%n|%v|%q")
	var records []any
	for line := range strings.Lines(out) {
		fields := strings.SplitN(strings.TrimSpace(line), "|", 3)
		if len(fields) != 3 {
			continue
		}
		name, version, abi := fields[0], fields[1], fields[2]
		if name == "" || version == "" {
			continue
		}
		records = append(records, packageRecord(name, version, "architecture", abi))
	}
	sortPackages(records)
	return records
}

// openbsdPackages enumerates the /var/db/pkg/<name>-<version>[-flavor]
// subdirectories that record every installed OpenBSD package, deriving the stem
// and version from the directory name and the architecture from each package's
// +CONTENTS @arch annotation.
func openbsdPackages(readDir func(string) ([]os.DirEntry, error), readFile fileReader) []any {
	entries, err := readDir("/var/db/pkg")
	if err != nil {
		return nil
	}
	var records []any
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name, version := splitBSDPackageName(entry.Name())
		if name == "" || version == "" {
			continue
		}
		arch := bsdContentsArch(readFile, filepath.Join("/var/db/pkg", entry.Name(), "+CONTENTS"))
		records = append(records, packageRecord(name, version, "architecture", arch))
	}
	sortPackages(records)
	return records
}

// pkgsrcPackages enumerates the installed NetBSD pkgsrc packages recorded as
// <name>-<version> subdirectories of PKG_DBDIR. PKG_DBDIR is not hardcoded to a
// single path: the standard candidates are probed in order (the pkgsrc default
// /usr/pkg/pkgdb, then the legacy /var/db/pkg), using the first that lists any
// package entries.
func pkgsrcPackages(readDir func(string) ([]os.DirEntry, error)) []any {
	for _, dbdir := range []string{"/usr/pkg/pkgdb", "/var/db/pkg"} {
		entries, err := readDir(dbdir)
		if err != nil {
			continue
		}
		var records []any
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name, version := splitBSDPackageName(entry.Name())
			if name == "" || version == "" {
				continue
			}
			records = append(records, packageRecord(name, version))
		}
		if len(records) > 0 {
			sortPackages(records)
			return records
		}
	}
	return nil
}

// ipsPackages lists the packages installed in the local illumos IPS image with
// `pkg list -H` (no header row, no network refresh). Columns are
// NAME [ (PUBLISHER) ] VERSION IFO, where the parenthesised publisher only
// appears for packages from a non-preferred publisher. The FMRI stem is the
// name, the branch-bearing version column is the version, and the trailing IFO
// flags are ignored.
func ipsPackages(run commandRunner) []any {
	out := run("pkg", "list", "-H")
	var records []any
	for line := range strings.Lines(out) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name, rest := fields[0], fields[1:]
		if strings.HasPrefix(rest[0], "(") {
			rest = rest[1:]
		}
		if len(rest) == 0 {
			continue
		}
		version := rest[0]
		if name == "" || version == "" {
			continue
		}
		records = append(records, packageRecord(name, version))
	}
	sortPackages(records)
	return records
}

// splitBSDPackageName splits an OpenBSD/pkgsrc package directory name into its
// stem and version. Both encode the version as the last hyphen-separated
// component (following a non-empty stem) whose token begins with a digit; any
// trailing hyphen-separated tokens are flavors and are discarded. Choosing the
// last such component keeps names whose final stem token is numeric intact
// (e.g. gcc-11-11.2.0 -> gcc-11 / 11.2.0), while still handling flavored names
// (vim-9.0.2035-no_x11 -> vim / 9.0.2035). Returns empty strings when no version
// component is present (e.g. the pkgdb.byfile.db index file).
func splitBSDPackageName(dir string) (name, version string) {
	parts := strings.Split(dir, "-")
	versionIdx := -1
	for i := 1; i < len(parts); i++ {
		if token := parts[i]; token != "" && token[0] >= '0' && token[0] <= '9' {
			versionIdx = i
		}
	}
	if versionIdx < 0 {
		return "", ""
	}
	return strings.Join(parts[:versionIdx], "-"), parts[versionIdx]
}

// bsdContentsArch returns the architecture recorded in an OpenBSD +CONTENTS
// packing list (the @arch annotation). The arch-independent "*" marker and a
// missing file both yield an empty string.
func bsdContentsArch(readFile fileReader, path string) string {
	data, err := readFile(path)
	if err != nil {
		return ""
	}
	for line := range strings.Lines(string(data)) {
		if rest, ok := strings.CutPrefix(strings.TrimRight(line, "\n"), "@arch "); ok {
			if arch := strings.TrimSpace(rest); arch != "*" {
				return arch
			}
			return ""
		}
	}
	return ""
}
