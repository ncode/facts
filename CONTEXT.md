# Facts

Facts is a Go port of Puppet Facter: it discovers and reports facts about the system it runs on — embeddable as a Go library, and shipped as the `facts` CLI (ADR-0008; no `facter` alias).

## Language

### Naming

**Facts**:
The project, the Go library, and the identity of everything user-visible: `package facts`, module `github.com/ncode/facts`, the `facts` binary, the `Facts` diagnostics token.
_Avoid_: Facter (that's the upstream Ruby product we interoperate with)

**facts** (the CLI):
The CLI binary shipped by Facts (ADR-0008, superseding ADR-0004). There is no `facter` alias; the facter-named *inputs* (`facter.conf` paths, `FACTER_*` environment facts, puppetlabs fact directories) keep working as the compatibility tier of the input surface, with the facts-native names (`facts.conf`, `FACTS_*`, `/etc/facts/facts.d` and friends) taking precedence.

### Facts

**Fact**:
A named piece of system information addressed by dot-notation (e.g. `os.name`), whose value is a scalar, list, or map.
_Avoid_: property, attribute, metric

**Core fact**:
A fact discovered by the engine's built-in resolvers from the host system itself.
_Avoid_: built-in fact, native fact

**Registered fact**:
A fact registered programmatically on an engine at construction by the embedding Go program.
_Avoid_: custom fact (that's upstream Facter's Ruby-DSL concept, which Facts does not support)

**External fact**:
A fact supplied from outside the engine — structured data files, executables, or environment variables — that takes precedence over core and registered facts.

**Legacy fact**:
A flat, pre-structured fact name (e.g. `operatingsystem`) from Ruby Facter's deprecated alias layer. Removed entirely (ADR-0007): no legacy alias resolves anywhere; the structured tree is the only fact surface. Unrelated to the *legacy text format* (the default `key => value` output), which stays.

**Not-applicable fact**:
A fact whose preconditions don't hold on this host (e.g. EC2 metadata off-cloud). It is simply absent from the canonical tree — never an error. Only facts that were supposed to resolve and didn't count as failures.
_Avoid_: failed fact, missing fact (that means "no such fact name")

### Contracts

**Output contract**:
The externally observable shape of resolved facts — fact names, nesting, value normalization, and formatter output (JSON, YAML, HOCON, legacy text) — which must remain compatible with Ruby Facter. Binding; not negotiable in the library work.

**Input contract**:
The accepted sources and semantics of operator-supplied facts — external fact files/executables/env vars, and the config file (facts-native `facts.conf` first, `facter.conf` as the compat read) — which must keep working unchanged. Binding; not negotiable in the library work. The Ruby custom-fact DSL is deliberately outside the contract: `.rb` fact files are not read anywhere.

### Library surfaces

**Engine**:
An isolated, immutable unit of fact-discovery configuration — fact registrations, sources, and diagnostics are fixed at construction, and nothing mutates afterward. There is no package-global engine and no global mutable state; every consumer (the `facts` CLI included) constructs its own. Engines are hermetic at birth: they discover core facts only, until explicitly configured with registered facts, external fact sources, config files, or caches.
_Avoid_: collector, client, instance API

**Snapshot**:
The immutable result of one discovery run: the canonical tree plus pure query and decode operations over it. Facts within a snapshot are mutually consistent; freshness is obtained by discovering again, never by mutating. Discovery is expensive and explicit; querying a snapshot is pure and free.
_Avoid_: session, fact cache

**Compatibility boundary**:
The `facts` CLI process edge — the only place Ruby Facter compatibility is promised, via the output contract and input contract. The Go API itself makes no Ruby-compatibility promises.
_Avoid_: Ruby-compat facade (removed; the Go-level Ruby API no longer exists)

**Canonical tree**:
The single post-precedence dynamic representation of all resolved facts — the one structure both formatters and library consumers read. There is no second model.
_Avoid_: fact hash, output map

**Typed view**:
A decode of part of the canonical tree into a caller-supplied Go type, failing loudly on shape mismatch. A view never resolves facts independently of the canonical tree.
_Avoid_: typed fact, fact struct

## Example dialogue

> **Dev**: A consumer's monitoring agent wants `networking.ip` but with their own registered fact overriding it. Is there a global registry they add it to?
>
> **Expert**: No globals exist — they construct their own engine and register the fact on it. The `facts` CLI is just another consumer that wires up its own system-following engine; two programs in one process never share state.
>
> **Dev**: Their override comes from a script dropped in a directory — registered fact?
>
> **Expert**: External fact. Registered facts exist only in Go code, on the engine that registered them; anything delivered as executables or data files is external, and it wins over core and registered facts. External facts arrive through the input contract, so the library work must not change how they load or resolve. And if someone drops a `.rb` file in that directory, nothing reads it — the Ruby DSL is outside the contract.
