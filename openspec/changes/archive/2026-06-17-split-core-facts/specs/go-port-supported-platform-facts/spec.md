## ADDED Requirements

### Requirement: Core facts are assembled by category for independent testing

Core-fact resolution SHALL be organized into per-fact-category resolver modules, with a primary file per category (e.g. networking, processors, memory, os, dmi, disks, ssh) and optional non-GOOS auxiliary files only when a category becomes unwieldy. Each category module SHALL expose a package-internal assembly function that returns that category's resolved facts from a resolution Session. The core-fact orchestrator SHALL be the composition of those category functions, so a test MAY resolve and assert a single category in isolation without running full core-fact discovery. This is a structural constraint only: the resolved fact set, names, values, ordering after collection, and per-platform behavior MUST remain identical, and a per-platform split within a category MUST NOT use Go's reserved GOOS filename suffixes (`_linux`, `_windows`, `_darwin`, `_freebsd`), which would impose an implicit build constraint and exclude cross-platform resolver logic from other platforms' builds and tests.

#### Scenario: A category resolves independently of the full core set

- **WHEN** a test invokes a single core-fact category's assembly function (for example the networking category) with a resolution Session backed by category-specific fake host inputs
- **THEN** it MUST receive only that category's resolved facts, without invoking `CoreFacts`, `buildCoreFacts`, or another category's assembly function

#### Scenario: Category composition preserves the core fact set

- **WHEN** core facts are discovered through the per-category orchestrator and compared with the pre-split baseline under the same host and fixture inputs
- **THEN** the resolved fact names and values MUST be identical to the pre-split core fact set on every supported platform, with no fact added, removed, or reshaped

#### Scenario: Cross-platform resolver logic stays buildable on every platform

- **WHEN** a category's resolver or parsing logic for one platform (for example parsing Windows networking command output) is exercised by a deterministic Go test
- **THEN** that logic MUST compile and run on the other supported platforms' builds and CI, reached through the `goos` parameter seam rather than gated behind a GOOS-suffixed file
