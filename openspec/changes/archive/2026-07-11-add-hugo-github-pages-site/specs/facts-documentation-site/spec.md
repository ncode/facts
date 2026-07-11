## ADDED Requirements

### Requirement: Hugo documentation site
Facts SHALL provide a Hugo-based static documentation site whose source is committed to the repository and whose generated HTML is not committed.

#### Scenario: Local Hugo build
- **WHEN** a contributor runs the documented Hugo build command from the repository root
- **THEN** Hugo MUST generate the site into the configured output directory
- **AND** the generated output directory MUST remain ignored or otherwise uncommitted

#### Scenario: No JavaScript package pipeline
- **WHEN** a contributor inspects the site build inputs
- **THEN** the site MUST NOT require npm, a JavaScript bundler, Hugo modules, or a third-party Hugo theme

### Requirement: Homepage presents Facts
The documentation site SHALL include a homepage that presents Facts as both an embeddable Go library and the `facts` CLI.

#### Scenario: Homepage primary message
- **WHEN** a visitor opens `https://facts.martinez.io/`
- **THEN** the first screen MUST identify the product as Facts
- **AND** it MUST describe Facts as a Go port of Puppet Facter with both library and CLI usage

#### Scenario: Homepage install actions
- **WHEN** a visitor scans the homepage
- **THEN** it MUST expose the Go library install command `go get github.com/ncode/facts`
- **AND** it MUST expose the CLI install command `brew install ncode/tap/facts`

### Requirement: Existing docs are rendered as site content
The documentation site SHALL render the existing Markdown documentation under `docs/` as the source content for documentation pages.

#### Scenario: Docs navigation
- **WHEN** a visitor uses the site navigation
- **THEN** it MUST expose shallow groups for Start, Library, CLI, Supported facts, and Project
- **AND** those groups MUST link to the relevant existing documentation pages

#### Scenario: Supported fact pages remain generated
- **WHEN** supported fact reference pages are displayed on the site
- **THEN** they MUST be rendered from `docs/supported-facts/*.md`
- **AND** their content MUST remain owned by the existing schema-driven generation flow

### Requirement: README-derived visual system
The documentation site SHALL use the visual language already established in the README assets.

#### Scenario: Palette and typography
- **WHEN** the site is rendered
- **THEN** it MUST use the README palette: near-black ink, near-white canvas surfaces, muted gray text, hairline borders, and the existing blue/cyan/violet/pink/coral/amber accent colors
- **AND** it MUST use system sans-serif and monospace font stacks without remote font dependencies

#### Scenario: Hero visual treatment
- **WHEN** the homepage first screen is rendered
- **THEN** it MUST include a hero-scale mesh-gradient treatment based on the README hero colors
- **AND** the gradient MUST be used as a large atmospheric element, not as small decorative swatches

### Requirement: GitHub Pages custom-domain deployment
Facts SHALL deploy the generated documentation site to GitHub Pages at `https://facts.martinez.io/`.

#### Scenario: Pages artifact deployment
- **WHEN** changes are merged to the default branch
- **THEN** a GitHub Actions workflow MUST build the Hugo site
- **AND** it MUST publish the generated site using GitHub Pages artifact deployment

#### Scenario: Custom domain file
- **WHEN** the site is built for GitHub Pages
- **THEN** the generated artifact MUST include a `CNAME` file containing `facts.martinez.io`

#### Scenario: Canonical base URL
- **WHEN** Hugo renders absolute URLs
- **THEN** the configured base URL MUST be `https://facts.martinez.io/`
