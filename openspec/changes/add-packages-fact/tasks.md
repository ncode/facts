## 1. Tests First

- [ ] 1.1 Add record-shape tests: every record carries `name` and a verbatim `version`; `packages.<source>` is an array, never a name-keyed map.
- [ ] 1.2 Add collision tests proving siblings are kept: dpkg multiarch (`libc6:amd64` + `:i386`), rpm install-only multiversion kernels (same name + arch), homebrew formula-vs-cask `docker`, flatpak branches, Windows x86/x64 same DisplayName.
- [ ] 1.3 Add per-source identity-field tests: `architecture` (dpkg/rpm/apk), `type`+`tap` (homebrew), `branch`+`architecture` (flatpak), `store_path` (nix), `bundle_id`+`path` (apps), `product_code`+`architecture` (registry).
- [ ] 1.4 Add tests that a source is omitted when its database is absent, that records are never merged across sources, and that Plan 9 emits no `packages` subtree.
- [ ] 1.5 Add reader/parse fixtures per source (dpkg status, `rpm -qa` output, pacman `desc`, apk `installed`, snapd state, flatpak list, pkgng query, openbsd `/var/db/pkg`, pkgsrc PKG_DBDIR, ips `pkg list`, nix profile manifest, macOS receipts plist + Info.plist, registry hive, appx).

## 2. Implementation

- [ ] 2.1 Add `internal/engine/packages.go` with `packagesCoreFacts(s *Session) []ResolvedFact`; compose it into `buildCoreFacts`. Pure `parse*` functions per source, no GOOS-suffixed files (ADR-0010).
- [ ] 2.2 Implement the Linux readers: `dpkg`, `rpm` (explicit epoch-bearing query), `pacman`, `apk`, `snap`, `flatpak`.
- [ ] 2.3 Implement the BSD/illumos readers: `pkg` (FreeBSD + DragonFly), `openbsd_pkg`, `pkgsrc` (NetBSD), `ips` (illumos).
- [ ] 2.4 Implement the macOS readers: `receipts` (primary), `apps` (secondary, never merged), `homebrew` (optional auto-detected); and `nix` where a profile is present.
- [ ] 2.5 Implement the Windows readers: `registry` (both HKLM hives, `product_code`+`architecture`) and `appx` (provisioned + collector context).
- [ ] 2.6 Enforce the collection rule (one cheap read per source; no spawn-per-package; no network) and the system-global + collector-context boundary.
- [ ] 2.7 Register `packages` as a fact group (no default-visibility change in this work).

## 3. Schema and Docs

- [ ] 3.1 Add `packages.*` to `docs/schema/facts.yaml` under ADR-0011 canonical spelling: the per-source arrays, the always-present `name`/`version`, and the per-source identity fields, with per-platform applicability.
- [ ] 3.2 Regenerate `docs/supported-facts/` to show the per-source record shapes.
- [ ] 3.3 Update README, man page, and CHANGELOG to mention the `packages` fact and its scope (system package databases; language managers out).

## 4. Verification

- [ ] 4.1 Run focused resolver/parser tests for every source.
- [ ] 4.2 Run `go test ./...` and `go vet ./...`.
- [ ] 4.3 Validate the fact on each supported target's native smoke/release gate (dpkg, rpm, pacman, apk, snap/flatpak where present, pkg, openbsd_pkg, pkgsrc, ips, macOS receipts/apps, windows registry/appx); confirm Plan 9 emits nothing.
- [ ] 4.4 Run `openspec validate add-packages-fact --strict`.
