## 1. Tests First

- [x] 1.1 Record-shape tests: every reader asserts `name`+verbatim `version`; `packages.<source>` is a `[]any` array, never a name-keyed map.
- [x] 1.2 Collision/siblings tests: dpkg multiarch (`libc6:amd64`+`:i386`) kept; rpm epoch-bearing multiversion kernels; homebrew formula-vs-cask distinguished by `type`; Windows x86/x64 by `architecture`. (flatpak branch siblings: format-only, see 4.3.)
- [x] 1.3 Identity-field tests: `architecture` (dpkg/rpm/apk/pkg/flatpak), `type` (homebrew formula/cask), `bundle_id`+`path` (apps), `product_code`+`architecture` (registry). Deferred vs design (documented): homebrew `tap` (needs per-formula receipt read), flatpak `branch`, nix `store_path` (conflicts with the output-dedup — a derivation's outputs have distinct store paths; resolving the primary output is future work).
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
- [x] 4.3 nlab/local validation, counts vs native tools: dpkg 324, rpm 386, pacman 183, apk 200, pkg freebsd 2 / dragonfly 48, openbsd_pkg 18, pkgsrc 19, ips 418, nix 70, macOS receipts 10 / apps 61 / homebrew 88 — all exact matches; Plan 9 emits nothing. **Honest gaps:** Windows `registry`/`appx` are format- and empty-case-validated on the bare Server 2025 guest (0 installed apps; could not populate it), and `snap`/`flatpak` are format-only (a guest has snapd but 0 snaps; no guest has flatpak) — none could be populated without mutating shared lab state.
- [x] 4.4 `openspec validate add-packages-fact --strict`.
