# Delta: facts-library-api

## MODIFIED Requirements

### Requirement: Hermetic engine construction
`facts.New` SHALL return an Engine that resolves core facts only: it MUST NOT read configuration files, scan external fact directories, execute external-fact scripts, read external-fact environment variables, or touch the persistent cache unless the corresponding functional option is supplied. A `WithSystemDefaults` option SHALL configure an Engine with full CLI-equivalent system-following behavior. The library SHALL expose no option for loading Ruby DSL fact files; `WithCustomDirs` does not exist.

#### Scenario: Default engine performs no implicit host configuration
- **WHEN** a consumer calls `facts.New()` with no options on a host that has a configuration file, populated default fact directories, and executable external facts
- **THEN** discovery resolves core facts only, executes no scripts, reads no config file, and the resolved facts are unaffected by the host's configuration

#### Scenario: Explicit opt-in to fact sources
- **WHEN** an Engine is constructed with `WithExternalDirs`, `WithConfigFile`, or `WithFact` options
- **THEN** discovery loads exactly the opted-in sources with input-contract semantics (external-fact parsing, precedence, configuration interpretation) identical to the CLI's

#### Scenario: System-following engine matches CLI behavior
- **WHEN** an Engine is constructed with `WithSystemDefaults`
- **THEN** its resolved canonical tree matches what the `facts` CLI resolves on the same host
