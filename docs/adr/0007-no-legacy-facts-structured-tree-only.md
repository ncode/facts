# No legacy facts; the structured tree is the only fact surface

Ruby Facter carries ~150 flat "legacy" aliases of structured facts (`operatingsystem` for `os.name`, `hostname`/`fqdn`/`domain` for `networking.*`, `processorcount` for `processors.count`, per-interface `mtu_*`/`ipaddress_*`, `ssh*key`/`sshfp_*`), deprecated since Facter 3 and hidden behind `--show-legacy`. The Go port reproduced them twice: a `LegacyFacts` set appended under `--show-legacy`, and — by accident — 25 untagged legacy-named facts leaking into default output (51 top-level facts on macOS vs Ruby's 22, found in the 2026-06-11 host comparison against Ruby Facter 4.10.0). With the project unreleased and no known deployments, we remove the layer entirely: no legacy alias resolves anywhere — not in default output, not under any flag, not as an explicit query, not in the library Snapshot. The deletion boundary is Ruby's own classification (Ruby's `--show-legacy` output minus its default output), so structured facts Ruby shows by default (`kernel*`, `timezone`, `dmi`, `os`, `networking`, …) are untouched.

The removal is a hard break, per the ADR-0006 no-zombie-flags precedent: `--show-legacy` and `--no-show-legacy` fail as unrecognized options, the `show-legacy` `facter.conf` key is inert like any other unrecognized key, and a `legacy` blocklist entry loads without error and blocks nothing (there is nothing left to block). An explicitly queried alias such as `facter operatingsystem` behaves like any missing fact: empty output and exit 0, or a missing-fact error under `--strict`. The *legacy output format* — Ruby's name for the default `key => value` text format, implemented by the `FormatLegacy*` functions — is an unrelated concept and stays.

Consumers of legacy aliases move to the structured sources that always backed them:

| Legacy alias | Structured fact |
|---|---|
| `operatingsystem` | `os.name` |
| `operatingsystemrelease` / `operatingsystemmajrelease` | `os.release.full` / `os.release.major` |
| `osfamily` | `os.family` |
| `architecture` | `os.architecture` |
| `hardwaremodel` | `os.hardware` |
| `hostname` / `fqdn` / `domain` | `networking.hostname` / `networking.fqdn` / `networking.domain` |
| `ipaddress` / `ipaddress6` / `macaddress` / `netmask` / `network` / `interfaces` | `networking.*` and `networking.interfaces.<name>.*` |
| `ipaddress_<if>` / `macaddress_<if>` / `mtu_<if>` / `netmask*_<if>` / `network*_<if>` / `scope6_<if>` | `networking.interfaces.<name>.*` |
| `processorcount` / `physicalprocessorcount` / `processor<N>` / `hardwareisa` | `processors.count` / `processors.physicalcount` / `processors.models` / `processors.isa` |
| `memorysize` / `memorysize_mb` / `memoryfree` / `memoryfree_mb` | `memory.system.*` |
| `swapsize` / `swapfree` / `swapencrypted` (and `_mb`) | `memory.swap.*` |
| `uptime` / `uptime_seconds` / `uptime_hours` / `uptime_days` | `system_uptime.*` |
| `sshrsakey` / `sshecdsakey` / `sshed25519key` / `sshfp_*` | `ssh.<alg>.key` / `ssh.<alg>.fingerprints.*` |
| `id` / `gid` | `identity.user` / `identity.group` |
| `manufacturer` / `productname` / `serialnumber` / `uuid` / `bios_*` / `board*` / `chassis*` | `dmi.*` |
| `dhcp_servers` | `networking.dhcp` and `networking.interfaces.<name>.dhcp` |
| `selinux*` | `os.selinux.*` |
| `lsbdist*` / `lsbrelease` / `lsbmajdistrelease` / `lsbminordistrelease` | `os.distro.*` |
| `macosx_*` | `os.macosx.*` |
| `sp_*` | `system_profiler.*` |
| `system32` / `windows_*` | `os.windows.*` |
| `rubyversion` / `rubyplatform` / `rubysitedir` | `ruby.*` |
| `blockdevices` / `blockdevice_*` | `disks.*` |
| `augeasversion` | `augeas.version` |
| `xendomains` | `xen.domains` |

Anything truly unmapped (none known) would be added as a structured fact, never as an alias — aliasing is the layer being removed.

## Considered Options

- **Ruby-parity hiding: tag the 25 leaked facts `Type: "legacy"` and keep `--show-legacy`** — rejected: it fixes the default-output bug but ships a deprecated alias layer permanently just to match a compatibility flag; that optimizes for a past nobody here has, and every alias is a name we can never reuse in the structured tree.
- **Alias-to-structured redirects (`facter operatingsystem` answers with `os.name`)** — rejected: a hidden aliasing table is the same compatibility layer in a different shape, and silently answering retired names hides the migration instead of completing it.
- **Keep flags as accepted no-ops** — rejected per ADR-0006: with zero adoption there is nobody to protect from a usage error, and zombie flags misrepresent what the binary does.
- **Remove legacy facts entirely (chosen)** — breaking only on paper; the project is unreleased, so the contract amendment is free now and expensive after the first release. Release gates and acceptance tests swap to structured names in the same change, keeping coverage identical.
