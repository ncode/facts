## ADDED Requirements

### Requirement: Supported targets report their native installed-package sources

Facts SHALL resolve the `packages` fact on each supported release target using the package source(s) native to that target.

#### Scenario: Linux targets report their package database and cross-distro sources

- **WHEN** packages are discovered on a supported Linux target
- **THEN** Debian and Ubuntu MUST report `packages.dpkg`
- **AND** the RHEL family and SUSE MUST report `packages.rpm`
- **AND** Arch MUST report `packages.pacman` and Alpine MUST report `packages.apk`
- **AND** `packages.snap` and `packages.flatpak` MUST be reported wherever those system databases are present
- **AND** `packages.nix` MUST be reported wherever a Nix profile is present

#### Scenario: BSD and illumos targets report their package database

- **WHEN** packages are discovered on a supported BSD or illumos target
- **THEN** FreeBSD and DragonFly MUST report `packages.pkg`
- **AND** OpenBSD MUST report `packages.openbsd_pkg`
- **AND** NetBSD MUST report `packages.pkgsrc`
- **AND** illumos MUST report `packages.ips`

#### Scenario: macOS reports receipts, apps, and optional homebrew

- **WHEN** packages are discovered on macOS
- **THEN** `packages.receipts` MUST be reported from the installer receipt database
- **AND** `packages.apps` MUST be reported from the installed `.app` inventory and MUST NOT be merged into `receipts`
- **AND** `packages.homebrew` MUST be reported only when a Homebrew prefix exists

#### Scenario: Windows reports registry and appx

- **WHEN** packages are discovered on Windows
- **THEN** `packages.registry` MUST be reported from both HKLM uninstall hives
- **AND** `packages.appx` MUST be reported from the provisioned set plus the collector context

#### Scenario: Plan 9 reports no packages

- **WHEN** packages are discovered on Plan 9
- **THEN** no `packages` subtree MUST be emitted

### Requirement: Package collection stays cheap and context-bounded

Facts SHALL collect each package source with a single cheap read and SHALL report only system-global databases plus the collector's own execution context.

#### Scenario: no per-package process spawning or network

- **WHEN** a package source is collected
- **THEN** it MUST be read with one on-disk database read or one batch query
- **AND** it MUST NOT spawn one process per package
- **AND** it MUST NOT perform any network request

#### Scenario: other users' per-user installs are not enumerated

- **WHEN** a host has per-user installs owned by other users (homebrew, nix profiles, Windows HKCU, per-user flatpak or appx)
- **THEN** discovery MUST NOT drop privileges or walk other users' homes to enumerate them
- **AND** the under-report MUST be documented
