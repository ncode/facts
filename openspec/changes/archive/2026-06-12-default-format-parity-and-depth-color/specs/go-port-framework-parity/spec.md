# Delta: go-port-framework-parity

## ADDED Requirements

### Requirement: Depth-colored keys in the default format
The `facter` CLI SHALL colorize keys in the default text format according to their nesting depth, cycling a fixed ANSI palette per level; values SHALL remain uncolored. Color SHALL be enabled by default when standard output is a terminal and disabled otherwise; `--color` forces it on and `--no-color` disables it. This is a Facts extension: Ruby Facter's `--color` affects diagnostics only. Machine formats are never colorized.

#### Scenario: Keys are colored by depth
- **WHEN** the default text format renders with color in effect (terminal output, or `--color` given)
- **THEN** every key MUST be wrapped in the ANSI color assigned to its nesting depth (top-level keys depth 0, their children depth 1, and so on, cycling the palette), and values MUST carry no color codes

#### Scenario: Piped output is clean by default
- **WHEN** `facter` runs without `--color` and standard output is not a terminal (piped or redirected)
- **THEN** the default text format MUST contain no ANSI escape sequences

#### Scenario: --no-color always disables
- **WHEN** `facter --no-color` runs, regardless of whether output is a terminal
- **THEN** the default text format MUST contain no ANSI escape sequences

#### Scenario: --color forces color for non-terminal output
- **WHEN** `facter --color` runs with output piped or redirected
- **THEN** keys in the default text format MUST carry their depth colors

#### Scenario: Machine formats are never colorized
- **WHEN** `facter` runs with `--json`, `--yaml`, or `--hocon`, with or without `--color`
- **THEN** the formatted output MUST be byte-identical regardless of color settings
