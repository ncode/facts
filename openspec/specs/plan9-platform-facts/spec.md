# plan9-platform-facts Specification

## Purpose
TBD - created by archiving change add-plan9-platform-facts. Update Purpose after archive.
## Requirements
### Requirement: Plan 9 identity facts
Facts SHALL emit canonical Plan 9 identity facts when running on `GOOS=plan9`.

#### Scenario: Plan 9 OS and kernel identity
- **WHEN** the `facts` CLI runs on Plan 9
- **THEN** `os.name` MUST be `Plan 9`
- **AND** `os.family` MUST be `Plan 9`
- **AND** `kernel.name` MUST be `Plan 9`

#### Scenario: Plan 9 hostname
- **WHEN** `/dev/sysname` contains a non-empty system name
- **THEN** Facts MUST use the trimmed `/dev/sysname` value for `networking.hostname`

#### Scenario: Missing Plan 9 hostname source
- **WHEN** `/dev/sysname` is missing or empty
- **THEN** Facts MUST omit `networking.hostname` rather than inventing a hostname

### Requirement: Plan 9 release and version facts are not invented
Facts SHALL NOT derive OS or kernel release facts from Plan 9 sources whose semantics do not match those facts.

#### Scenario: Plan 9 protocol version is not OS release
- **WHEN** `/dev/osversion` is present on Plan 9
- **THEN** Facts MUST NOT use it as `os.release`, `os.release.full`, `os.release.major`, `kernel.release.full`, `kernel.release.major`, or `kernel.version.full`

#### Scenario: Unsupported Plan 9 release facts
- **WHEN** the first Plan 9 support slice runs
- **THEN** Facts MUST omit `os.release.*` and `kernel.release.*` facts unless a later spec identifies a correct Plan 9 release source

### Requirement: Plan 9 architecture and hardware facts
Facts SHALL report Plan 9 architecture and hardware facts from Go runtime values and native Plan 9 CPU sources.

#### Scenario: Plan 9 architecture
- **WHEN** Facts runs on Plan 9
- **THEN** `os.architecture` MUST be derived from the Plan 9 architecture value, preferring `$objtype` when available and falling back to `runtime.GOARCH`
- **AND** `os.hardware` MUST be derived from the same architecture source

#### Scenario: Plan 9 processor ISA
- **WHEN** Facts can read `$cputype`, `$objtype`, or `runtime.GOARCH`
- **THEN** `processors.isa` MUST contain the best available Plan 9 processor architecture value

#### Scenario: Plan 9 processor model
- **WHEN** `/dev/cputype` or `/dev/archctl` contains a CPU model string
- **THEN** Facts MUST include that model in `processors.models`

### Requirement: Plan 9 memory facts
Facts SHALL report Plan 9 memory totals from `/dev/swap`.

#### Scenario: Plan 9 total memory
- **WHEN** `/dev/swap` contains a line matching `<bytes> memory`
- **THEN** `memory.system.total_bytes` MUST equal that byte count
- **AND** `memory.system.total` MUST be the human-readable value derived from that byte count using the existing Facts formatting rules

#### Scenario: Plan 9 memory parser ignores unsupported lines
- **WHEN** `/dev/swap` includes page size, user page, or swap page lines
- **THEN** the first Plan 9 support slice MUST NOT require those lines to produce `memory.system.available`, `memory.system.used`, or `memory.swap.*`

#### Scenario: Plan 9 memory source missing
- **WHEN** `/dev/swap` cannot be read or does not contain a memory total
- **THEN** Facts MUST omit Plan 9 memory facts rather than returning zero

### Requirement: Plan 9 processor count facts
Facts SHALL report Plan 9 processor count from `/dev/sysstat`.

#### Scenario: Plan 9 processor count
- **WHEN** `/dev/sysstat` contains one or more processor status lines
- **THEN** `processors.count` MUST equal the number of non-empty processor status lines

#### Scenario: Plan 9 processor count source missing
- **WHEN** `/dev/sysstat` cannot be read or contains no processor status lines
- **THEN** Facts MUST omit `processors.count` rather than returning zero

### Requirement: Plan 9 basic IPv4 networking facts
Facts SHALL report basic Plan 9 IPv4 networking facts from the native `/net` filesystem.

#### Scenario: Plan 9 interface address
- **WHEN** `/net/ipifc/*/status` contains an IPv4 address row
- **THEN** Facts MUST parse the interface IP address, network prefix, netmask, and network address from that row

#### Scenario: Plan 9 IPv4 mapped prefix
- **WHEN** a Plan 9 IPv4 address row uses an IPv6-mapped prefix such as `/120`
- **THEN** Facts MUST convert it to the equivalent IPv4 prefix length such as `/24` before deriving `networking.netmask` and `networking.network`

#### Scenario: Plan 9 interface MAC address
- **WHEN** the Plan 9 interface device has an `addr` file such as `/net/ether0/addr`
- **THEN** Facts MUST parse that value as the interface MAC address

#### Scenario: Plan 9 primary network
- **WHEN** `/net/iproute` contains a default IPv4 route
- **THEN** Facts MUST use that route to select `networking.primary` and the top-level `networking.ip`, `networking.netmask`, `networking.network`, and `networking.mac` values

#### Scenario: Plan 9 DHCP metadata
- **WHEN** `/net/ndb` lacks DHCP server metadata
- **THEN** Facts MUST omit DHCP server facts rather than inventing a value

### Requirement: Plan 9 uptime facts
Facts SHALL report Plan 9 uptime from the native Plan 9 `uptime` command.

#### Scenario: Plan 9 uptime format
- **WHEN** `uptime` returns a value like `cirno up 0 days, 01:35:26`
- **THEN** Facts MUST parse the day, hour, minute, and second values into `system_uptime.seconds`
- **AND** Facts MUST produce the existing `system_uptime` structure from that duration

#### Scenario: Plan 9 uptime source missing
- **WHEN** `uptime` cannot be executed or does not match the Plan 9 format
- **THEN** Facts MUST omit uptime facts rather than returning zero

### Requirement: Plan 9 timezone facts
Facts SHALL report the Plan 9 timezone using Go local time behavior or native Plan 9 timezone sources.

#### Scenario: Plan 9 timezone from Go runtime
- **WHEN** Go's local time support returns a timezone abbreviation on Plan 9
- **THEN** Facts MUST use the existing timezone fact path and formatting

#### Scenario: Plan 9 timezone fallback
- **WHEN** Go's local time support cannot provide the timezone abbreviation
- **THEN** Facts MAY parse `date`, `date -t`, `/env/timezone`, or `/adm/timezone/local` to produce the existing timezone fact

### Requirement: Plan 9 unsupported first-slice facts
Facts SHALL omit Plan 9 facts whose native semantics are unresolved in the first support slice.

#### Scenario: Plan 9 mount and filesystem facts
- **WHEN** Facts runs on Plan 9 in the first support slice
- **THEN** Facts MUST NOT claim support for mountpoint capacity, filesystem inventory, disk inventory, or partition inventory

#### Scenario: Plan 9 load facts
- **WHEN** `/dev/sysstat` exposes Plan 9 load data
- **THEN** Facts MUST NOT map it to `load_averages` unless a later spec defines the conversion

#### Scenario: Plan 9 virtualization facts
- **WHEN** virtio or VM-specific devices are visible on the lab guest
- **THEN** Facts MUST NOT require exact virtualization classification in the first Plan 9 release gate

