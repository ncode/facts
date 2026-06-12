// Package facts discovers and reports facts about the system it runs on —
// hardware details, network settings, OS type and version, cloud metadata,
// and more. It is the library form of the facts CLI this module ships: both
// read the same canonical fact tree, so library answers never drift from CLI
// answers.
//
// An [Engine] is an isolated, immutable unit of fact-discovery configuration,
// built by [New] from functional options. Engines are hermetic by default:
// they resolve core facts only, and do not read config files, scan fact
// directories, execute external-fact scripts, read FACTS_*/FACTER_*
// environment variables, or touch the persistent cache until the
// corresponding option —
// [WithConfigFile], [WithExternalDirs], [WithCache], or
// [WithSystemDefaults] for full CLI-equivalent behavior — opts in.
// Registered facts are fixed at construction via [WithFact]; once built,
// nothing mutates.
//
// Resolution is explicit: [Engine.Discover] runs the configured resolvers and
// returns an immutable [Snapshot] of the canonical tree. Freshness is
// obtained by discovering again — the caller's Snapshot is the cache. Both
// types are safe for concurrent use, and independent Engines in one process
// share no state.
//
// A Snapshot answers dot-notation queries ([Snapshot.Value]) with the same
// values the CLI reports, exposes the whole tree ([Snapshot.Tree],
// [Snapshot.All]), and decodes subtrees into caller types ([As]). Missing
// facts are reported as [ErrFactNotFound]; a registered or external fact that
// legitimately resolved to nil returns (nil, nil). Discovery failures are
// partial: the Snapshot holds every fact that resolved and the returned error
// joins the per-source failures.
//
// Engine diagnostics flow through log/slog ([WithLogger]) and are discarded
// by default. Ruby Facter compatibility is promised only at the facts CLI
// process boundary and for operator-supplied fact sources (external facts,
// facter.conf semantics) — this API makes no Ruby-compatibility promises of
// its own. Ruby DSL fact files are not read from any source (ADR-0006).
package facts
