# Design: Flatten the repository history

## Context

The repo is a fork of puppetlabs/facter carrying its full history (2,340 commits, 84 contributors, since 2019). Every other tie to the upstream identity has been deliberately severed (ADR-0006/0007/0008, the docs consolidation); the history is the last inherited signal. The maintainer chose flattening WITHOUT an archival copy, accepting that the porting-era detail records (migration log, parity ledger, port tracker — summarized in `docs/HISTORY.md` before their deletion from HEAD) become unrecoverable from this repository.

## Goals / Non-Goals

**Goals:**
- `main` history = one initial commit whose tree is the current verified state and whose message records provenance.
- `docs/HISTORY.md` and the spec tree tell no lies: the recoverability promise is removed, not left dangling.

**Non-Goals:**
- No archival branch, bundle, or mirror (explicit owner decision; the option was offered and declined).
- No NOTICE/LICENSE changes (attribution is file-based, not history-based).
- No edits to archived OpenSpec artifacts (point-in-time records; their commit-hash citations become inert text).

## Decisions

**1. Order of operations: docs and spec first, flatten last.**
The HISTORY.md rewrite, the spec delta sync, and this change's own archive land in the tree BEFORE the orphan commit is created, so the single surviving commit contains the fully consistent state.

**2. Orphan-branch procedure.**
`git checkout --orphan` + `git add -A` + one commit; `git branch -M main`; `git push --force origin main`. The initial commit message: project one-liner, provenance (ported from github.com/puppetlabs/facter, Apache 2.0, see NOTICE and docs/HISTORY.md), and the note that history begins here by decision.

**3. HISTORY.md keeps the summary, drops the recovery commands.**
The "Recovering the full records" section becomes "About this repository's history": flattened 2026-06-12; this file is the surviving summary; the Ruby history lives upstream. No hash citations remain in living docs.

## Risks / Trade-offs

- [A future provenance or derivation question needs the porting detail] → unanswerable from this repo by accepted decision; HISTORY.md's summary and upstream's history are the remaining evidence.
- [Existing clones/forks diverge] → zero known users; the unreleased window is the only cheap moment, consistent with every prior removal.
- [GitHub may cache old objects/PR refs for a while] → cosmetic; they are unreachable from any ref we control.

## Migration Plan

Single session: HISTORY.md rewrite → spec delta sync → archive this change → orphan commit → force-push → CI verification on the new root commit.

Rollback: none after the force-push and local repack — that is the point. Until the push, everything is reversible.

## Open Questions

None — preservation was explicitly declined by the owner.
