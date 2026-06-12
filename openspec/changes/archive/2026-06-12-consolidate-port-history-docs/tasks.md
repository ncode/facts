# Tasks: Consolidate the porting history

## 1. Write the new documents

- [x] 1.1 Write `docs/HISTORY.md` (~200 lines): origin and method, final ledger accounting (614/281/0, 612 refs + 2 waivers), release-readiness milestones, ADR-0001…0008 one-liners, what was removed from HEAD, and the exact `git show daa2f7b9:<path>` recovery commands for `PORTING.md`, `docs/MIGRATION.md`, `docs/PARITY_LEDGER.md`, `docs/data-flow.md`, and the CHANGELOG tail
- [x] 1.2 Rewrite `CONTRIBUTING.md` for Facts: build/test/race/bench commands, platform scope and out-of-scope list, TDD rules, benchmark discipline, the four release gates, OpenSpec change workflow, and the Ruby Facter `--json` comparison technique — no Puppet boilerplate, no broken links

## 2. Remove the frozen content

- [x] 2.1 Delete `PORTING.md`, `docs/MIGRATION.md`, `docs/PARITY_LEDGER.md`, `docs/data-flow.md`
- [x] 2.2 Trim `CHANGELOG.md`: remove "# Previous versions" and everything below it (~507 lines); keep the Unreleased section
- [x] 2.3 Cross-reference sweep: `grep -rn 'PORTING\.md\|MIGRATION\.md\|PARITY_LEDGER\|data-flow'` over docs, README, CONTEXT.md, specs, workflows, and code comments returns zero hits outside `docs/HISTORY.md` and `openspec/changes/archive/`
- [x] 2.4 Puppet-mention sweep against the keep-list (compat paths, `--puppet` feature, NOTICE, ADRs, one README positioning sentence): no other Puppet-the-company references remain

## 3. Verification

- [x] 3.1 `openspec validate consolidate-port-history-docs` passes; spec deltas consistent with the files actually present
- [x] 3.2 `go test ./...` unaffected (docs-only); all links in HISTORY.md and CONTRIBUTING.md resolve; recovery commands verified to work (`git show daa2f7b9:docs/PARITY_LEDGER.md | head` succeeds)
- [x] 3.3 Platform CI gates green on the final commit
