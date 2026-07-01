package engine

import (
	"os"
	"sort"
	"strings"
)

// packagesCoreFacts resolves the packages fact: for each package database
// present on the host, a source-namespaced array of {name, version, ...} records
// (ADR-0014). Sources are never merged, and a source is omitted when its
// database is absent or lists no installed packages. Registered as a
// single-output resolver so a disable of "packages" gates the whole probe
// (ADR-0015).
func packagesCoreFacts(s *Session) []ResolvedFact {
	sources := map[string]any{}
	add := func(name string, records []any) {
		if len(records) > 0 {
			sources[name] = records
		}
	}

	switch s.goos() {
	case "linux":
		add("dpkg", dpkgPackages(s.readFile))
		if rpmDatabasePresent(s.stat) {
			add("rpm", rpmPackages(s.commandOutput))
		}
		add("pacman", pacmanPackages(s.glob, s.readFile))
		add("apk", apkPackages(s.readFile))
		if snapdPresent(s.stat) {
			add("snap", snapPackages(s.commandOutput))
		}
		if flatpakPresent(s.stat) {
			add("flatpak", flatpakPackages(s.commandOutput))
		}
		if nixProfilePresent(s.stat) {
			add("nix", nixPackages(s.commandOutput))
		}
	case "freebsd", "dragonfly":
		add("pkg", pkgngPackages(s.commandOutput))
	case "openbsd":
		add("openbsd_pkg", openbsdPackages(s.readDir, s.readFile))
	case "netbsd":
		add("pkgsrc", pkgsrcPackages(s.readDir))
	case "illumos":
		add("ips", ipsPackages(s.commandOutput))
	case "darwin":
		add("receipts", receiptsPackages(s.glob, s.commandOutput))
		add("apps", appsPackages(s.glob, s.commandOutput))
		add("homebrew", homebrewPackages(s.glob))
	case "windows":
		add("registry", registryPackages(s.commandOutput))
		add("appx", appxPackages(s.commandOutput))
	}

	if len(sources) == 0 {
		return nil
	}
	return []ResolvedFact{{Name: "packages", Value: sources}}
}

// packageRecord builds a record with the always-present name/version plus any
// non-empty identity fields supplied as key/value pairs.
func packageRecord(name, version string, kv ...string) map[string]any {
	record := map[string]any{"name": name, "version": version}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			record[kv[i]] = kv[i+1]
		}
	}
	return record
}

// sortPackages orders records deterministically by name, then architecture, then
// version, so multiarch/multiversion siblings keep a stable order across runs.
func sortPackages(records []any) {
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i].(map[string]any), records[j].(map[string]any)
		// name/architecture/version first, then the remaining identity fields,
		// so siblings that share a name+arch+version still order deterministically
		// regardless of the reader's append order.
		for _, key := range []string{"name", "architecture", "version", "type", "branch", "product_code", "bundle_id", "store_path", "path"} {
			if av, bv := packageField(a, key), packageField(b, key); av != bv {
				return av < bv
			}
		}
		return false
	})
}

func packageField(record map[string]any, key string) string {
	value, _ := record[key].(string)
	return value
}

// dpkgPackages parses /var/lib/dpkg/status, keeping only "install ok installed"
// entries. Multiarch siblings (same name, different Architecture) are kept.
func dpkgPackages(readFile fileReader) []any {
	data, err := readFile("/var/lib/dpkg/status")
	if err != nil {
		return nil
	}
	var records []any
	for _, stanza := range strings.Split(string(data), "\n\n") {
		var name, version, arch, status string
		for line := range strings.Lines(stanza) {
			key, value, ok := strings.Cut(strings.TrimRight(line, "\n"), ": ")
			if !ok {
				continue
			}
			switch key {
			case "Package":
				name = value
			case "Version":
				version = value
			case "Architecture":
				arch = value
			case "Status":
				status = value
			}
		}
		if name == "" || version == "" || !dpkgInstalled(status) {
			continue
		}
		records = append(records, packageRecord(name, version, "architecture", arch))
	}
	sortPackages(records)
	return records
}

// dpkgInstalled reports whether a dpkg Status line's current-state component
// (the third field of "<want> ok <state>") is "installed", so held packages
// ("hold ok installed") are kept while removed/config-files entries are dropped.
func dpkgInstalled(status string) bool {
	fields := strings.Fields(status)
	return len(fields) == 3 && fields[2] == "installed"
}

// rpmPackages runs one epoch-bearing rpm query (a bare `rpm -qa` omits the epoch
// that separates otherwise-identical install-only kernels). The (none) epoch is
// stripped; gpg-pubkey pseudo-packages are filtered.
func rpmPackages(run commandRunner) []any {
	out := run("rpm", "-qa", "--qf", "%{NAME}|%{EPOCH}:%{VERSION}-%{RELEASE}|%{ARCH}\n")
	var records []any
	for line := range strings.Lines(out) {
		fields := strings.SplitN(strings.TrimSpace(line), "|", 3)
		if len(fields) != 3 {
			continue
		}
		name, version, arch := fields[0], strings.TrimPrefix(fields[1], "(none):"), fields[2]
		if name == "" || name == "gpg-pubkey" {
			continue
		}
		if arch == "(none)" {
			arch = ""
		}
		records = append(records, packageRecord(name, version, "architecture", arch))
	}
	sortPackages(records)
	return records
}

// rpmDatabasePresent reports whether an rpm database directory exists, so the rpm
// query is not spawned on dpkg/apk/pacman-only hosts.
func rpmDatabasePresent(stat func(string) (os.FileInfo, error)) bool {
	for _, dir := range []string{"/var/lib/rpm", "/usr/lib/sysimage/rpm"} {
		if info, err := stat(dir); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

// apkPackages parses /lib/apk/db/installed (blank-line-separated stanzas with
// P:/V:/A: fields).
func apkPackages(readFile fileReader) []any {
	data, err := readFile("/lib/apk/db/installed")
	if err != nil {
		return nil
	}
	var records []any
	for _, stanza := range strings.Split(string(data), "\n\n") {
		var name, version, arch string
		for line := range strings.Lines(stanza) {
			key, value, ok := strings.Cut(strings.TrimRight(line, "\n"), ":")
			if !ok {
				continue
			}
			switch key {
			case "P":
				name = value
			case "V":
				version = value
			case "A":
				arch = value
			}
		}
		if name == "" || version == "" {
			continue
		}
		records = append(records, packageRecord(name, version, "architecture", arch))
	}
	sortPackages(records)
	return records
}

// pacmanPackages reads each /var/lib/pacman/local/*/desc entry.
func pacmanPackages(glob pathGlobber, readFile fileReader) []any {
	paths, err := glob("/var/lib/pacman/local/*/desc")
	if err != nil {
		return nil
	}
	var records []any
	for _, path := range paths {
		data, err := readFile(path)
		if err != nil {
			continue
		}
		name, version, arch := parsePacmanDesc(string(data))
		if name == "" || version == "" {
			continue
		}
		records = append(records, packageRecord(name, version, "architecture", arch))
	}
	sortPackages(records)
	return records
}

// parsePacmanDesc reads the %NAME%/%VERSION%/%ARCH% sections whose value is the
// line following the section header.
func parsePacmanDesc(input string) (name, version, arch string) {
	lines := strings.Split(input, "\n")
	for i := 0; i+1 < len(lines); i++ {
		switch strings.TrimSpace(lines[i]) {
		case "%NAME%":
			name = strings.TrimSpace(lines[i+1])
		case "%VERSION%":
			version = strings.TrimSpace(lines[i+1])
		case "%ARCH%":
			arch = strings.TrimSpace(lines[i+1])
		}
	}
	return name, version, arch
}
