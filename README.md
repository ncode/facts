<picture>
  <source media="(prefers-color-scheme: dark)" srcset="docs/assets/hero-dark.svg">
  <img alt="Facts — every fact about the machine. One canonical tree." src="docs/assets/hero-light.svg" width="100%">
</picture>

<p align="center">
  <a href="https://github.com/ncode/facts/actions/workflows/unit_tests.yaml"><img alt="tests" src="https://img.shields.io/github/actions/workflow/status/ncode/facts/unit_tests.yaml?branch=main&style=flat-square&label=tests&labelColor=171717"></a>
  <a href="https://goreportcard.com/report/github.com/ncode/facts"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/ncode/facts"></a>
  <a href="https://codecov.io/gh/ncode/facts"><img alt="codecov" src="https://codecov.io/gh/ncode/facts/graph/badge.svg?token=3PW5AF4IAV"></a>
  <a href="https://pkg.go.dev/github.com/ncode/facts"><img alt="go.dev reference" src="https://img.shields.io/badge/go.dev-reference-0070f3?style=flat-square&labelColor=171717"></a>
  <img alt="go version" src="https://img.shields.io/github/go-mod/go-version/ncode/facts?style=flat-square&labelColor=171717&color=4d4d4d">
  <a href="https://opensource.org/licenses/Apache-2.0"><img alt="License" src="https://img.shields.io/badge/License-Apache_2.0-blue.svg"></a>
</p>

<p align="center">
  <samp><a href="#hermetic-by-default">Library</a> · <a href="#the-cli-every-fact-one-command">CLI</a> · <a href="#tested-where-it-ships">Platforms</a> · <a href="#go-deeper">Documentation</a></samp>
</p>

<br>

Facts is a Go port of [Puppet Facter](https://github.com/puppetlabs/facter). It discovers facts about the system it runs on — hardware, networking, OS, cloud metadata — and serves them two ways: as an embeddable Go library, and as the `facts` CLI. It reads your existing facter configuration, fact directories, and `FACTER_*` environment facts as the compatibility tier of its input surface, so facter-configured hosts keep working unchanged.

Ruby Facter compatibility is promised exactly where it matters and nowhere else: at the CLI process boundary (the output contract) and for operator-supplied fact sources — external facts and `facter.conf` (the input contract, now read under facts-native names first; see [the input-compatibility reference](docs/FACTER_CONF_COMPATIBILITY.md)). Ruby DSL fact files are not read; rewrite them as external facts ([migration guide](docs/CUSTOM_FACT_MIGRATION.md)). The Go API itself is just Go.

<br>

<samp>LIBRARY</samp>

## Hermetic by default.

An `Engine` is an isolated, immutable unit of fact-discovery configuration. A fresh engine resolves core facts only — it reads no config files, scans no directories, executes no scripts — until you opt in. No package globals, no shared state: two engines in one process never see each other.

```go
eng, err := facts.New() // hermetic: core facts only
if err != nil {
	log.Fatal(err)
}
snap, err := eng.Discover(ctx)
if err != nil {
	log.Printf("partial discovery: %v", err) // snapshot still valid
}

osName, err := snap.Value("os.name") // same answer as `facts os.name`
```

Every source is an explicit option:

```go
eng, err := facts.New(
	facts.WithExternalDirs("/etc/myagent/facts.d/external"), // data files, executables
	facts.WithFact("agent_version", func(ctx context.Context) (any, error) {
		return "1.2.3", nil
	}),
	facts.WithLogger(slog.Default()), // diagnostics; discarded without it
)
```

Or follow the system exactly like the CLI does — the default config file (`facts.conf`, then `facter.conf`), default fact directories, `FACTS_*`/`FACTER_*` environment facts:

```go
eng, err := facts.New(facts.WithSystemDefaults())
```

<br>

<samp>SNAPSHOT</samp>

## Discover once. Query forever.

`Discover(ctx)` runs the resolvers and returns an immutable `Snapshot` — the canonical tree plus pure query and decode operations over it. Freshness means discovering again; the snapshot you hold is the cache. Both types are safe for concurrent use.

```go
type OS struct {
	Name   string `json:"name"`
	Family string `json:"family"`
}

osFact, err := facts.As[OS](snap, "os") // decode any subtree, loudly on mismatch

for name, value := range snap.All() {   // sorted top-level iteration
	fmt.Println(name, value)
}
```

Errors are honest: missing facts are `ErrFactNotFound`, a registered fact that legitimately resolved to nil is `(nil, nil)`, partial failures arrive joined while the snapshot keeps every fact that did resolve, and cancellation flows through `ctx` into command execution and cloud-metadata requests.

<br>

<samp>CLI</samp>

## The CLI. Every fact, one command.

The shipped binary is `facts`, and it keeps Ruby Facter's output contract — formatting, exit statuses, stderr diagnostics (with the program token rebranded to `Facts`), `facter.conf` semantics. Existing facter inputs keep working as the compat tier: `facter.conf` default paths, `FACTER_*` environment facts, and the puppetlabs fact directories are all still read, with the facts-native names winning when both are present.


```console
$ brew install ncode/tap/facts

$ facts os.name
Darwin

$ facts --json os.family kernel.version.full
{
  "kernel.version.full": "25.5.0",
  "os.family": "Darwin"
}

$ facts --external-dir ./facts.d site_role
web
```

```sh
make build   # builds ./facts from the project root
```

<br>

<samp>PLATFORMS</samp>

## Tested where it ships.

Every release target is a blocking CI gate — unit tests, the race detector over the engine, a built-binary smoke, and per-platform release-gate fact checks.

| Platform | Architectures | Gate | Supported facts |
| --- | --- | --- | --- |
| Linux | x64, arm64 | native runners + container distro matrix | [Linux facts](docs/supported-facts/linux.md) |
| macOS | arm64, x64 | native runners | [macOS / Darwin facts](docs/supported-facts/darwin.md) |
| Windows | Server 2022, 2025 | native runners + release-gate fact set | [Windows facts](docs/supported-facts/windows.md) |
| FreeBSD | amd64 | VM job + release-gate fact set | [FreeBSD facts](docs/supported-facts/freebsd.md) |
| OpenBSD | amd64 | VM job + release-gate fact set | [OpenBSD facts](docs/supported-facts/openbsd.md) |
| NetBSD | amd64 | VM job + release-gate fact set | [NetBSD facts](docs/supported-facts/netbsd.md) |

Requires Go 1.26+.

<br>

<samp>DOCS</samp>

## Go deeper.

| | |
| --- | --- |
| [pkg.go.dev/github.com/ncode/facts](https://pkg.go.dev/github.com/ncode/facts) | API reference |
| [docs/schema/facts.yaml](docs/schema/facts.yaml) | every supported fact, with types, platforms, and descriptions — enforced in CI |
| [docs/supported-facts/](docs/supported-facts/) | generated per-platform supported fact pages with production-shaped examples |
| [CONTEXT.md](CONTEXT.md) · [docs/adr/](docs/adr/) | the project's language and architectural decisions |
| [docs/HISTORY.md](docs/HISTORY.md) | how the port happened, the verification record, and how to recover the full porting docs from git history |
| [docs/CUSTOM_FACT_MIGRATION.md](docs/CUSTOM_FACT_MIGRATION.md) | migrating Ruby custom facts to external facts (no `.rb` files are read) |
| [docs/FACTER_CONF_COMPATIBILITY.md](docs/FACTER_CONF_COMPATIBILITY.md) | operator input compatibility: facts-native names, facter-compat reads, `facter.conf` semantics |
| [man/man8/facts.8](man/man8/facts.8) | the CLI manual |

<br>

<samp>DEVELOPING</samp>

## Working on Facts.

```sh
make test          # go test ./...
make race          # race detector across the module
make bench-stable  # repeated benchmark baseline for hot paths
```

When changing fact resolution or formatting paths, capture a benchmark baseline before and after:

```sh
make bench-stable BENCH=BenchmarkFormatJSON PACKAGES=./internal/engine
```

<br>

---

<p align="center">
  <samp>APACHE-2.0 · <a href="LICENSE">LICENSE</a> · <a href="NOTICE">NOTICE</a></samp>
</p>
<p align="center">
  <sub>Copyright 2026 Juliano Martinez.</sub>
</p>
