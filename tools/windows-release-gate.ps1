# Windows release gate: builds the real cmd/facts binary and verifies the
# Windows release-gate fact set through the CLI. Exits non-zero on any
# failure; CI treats this as a blocking release gate (see CONTRIBUTING.md).
param(
    [string]$FactsPath = ""
)

$ErrorActionPreference = "Stop"
$RemoveBuiltFacts = $false

$isWindowsHost = $IsWindows
if ($null -eq $isWindowsHost) {
    $isWindowsHost = $env:OS -eq "Windows_NT"
}

if (-not $isWindowsHost) {
    Write-Error "windows-release-gate.ps1 must run on Windows"
    exit 1
}

try {
    if ($FactsPath -eq "") {
        $FactsPath = Join-Path ([System.IO.Path]::GetTempPath()) ("facts-release-gate-{0}.exe" -f [System.Guid]::NewGuid())
        $RemoveBuiltFacts = $true
        & go build -o $FactsPath ./cmd/facts
        if ($LASTEXITCODE -ne 0) {
            throw "go build ./cmd/facts failed with exit code $LASTEXITCODE"
        }
    }

$factSet = @(
    "os.name",
    "os.family",
    "os.release",
    "os.architecture",
    "os.hardware",
    "os.windows.system32",
    "kernel.name",
    "virtual",
    "is_virtual",
    "networking",
    "memory",
    "memory.system.total",
    "processors",
    "processors.count",
    "dmi",
    "system_uptime",
    "fips_enabled",
    "timezone",
    "path",
    "facterversion",
    "kernel.release.full",
    "kernel.version.full",
    "kernel.release.major"
)

$output = & $FactsPath --json @factSet
if ($LASTEXITCODE -ne 0) {
    throw "facts Windows release-gate command failed with exit code $LASTEXITCODE"
}

$json = $output -join [Environment]::NewLine
Write-Output $json
$factsObject = $json | ConvertFrom-Json
$facts = New-Object 'System.Collections.Hashtable' -ArgumentList ([System.StringComparer]::Ordinal)
foreach ($property in $factsObject.PSObject.Properties) {
    $facts[$property.Name] = $property.Value
}

function Assert-Key($key) {
    if (-not $facts.ContainsKey($key)) {
        throw "missing fact $key"
    }
}

function Assert-Equals($key, $want) {
    Assert-Key $key
    $got = $facts[$key]
    if ($got -ne $want) {
        throw "$key = '$got', want '$want'"
    }
}

function Assert-NonEmpty($key) {
    Assert-Key $key
    $got = $facts[$key]
    if ($null -eq $got -or $got -eq "") {
        throw "$key is empty"
    }
}

function Assert-Map($key) {
    Assert-Key $key
    $got = $facts[$key]
    if ($got -is [System.Collections.IDictionary] -and $got.Count -gt 0) {
        return
    }
    if ($got -is [pscustomobject] -and $got.PSObject.Properties.Count -gt 0) {
        return
    }
    throw "$key = '$got', want non-empty map"
}

foreach ($fact in $factSet) {
    Assert-Key $fact
}

Assert-Equals "os.name" "windows"
Assert-Equals "os.family" "windows"
Assert-Equals "kernel.name" "windows"

Assert-NonEmpty "os.release"
Assert-NonEmpty "os.architecture"
Assert-NonEmpty "os.hardware"
Assert-NonEmpty "os.windows.system32"
Assert-NonEmpty "virtual"
Assert-NonEmpty "timezone"
Assert-NonEmpty "path"
Assert-NonEmpty "facterversion"
Assert-NonEmpty "kernel.release.full"
Assert-NonEmpty "kernel.version.full"
Assert-NonEmpty "kernel.release.major"
Assert-NonEmpty "memory.system.total"
Assert-NonEmpty "processors.count"

Assert-Map "networking"
Assert-Map "memory"
Assert-Map "processors"
Assert-Map "dmi"
Assert-Map "system_uptime"

if (-not ($facts["is_virtual"] -is [bool])) {
    throw "is_virtual = '$($facts['is_virtual'])', want boolean"
}

if (-not ($facts["fips_enabled"] -is [bool])) {
    throw "fips_enabled = '$($facts['fips_enabled'])', want boolean"
}

    Write-Output "windows-release-gate: all facts present"
}
catch {
    Write-Error "windows-release-gate failed: $_"
    exit 1
}
finally {
    if ($RemoveBuiltFacts -and (Test-Path -LiteralPath $FactsPath)) {
        Remove-Item -LiteralPath $FactsPath -Force -ErrorAction SilentlyContinue
    }
}
exit 0
