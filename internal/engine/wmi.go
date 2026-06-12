package engine

import "strings"

// wmicAliasClasses maps the wmic aliases used by the Windows resolvers to
// their CIM class names for the PowerShell fallback.
var wmicAliasClasses = map[string]string{
	"os":                    "Win32_OperatingSystem",
	"cpu":                   "Win32_Processor",
	"bios":                  "Win32_BIOS",
	"computersystem":        "Win32_ComputerSystem",
	"computersystemproduct": "Win32_ComputerSystemProduct",
}

// windowsWMIOutput queries WMI through wmic when it is available and falls
// back to a PowerShell Get-CimInstance query emitting the same
// Property=Value record shape. wmic ships through Windows Server 2022 but
// was removed from Windows 11 and Windows Server 2025; Ruby Facter is
// unaffected because it queries WMI over COM.
func windowsWMIOutput(run commandRunner, alias, props string) string {
	if out := run("wmic", alias, "get", props, "/value"); strings.Contains(out, "=") {
		return out
	}
	class := wmicAliasClasses[alias]
	if class == "" {
		return ""
	}
	return run("powershell", "-NoProfile", "-NonInteractive", "-Command", windowsCIMScript(class, props))
}

// windowsCIMScript builds a PowerShell script that prints Property=Value
// lines per CIM instance, separated by blank lines, matching `wmic ... get
// ... /value` output: DMTF datetimes and `{"a","b"}` array values included.
func windowsCIMScript(class, props string) string {
	quoted := make([]string, 0, 8)
	for prop := range strings.SplitSeq(props, ",") {
		quoted = append(quoted, "'"+strings.TrimSpace(prop)+"'")
	}
	return "$ErrorActionPreference='SilentlyContinue';" +
		"foreach ($o in @(Get-CimInstance -ClassName " + class + ")) {" +
		" foreach ($p in @(" + strings.Join(quoted, ",") + ")) {" +
		" $v = $o.$p;" +
		" if ($v -is [datetime]) { $v = [System.Management.ManagementDateTimeConverter]::ToDmtfDateTime($v) }" +
		" elseif ($v -is [array]) { $v = '{' + (($v | ForEach-Object { '\"' + $_ + '\"' }) -join ',') + '}' };" +
		" Write-Output ($p + '=' + $v) };" +
		" Write-Output '' }"
}
