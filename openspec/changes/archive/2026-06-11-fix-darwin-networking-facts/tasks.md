# Tasks: Fix Darwin networking facts

## 1. Hostname / domain / FQDN split

- [x] 1.1 Locate the hostname probe and `networking.hostname`/`domain`/`fqdn` derivation in `internal/facter/`; confirm whether the FQDN-in-hostname bug is Darwin-only or in the shared split
- [x] 1.2 Implement Ruby's split: hostname = node name before the first dot; domain = remainder or resolver search/domain fallback; fqdn = hostname[.domain]
- [x] 1.3 Fixture tests: dotted node name, undotted node name with resolver domain, undotted with no domain — for each supported platform path the fix touches

## 2. Interface enumeration

- [x] 2.1 Enumerate `networking.interfaces` from the interface list (flags, MTU) and attach addresses where present, so address-less interfaces (macOS `gif0`/`stf0`) appear
- [x] 2.2 Fixture tests: address-less tunnel interfaces included with MTU; interfaces with addresses unchanged
- [x] 2.3 Verify Linux/Windows/FreeBSD interface fixtures still pass unmodified

## 3. Primary IPv6 selection

- [x] 3.1 Pin the selection order in code and tests: global > unique-local > link-local on the primary interface for `networking.ip6`/`network6`/`scope6`; link-local-only hosts report the link-local with scope `link`
- [x] 3.2 Add the deviation note to the man page GO PORT NOTES

## 4. Verification

- [x] 4.1 `go test ./...`, `go test -race ./...`, vet/gofmt clean; platform CI gates green
- [x] 4.2 macOS comparison rerun: `networking.hostname` = short name, `networking.fqdn` unchanged, `gif0`/`stf0` in `networking.interfaces`; CHANGELOG entry added
- [x] 4.3 `openspec validate fix-darwin-networking-facts` passes
