# The project is "facts"; the CLI binary stays "facter"

The module path `github.com/ncode/facts` is canonical, and the root package renames from `facter` to `facts` so the import path's last element matches the package identifier (consumers write `facts.New()`, not `import ".../facts"` → `facter.New()`). The shipped binary keeps the name `facter` so every script that shells out to it — and the output contract — is undisturbed. Ruby parity pins function behavior, not the Go package identifier, so the rename is parity-safe.

## Considered Options

- **Rename the repo/module to `…/facter`** — rejected: the module path is the hardest identifier to change once consumers exist, the repo name `facts` keeps deliberate distance from Puppet's Facter product name, and a repo rename adds churn for zero consumer benefit.
- **Keep the mismatch** — rejected: free today, paid by every future importer.
