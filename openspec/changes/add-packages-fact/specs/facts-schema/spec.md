## ADDED Requirements

### Requirement: Installed packages are reported as source-namespaced record lists

Facts SHALL document the `packages` fact as a map from package-source key to an array of package records, where each record is one installed package identified by its fields, and records are never merged across sources.

#### Scenario: packages schema is source-namespaced arrays

- **WHEN** a contributor reads `docs/schema/facts.yaml`
- **THEN** `packages` MUST be documented as a map keyed by source
- **AND** each `packages.<source>` MUST have type `array`
- **AND** each array element MUST be a package record (a map), never a bare scalar
- **AND** the documented source keys MUST be `dpkg`, `rpm`, `pacman`, `apk`, `snap`, `flatpak`, `pkg`, `openbsd_pkg`, `pkgsrc`, `ips`, `nix`, `receipts`, `apps`, `homebrew`, `registry`, and `appx`

#### Scenario: package records carry name and version

- **WHEN** a package record is documented
- **THEN** it MUST include `name` and `version` as always-present strings
- **AND** `version` MUST be the package manager's verbatim native version string, not decomposed into separate epoch or release fields

#### Scenario: per-source identity fields are documented

- **WHEN** a source needs more than `name` and `version` to keep two installs distinct
- **THEN** the schema MUST document its identity fields: `architecture` for `dpkg`/`rpm`/`apk`, `type` and `tap` for `homebrew`, `branch` and `architecture` for `flatpak`, `store_path` for `nix`, `bundle_id` and `path` for `apps`, and `product_code` and `architecture` for `registry`

### Requirement: Package sources are keyed by the installed-package database

Facts SHALL key each package source by its installed-package database, never by a package-manager frontend or an artifact file format.

#### Scenario: frontends collapse to their database

- **WHEN** packages are read on a host managed with apt/aptitude or dnf/yum/zypper
- **THEN** they MUST be reported under `packages.dpkg` or `packages.rpm` respectively
- **AND** `packages.apt`, `packages.deb`, `packages.dnf`, `packages.yum`, and `packages.zypper` MUST NOT be documented as sources

#### Scenario: IPS is keyed ips, not pkg

- **WHEN** packages are read on illumos or Solaris
- **THEN** they MUST be reported under `packages.ips`
- **AND** `packages.pkg` MUST refer only to the FreeBSD and DragonFly pkgng database

### Requirement: Package records preserve every installed instance

Facts SHALL keep one record per installed package instance so that same-named installs are never collapsed.

#### Scenario: colliding installs are all kept

- **WHEN** a host has two installs sharing a name (dpkg multiarch, rpm multiversion kernels, homebrew formula and cask, flatpak branches, Windows x86 and x64)
- **THEN** `packages.<source>` MUST contain a distinct record for each
- **AND** the records MUST differ in at least one per-source identity field

#### Scenario: absent sources are omitted and never merged

- **WHEN** a package source's database is not present on the host
- **THEN** its `packages.<source>` key MUST be absent, not an empty array
- **AND** records from different sources MUST NOT be merged or deduplicated into one list
