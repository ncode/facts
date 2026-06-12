# Design: Fix Darwin networking facts

## Context

Three defects surfaced in the 2026-06-11 macOS comparison against Ruby Facter 4.10.0 (captures in `/tmp/facter-parity/`). First, the Go hostname probe stores the full node name: `networking.hostname` = `dream-factory.lan` where Ruby reports `dream-factory` plus `domain` = `lan` and `fqdn` = `dream-factory.lan` — Ruby splits the node name at the first dot (hostname) and derives the domain from resolver configuration or the remainder. Second, Go's interface enumeration skips interfaces carrying no addresses, so macOS's default `gif0`/`stf0` tunnels are missing from `networking.interfaces` (Ruby lists them with MTU and flags). Third, primary-IPv6 selection: Ruby takes the first bound IPv6 on the primary interface (macOS binds link-local first, so Ruby reports `fe80::…`, `scope6: link`); Go prefers the routable address (`fd79:…`, `scope6: global`).

## Goals / Non-Goals

**Goals:**
- `networking.hostname`/`networking.domain`/`networking.fqdn` carry short name / domain / FQDN respectively, matching Ruby on hosts with and without a configured domain.
- `networking.interfaces` includes every interface getifaddrs-equivalent enumeration reports, including address-less tunnels, with MTU and available attributes.
- Primary-IPv6 selection is pinned as: global scope > unique-local > link-local, documented as a deviation from Ruby.

**Non-Goals:**
- No change to primary-interface selection (default-route-based) or IPv4 selection.
- No reintroduction of the flat legacy aliases (`hostname`, `interfaces`, `mtu_*`, `ipaddress6`) — `remove-legacy-facts` owns their fate; this change targets the structured `networking` tree (and fixes shared probes that feed it).
- No new platform scope: fixes are in shared/Darwin paths; Linux/Windows/FreeBSD behavior is verified unchanged by existing fixture tests.

## Decisions

**1. Split the node name at the first dot, fall back like Ruby.**
Hostname = node name up to the first `.`; domain = the remainder, falling back to resolver search/domain configuration when the node name is undotted. FQDN = hostname + "." + domain when a domain exists, else the bare hostname. This is Ruby's rule and also plain correctness — the current behavior makes `hostname` and `fqdn` identical and `domain` redundant.

**2. Enumerate interfaces from the full interface list, not the address list.**
Iterate interfaces first (gif0/stf0 have flags and MTU but no addresses), then attach addresses where present. An interface with no addresses still appears with its MTU/flags, matching Ruby's getifaddrs-driven map.

**3. Keep routable-first IPv6 selection; document the deviation.**
Ruby's first-bound rule surfaces link-locals on macOS, which is historical accident, not intent. The Go rule — global > unique-local > link-local on the primary interface — answers what consumers actually ask. Recorded in the spec delta and the man page Go-port notes rather than silently diverging.

## Risks / Trade-offs

- [Hosts with undotted hostnames and no resolver domain] → hostname stays the bare name, `domain`/`fqdn` absent or bare per Ruby's fallback; covered by fixture tests for both shapes.
- [Including address-less interfaces changes `networking.interfaces` shape consumers saw] → additive only (new keys), unreleased project; CHANGELOG notes it.
- [Shared probe fix shifts hostname facts on Linux/Windows/FreeBSD too] → that is the point (the bug is in the shared split); per-platform fixture tests pin the corrected behavior everywhere.
- [IPv6 deviation surprises a Ruby-parity expectation] → spec delta + man page note make it explicit; the link-local-only scenario still matches Ruby.

## Migration Plan

Single PR: hostname split fix + interface enumeration fix + IPv6 selection pinned by tests + man page note + CHANGELOG. Verify with the full suite, platform gates, and a macOS comparison rerun (expect `networking.hostname` short, `gif0`/`stf0` present; `ip6`/`scope6` unchanged but now spec-pinned).

Rollback: revert the PR.

## Open Questions

None.
