## Why

Facts has no inventory of installed software, and Ruby Facter never shipped one, so an operator doing compliance, drift detection, or asking "is package X installed, at what version?" has to shell out per platform. This change adds a `packages` fact — a Facts-native extension governed by ADR-0014 — that reports installed software across every supported target, namespaced by the installed-package database.

## What Changes

- Add a `packages` fact: `packages.<source>` is an **array of package records** `{name, version, …per-source identity fields}`. Name is a field, never a map key, and records are never merged or deduped across sources.
- A **source is the installed-package database**, never a frontend and never an artifact format: `dpkg` (apt/aptitude collapse here), `rpm` (dnf/yum/zypper), `pacman`, `apk`, `snap`, `flatpak`, `pkg` (FreeBSD and DragonFly — one pkgng database), `openbsd_pkg`, `pkgsrc` (NetBSD; illumos secondary), `ips` (illumos/Solaris — keyed `ips`, not `pkg`), `nix` (the installed profile set, not the whole store), macOS `receipts` + `apps` + optional `homebrew`, and Windows `registry` + `appx`. Plan 9 emits nothing — it has no package database.
- Per-record contract: `name` and `version` (the package manager's verbatim native version string) are always present; per-source identity fields (`architecture`, `type`/`tap`, `branch`, `store_path`, `bundle_id`/`path`, Windows `product_code`) are present where they are needed to keep two genuinely different installs distinct.
- Collection is **one cheap read per source** — parse the on-disk database where it is world-readable, or one batch query — never a process per package and never the network.
- The fact reports system-global databases plus whatever the collector's own execution context can read; it does not drop privileges or walk other users' homes.
- Document `packages.*` in `docs/schema/facts.yaml` under ADR-0011 canonical spelling and in the generated supported-facts pages.
- Register `packages` as a fact group; it is on by default like every other fact (ADR-0015) and removed only by disabling it (`--disable packages`, `FACTS_DISABLE=packages`, or the `facts.conf` `disable` key). The fact enable/disable mechanism itself is a separate change.

## Target Shape

```json
{
  "packages": {
    "dpkg": [
      {"name": "libc6", "version": "2.38-1ubuntu6", "architecture": "amd64"},
      {"name": "libc6", "version": "2.38-1ubuntu6", "architecture": "i386"}
    ],
    "snap": [{"name": "firefox", "version": "126.0"}],
    "rpm": [
      {"name": "kernel-core", "version": "5.14.0-427.el9", "architecture": "x86_64"},
      {"name": "kernel-core", "version": "5.14.0-362.el9", "architecture": "x86_64"}
    ],
    "homebrew": [
      {"name": "docker", "version": "27.0.3", "type": "formula", "tap": "homebrew/core"},
      {"name": "docker", "version": "4.31.0", "type": "cask", "tap": "homebrew/cask"}
    ],
    "nix": [{"name": "hello", "version": "2.12.1", "store_path": "/nix/store/abc-hello-2.12.1"}]
  }
}
```

## Impact

- **Behavior**: a new `packages` subtree on supported platforms; absent on Plan 9 and absent for any source whose database is not present.
- **Schema/docs**: `docs/schema/facts.yaml` gains `packages.*`; supported-facts pages list the per-source record shapes.
- **Compatibility**: additive — no existing fact changes shape.
- **Out of scope**: language/dev managers (pip, npm, gem, cargo); MacPorts and Chocolatey (deferred auto-detected secondaries); the fact enable/disable mechanism itself (a separate change — `packages` is default-on per ADR-0015); a separate cross-platform `applications` fact; multi-user enumeration of other users' per-user installs.
