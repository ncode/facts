# Tasks: Flatten the repository history

- [x] 1.1 Rewrite the `docs/HISTORY.md` recovery section: history flattened 2026-06-12 by owner decision; this file is the surviving summary; Ruby history lives in upstream puppetlabs/facter; remove all `git show` commands and commit-hash citations from living docs
- [x] 1.2 Sync the spec delta (remove "Historical records survive the removal" from `go-port-ruby-removal`) and archive this change
- [ ] 1.3 Create the orphan initial commit from the final tree with a provenance-recording message; force-push `main`
- [ ] 1.4 Verify: `git rev-list --count HEAD` is 1; CI green on the new root commit; `openspec validate --all` passes
