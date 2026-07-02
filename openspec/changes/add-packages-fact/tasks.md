## 1. Tests First

- [x] 1.1 Record-shape tests: every reader asserts `name`+verbatim `version`; `packages.<source>` is a `[]any` array, never a name-keyed map.
- [x] 1.2 Collision/siblings tests: dpkg multiarch (`libc6:amd64`+`:i386`) kept; rpm epoch-bearing multiversion kernels; homebrew formula-vs-cask distinguished by `type`; Windows x86/x64 by `architecture`; flatpak same-app-same-version siblings distinguished by `branch` (observed live: GL.default 25.08 vs 25.08-extra).
- [x] 1.3 Identity-field tests: `architecture` (dpkg/rpm/apk/pkg/flatpak), `branch` (flatpak), `type` (homebrew formula/cask), `bundle_id`+`path` (apps), `product_code`+`architecture` (registry). Deferred vs design (documented): homebrew `tap` (needs per-formula receipt read) and nix `store_path` (conflicts with the output-dedup — a derivation's outputs have distinct store paths; resolving the primary output is future work).
- [x] 1.4 Source omitted when its database is absent (or empty); records never merged across sources; Plan 9 emits no `packages` subtree (no darwin/linux/bsd/windows/illumos case matches).
- [x] 1.5 Reader/parse fixtures per source from **real** captured output (dpkg status, `rpm -qa` epoch query, pacman `desc`, apk `installed`, pkgng `pkg query`, openbsd `/var/db/pkg`, pkgsrc PKG_DBDIR, `pkg list -H`, `nix-store -q --references`, macOS receipts/apps `plutil -p`, registry/appx PowerShell lines, snap/flatpak columns).

## 2. Implementation

- [x] 2.1 `internal/engine/packages.go` with `packagesCoreFacts(s *Session) []ResolvedFact`; composed into `buildCoreFacts` as a gated resolver. Pure `parse*` functions, no GOOS-suffixed files (ADR-0010) — per-platform readers live in `packages_bsd.go`/`packages_mac.go`/`packages_win.go`/`packages_extra.go` with no build constraints.
- [x] 2.2 Linux readers: `dpkg` (install/hold state kept), `rpm` (epoch query, gpg-pubkey filtered, DB-presence-gated), `pacman`, `apk`, `snap`, `flatpak`.
- [x] 2.3 BSD/illumos readers: `pkg` (FreeBSD+DragonFly, absolute `/usr/local/sbin/pkg`), `openbsd_pkg`, `pkgsrc` (NetBSD, PKG_DBDIR discovery), `ips` (illumos).
- [x] 2.4 macOS readers: `receipts` (primary), `apps` (secondary, never merged), `homebrew` (auto-detected); and `nix` (Linux, NixOS system profile).
- [x] 2.5 Windows readers: `registry` (both HKLM hives, `product_code`+`architecture`) and `appx` (provisioned + collector context).
- [x] 2.6 One cheap read/query per source; no spawn-per-package; no network; commands outside the trusted PATH use absolute paths; presence-gated spawns.
- [x] 2.7 `packages` registers as a gated fact group (`--disable packages` skips resolution).

## 3. Schema and Docs

- [x] 3.1 `packages.*` added to `docs/schema/facts.yaml` (one array entry per source, per-platform applicability, conditional).
- [x] 3.2 Regenerated `docs/supported-facts/`.
- [x] 3.3 README, man page, and CHANGELOG mention the `packages` fact and scope (system package databases; language managers out).

## 4. Verification

- [x] 4.1 Focused resolver/parser tests for every source (green).
- [x] 4.2 `go test ./...` and `go vet ./...` (green); `gofmt` clean.
- [x] 4.3 nlab/local validation, counts vs native tools: dpkg 324, rpm 386, pacman 183, apk 200, pkg freebsd 2 / dragonfly 48, openbsd_pkg 18, pkgsrc 19, ips 418, nix 70, macOS receipts 10 / apps 61 / homebrew 88 — all exact matches; Plan 9 emits nothing. Windows `registry` is **populated-validated** on the nlab Server 2025 guest: installed 7-Zip x64 + x86 (MSI), and `facts.registry` reports both — `7-Zip 24.08 (x64 edition)`/x64 from the native hive and `7-Zip 24.08`/x86 from WOW6432Node, each with the correct MSI `product_code` GUID — matching the live registry exactly (dual-hive read + architecture + product_code all confirmed). Windows `appx`: PowerShell script syntax verified on the guest and parse logic unit-tested; correctly omitted on the appx-less Server (populating appx needs a signed MSIX — disproportionate for a secondary source). `snap` and `flatpak` are **populated-validated** on the nlab ubuntu2404 guest: installed hello-world (snap) and org.vim.Vim from flathub (flatpak); `facts.snap` = 3 = `snap list`, `facts.flatpak` = 4 = the versioned `flatpak list` rows, with the same-app-same-version GL.default siblings distinguished by `branch` and the versionless codecs-extra extension dropped by the name+version invariant — all alongside dpkg (740) on the same host.
- [x] 4.4 `openspec validate add-packages-fact --strict`.
- [x] 4.5 Deep multi-lens adversarial review (two workflow rounds: 10 lenses total, every finding independently verified). Round 1: 13 findings fixed (dpkg trigger states, pkgsrc/nix ADR-compliance, apps Utilities globs, plutil bisection + multiline tracking + array roots, homebrew prefix, packages fact group, 6 schema drifts). Round 2: 11 confirmed findings fixed — Windows appx DISM numeric architecture mapping, UTF-8 output encoding for PowerShell (OEM-codepage mojibake, verified live), `;exit 0` partial-output survival (verified live), ARM64-aware native-hive architecture, unit-separator registry delimiter, nix custom-output collapse (bind dnsutils/host) and Determinate-Nix system-set fallback, YAML plain-scalar quoting for Psych-retyped versions (`43_1`→431, `2026-05-14`→Date crash under safe_load — verified against Ruby Psych, round-trip now clean), plutil scalar-root blocks + block-count-validated bisection, gating-test probe cost. Windows fixes re-validated on the populated guest (registry 2 records, derived x64, GUIDs intact).
