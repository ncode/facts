package engine

import "strings"

// registryPackagesScript queries both HKLM uninstall hives in a single
// PowerShell invocation: the native 64-bit view and the 32-bit WOW6432Node
// redirect. Each entry that carries a DisplayName is emitted as one
// pipe-delimited line. The architecture marker leads and the free-text
// DisplayName trails so a stray '|' inside a product name cannot shift the
// fixed columns. Columns: arch|PSChildName|DisplayVersion|SystemComponent|DisplayName.
const registryPackagesScript = `$ErrorActionPreference='SilentlyContinue';` +
	`Get-ItemProperty 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\*'|Where-Object DisplayName|ForEach-Object{"x64|$($_.PSChildName)|$($_.DisplayVersion)|$($_.SystemComponent)|$($_.DisplayName)"};` +
	`Get-ItemProperty 'HKLM:\SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\*'|Where-Object DisplayName|ForEach-Object{"x86|$($_.PSChildName)|$($_.DisplayVersion)|$($_.SystemComponent)|$($_.DisplayName)"}`

// registryPackages reads installed programs from the two HKLM uninstall hives.
// Every subkey with a DisplayName becomes a record; system-component/update
// entries (SystemComponent=1) are dropped, matching what Programs & Features
// shows. The architecture reflects the hive (x64 native, x86 WOW6432Node) and
// product_code is set only when the subkey name is an MSI product GUID.
func registryPackages(run commandRunner) []any {
	out := run("powershell", "-NoProfile", "-NonInteractive", "-Command", registryPackagesScript)
	var records []any
	for line := range strings.Lines(out) {
		fields := strings.SplitN(strings.TrimRight(line, "\r\n"), "|", 5)
		if len(fields) != 5 {
			continue
		}
		arch, subkey, version, systemComponent, name := fields[0], fields[1], fields[2], fields[3], fields[4]
		if name == "" || version == "" || systemComponent == "1" {
			continue
		}
		records = append(records, packageRecord(name, version,
			"product_code", msiProductCode(subkey),
			"architecture", arch))
	}
	sortPackages(records)
	return records
}

// appxPackagesScript emits the system-provisioned appx set (Get-AppxProvisionedPackage)
// followed by the collector-context packages (Get-AppxPackage), one
// Name|Version|Architecture line each. The two views are unioned and deduplicated
// downstream, so an unavailable provisioning module (common on Server) simply
// yields fewer lines rather than an error.
const appxPackagesScript = `$ErrorActionPreference='SilentlyContinue';` +
	`Get-AppxProvisionedPackage -Online|ForEach-Object{"$($_.DisplayName)|$($_.Version)|$($_.Architecture)"};` +
	`Get-AppxPackage|ForEach-Object{"$($_.Name)|$($_.Version)|$($_.Architecture)"}`

// appxPackages reads the appx/MSIX packages via PowerShell. Records carry the
// package name, version, and (lowercased) architecture; duplicates across the
// provisioned and collector-context views are collapsed.
func appxPackages(run commandRunner) []any {
	out := run("powershell", "-NoProfile", "-NonInteractive", "-Command", appxPackagesScript)
	seen := map[string]bool{}
	var records []any
	for line := range strings.Lines(out) {
		fields := strings.SplitN(strings.TrimRight(line, "\r\n"), "|", 3)
		if len(fields) != 3 {
			continue
		}
		name, version, arch := fields[0], fields[1], strings.ToLower(fields[2])
		if name == "" || version == "" {
			continue
		}
		key := name + "|" + version + "|" + arch
		if seen[key] {
			continue
		}
		seen[key] = true
		records = append(records, packageRecord(name, version, "architecture", arch))
	}
	sortPackages(records)
	return records
}

// msiProductCode returns name unchanged when it is a canonical MSI product GUID
// ({8-4-4-4-12} hex), and "" otherwise, so only genuine MSI installs contribute
// a product_code identity field (Inno Setup "*_is1" and other bespoke uninstall
// keys do not).
func msiProductCode(name string) string {
	if len(name) != 38 || name[0] != '{' || name[37] != '}' {
		return ""
	}
	for i := 1; i < 37; i++ {
		switch i {
		case 9, 14, 19, 24:
			if name[i] != '-' {
				return ""
			}
		default:
			if !isHexDigit(name[i]) {
				return ""
			}
		}
	}
	return name
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
