# Design: Consolidate the porting history

## Context

The docs tree is dominated by the porting project that ended on 2026-06-10: `docs/MIGRATION.md` (9,471-line append-only log), `docs/PARITY_LEDGER.md` (938-line frozen verification), `PORTING.md` (140-line tracker, ~30 lines still live), `docs/data-flow.md` (Ruby Facter's internal architecture), and the 507-line Ruby Facter release-notes tail of `CHANGELOG.md`. `CONTRIBUTING.md` is upstream Puppet boilerplate with a broken `lib/schema/facter.yaml` link. Five porting-era specs pin these files by name. The 2026-06-12 analysis classified every Puppet mention in the repo: required-compat (input paths), feature (`--puppet`), attribution (NOTICE), frozen history, stale upstream, and positioning.

## Goals / Non-Goals

**Goals:**
- One file — `docs/HISTORY.md` — tells the whole porting story at summary depth and indexes git history for the full records.
- A reader's first contact (README, CONTRIBUTING) is 100% Facts; Puppet appears only where it is compat surface, the `--puppet` feature, attribution, or one honest positioning sentence.
- The spec tree stops requiring files that no longer earn their place in HEAD.

**Non-Goals:**
- No rewriting of recorded history: HISTORY.md summarizes outcomes, never alters them; deleted files stay byte-recoverable in git.
- No changes to `docs/FACTER_CONF_COMPATIBILITY.md`, `docs/CUSTOM_FACT_MIGRATION.md`, `CONTEXT.md`, or the ADRs — those are living documents.
- No NOTICE/LICENSE changes (Apache 2.0 attribution stays).
- No code changes.

## Decisions

**1. Delete from HEAD; git history is the archive.**
Alternative — move the frozen files to `docs/history/` — rejected: 11,000 lines of frozen text in HEAD costs every clone, grep, and doc search forever, for records nobody reads twice. `docs/HISTORY.md` cites commit `daa2f7b9` (the last commit containing all four files); `git show daa2f7b9:docs/PARITY_LEDGER.md` reproduces any of them exactly. The ruby-removal spec delta makes this recoverability a requirement.

**2. HISTORY.md structure (~200 lines).**
Sections: what Facts is and where it came from; the porting method (TDD slices against Ruby specs, ledger discipline, four platform gates); final ledger accounting (614 in-scope / 281 out-of-scope / 0 unclassified, 612 verified references + 2 waivers); release-readiness milestones with dates; the post-port departures (ADR-0001 through ADR-0008 one-liners); what was removed from HEAD and the recovery commands; where the Ruby Facter release notes went.

**3. CONTRIBUTING.md is the home for the still-live PORTING.md content.**
Platform scope (the four targets, the out-of-scope list), TDD rules, benchmark discipline, the release-gate descriptions, and the Ruby Facter comparison technique (install the gem, diff `--json`) move there, joined by build/test/bench commands and the OpenSpec workflow. The `facts-schema` change later adds the "new facts must be in the schema" rule — leave a placeholder reference out until that change lands.

**4. CHANGELOG keeps only Facts content.**
Everything from "# Previous versions" down is removed; the Unreleased section and the pointer line stay. HISTORY.md notes that Ruby Facter's 4.0.x notes live in git history and upstream.

**5. Spec scaffolding is retired, not silently broken.**
Each porting-era requirement that pinned a deleted file gets an explicit REMOVED (completed milestone) or MODIFIED (live requirement, new pointer) delta — never a dangling reference. `go-port-parity-ledger-integrity` empties entirely; its spec directory is deleted at archive time.

## Risks / Trade-offs

- [A future question needs the detailed porting record] → `git show <commit>:<path>` is one command away; HISTORY.md prints the exact incantations.
- [Cross-references to deleted files left dangling] → grep-driven sweep for `PORTING.md`, `MIGRATION.md`, `PARITY_LEDGER`, `data-flow` across the repo (docs, specs, code comments, workflows) is an explicit task with a zero-hits acceptance criterion (outside HISTORY.md itself and the openspec archive).
- [Losing the migration log's "append-only" discipline for future behavior changes] → that discipline already moved to CHANGELOG entries + OpenSpec change records; CONTRIBUTING.md says so explicitly.

## Migration Plan

Single PR: (1) write `docs/HISTORY.md`; (2) write the new `CONTRIBUTING.md`; (3) delete the four files and trim CHANGELOG; (4) cross-reference sweep; (5) spec deltas land with the change; (6) full test suite + CI gates (docs-only, but gates run on every push).

Rollback: revert; nothing is unrecoverable either way.

## Open Questions

None.
