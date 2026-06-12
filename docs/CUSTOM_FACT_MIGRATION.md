# Migrating Ruby Custom Facts to Facts

**Facts reads no `.rb` fact files.** The Ruby custom-fact DSL
(`Facter.add`, `setcode`, `confine`, `weight`, aggregates) is not part of the
input contract: there is no `--custom-dir` flag, no `FACTERLIB` lookup, no
`custom-dir` config key, and no Ruby evaluation of any kind. A `.rb` file in
an external-fact directory is skipped with a warning naming the file. The
rationale — and the alternatives that were considered and rejected — is
recorded in
[ADR-0006](adr/0006-no-ruby-dsl-external-facts-are-the-input-contract.md).

What replaces the DSL is the external-fact mechanism Ruby Facter already
ships: structured data files (`.yaml`, `.json`, `.txt`), executables in any
language the host can run, and `FACTER_*` environment variables. Embedders
using the Go library can also register facts programmatically with
`facts.WithFact` ("registered facts").

## Pattern mapping

| Ruby DSL pattern | External-fact equivalent |
|---|---|
| Literal `setcode` value (`setcode { 'web' }`, hashes, arrays) | A YAML or JSON file in an external-fact directory: `site_role: web` |
| Command `setcode` (`setcode 'hostname -f'`) or `Facter::Core::Execution.exec(...)` | An executable external fact that runs the command and prints `key=value`, JSON, or YAML on stdout |
| `confine` (value or block) | Conditional logic inside the executable: probe the platform and exit silently (print nothing) when the fact does not apply |
| `weight` / multiple resolutions | Not needed — keep a single source of truth per fact; one file or executable owns the name |
| Aggregates (`chunk`/`aggregate`) | Build the structured value inside the executable and emit it as one JSON/YAML document |

## Rewrite example

**Ruby DSL fact:**

```ruby
Facter.add(:role) do
  confine kernel: 'Linux'
  setcode do
    File.read('/etc/role').strip rescue 'unknown'
  end
end
```

**Executable external fact (`/etc/facter/facts.d/role.sh`, `chmod +x`):**

```sh
#!/bin/sh
[ "$(uname -s)" = "Linux" ] || exit 0   # the confine, now in the script
if [ -r /etc/role ]; then
  printf 'role=%s\n' "$(cat /etc/role)"
else
  printf 'role=unknown\n'
fi
```

For structured values emit a single JSON document instead:

```sh
#!/bin/sh
echo "{\"role\": {\"name\": \"$(cat /etc/role)\", \"tier\": \"web\"}}"
```

On Windows, use `.ps1`, `.bat`, `.cmd`, or `.exe` files in the external-fact
directory; `.ps1` files run through PowerShell automatically.

## Where external facts load from

- `--external-dir DIR` (repeatable) or the `external-dir` array in
  `facter.conf` — explicit external-fact directories.
- The platform default `facts.d` locations (for example
  `/etc/puppetlabs/facter/facts.d`, `/etc/facter/facts.d`).
- `FACTER_<name>` environment variables, which override file-based facts of
  the same name.

External facts win precedence over registered and core facts, exactly as in
Ruby Facter.

## Deviation: `facts --puppet` and Ruby plugin facts

Ruby Facter under `--puppet` loads Ruby *custom* facts synced by Puppet
pluginsync (`vardir/lib/facter`). Facts does not: `facts --puppet` searches
Puppet's plugin-fact destinations (`vardir/facts.d`) for **external** facts
only, and emits a warning when synced Ruby plugin facts are present that they
are not loaded. Rewrite those plugin facts as external facts using the
mapping above. This is a documented, permanent deviation (ADR-0006).

## Cutover checklist

1. Inventory the `.rb` fact files your fleet ships (including gem- and
   pluginsync-distributed ones).
2. Rewrite each per the pattern mapping; place the result in an external-fact
   directory your config already points at.
3. Diff `--json` output between Ruby Facter and the Facts binary on a
   representative host per platform.
4. Switch the fleet's `facter` entry point to the installed `facts` binary
   (the packaging cutover is recorded in [HISTORY.md](HISTORY.md)).
