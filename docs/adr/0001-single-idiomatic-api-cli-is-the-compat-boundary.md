# One idiomatic Go API; the CLI is the only compatibility boundary

> Partially superseded by ADR-0006: the custom-fact DSL is no longer part of the input contract.

The Go port originally preserved Ruby Facter's module-level API (`ToHash`, `Value`, `Loadfacts`, `PuppetFacts`, …) on package-global state, pinned by the `go-port-framework-parity` spec. With the port complete and the project renamed to Facts, that Go-level Ruby-compat surface is removed entirely rather than kept as a facade. The library exposes a single idiomatic API — `facts.Engine` instances from `facts.New`, context-aware methods returning errors, zero package-level mutable state. Ruby compatibility survives only at the process boundary: the `facter` binary keeps emitting byte-compatible output (output contract), and custom/external fact sources keep loading identically (input contract). The framework-parity spec's public-API requirement must be amended via an OpenSpec change, and the API-level parity tests in `facter_test.go` are reborn as Engine- and CLI-level contract tests rather than deleted.

## Considered Options

- **Keep the global Ruby-shaped API as the library surface** — rejected: shared state across consumers in one process, no context, no error returns.
- **Facade over a default engine** (`http.DefaultClient` pattern) — rejected: preserves ~58 Ruby-shaped exports for a consumer that doesn't exist. The CLI can wire an Engine directly, and every external consumer of Ruby semantics talks to the binary, not the Go API.
- **Remove the compat API** (chosen) — breaking only for Go importers of the pre-rename package, of which there are none outside this repo.
